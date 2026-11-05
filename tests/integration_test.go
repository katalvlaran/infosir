package tests

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"infosir/internal/db/repository"
	"infosir/internal/models"
)

// TestRepositoryAtomicity uses only an explicitly supplied disposable database.
// Its isolated schema emulates the legacy base table without Timescale aggregates.
func TestRepositoryAtomicity(t *testing.T) {
	dsn := os.Getenv("INFOSIR_TEST_DSN")
	if dsn == "" {
		t.Skip("set INFOSIR_TEST_DSN to a disposable PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("infosir_test_%d", time.Now().UnixNano())
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, e := admin.Exec(c, "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	_, e = pool.Exec(ctx, `CREATE TABLE futures_klines(time timestamptz NOT NULL,symbol text NOT NULL CHECK(symbol<>'FAIL'),open_price double precision,high_price double precision,low_price double precision,close_price double precision,volume double precision,quote_volume double precision,trades bigint,taker_buy_base_volume double precision,taker_buy_quote_volume double precision,PRIMARY KEY(symbol,time))`)
	if e != nil {
		t.Fatal(e)
	}
	migration, e := os.ReadFile(filepath.Join("..", "internal", "db", "migrations", "0003_ingestion_receipts.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(migration)); e != nil {
		t.Fatal(e)
	}
	repo := repository.NewKlineRepository(pool)
	k := models.Kline{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Symbol: "BTCUSDT", OpenPrice: 1, HighPrice: 1, LowPrice: 1, ClosePrice: 1}
	if e = repo.InsertKline(ctx, k); e != nil {
		t.Fatal(e)
	}
	if e = repo.BindProfile(ctx, "spot", "1m", false); !errors.Is(e, repository.ErrStorageProfile) {
		t.Fatalf("unverified history accepted: %v", e)
	}
	if e = repo.BindProfile(ctx, "spot", "1m", true); e != nil {
		t.Fatal(e)
	}
	if e = repo.BindProfile(ctx, "usdm", "1m", true); !errors.Is(e, repository.ErrStorageProfile) {
		t.Fatalf("profile collision: %v", e)
	}
	got, e := repo.FindLast(ctx, " btcusdt ")
	if e != nil || got.Symbol != k.Symbol {
		t.Fatalf("canonical lookup %v %v", got, e)
	}
	h := sha256.Sum256([]byte("delivery"))
	k.Time = k.Time.Add(time.Minute)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.CommitMessage(ctx, "consumer", "same", h[:], []models.Kline{k}) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var n int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM futures_klines").Scan(&n); e != nil || n != 2 {
		t.Fatalf("dedup rows=%d err=%v", n, e)
	}
	changed := sha256.Sum256([]byte("changed"))
	if e = repo.CommitMessage(ctx, "consumer", "same", changed[:], []models.Kline{k}); !errors.Is(e, repository.ErrPayloadConflict) {
		t.Fatalf("hash collision accepted %v", e)
	}
	good := k
	good.Time = good.Time.Add(time.Minute)
	bad := good
	bad.Symbol = "FAIL"
	if e = repo.CommitMessage(ctx, "consumer", "rollback", h[:], []models.Kline{good, bad}); e == nil {
		t.Fatal("expected second-row database constraint failure")
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM futures_klines").Scan(&n); e != nil || n != 2 {
		t.Fatalf("partial batch persisted %d %v", n, e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM infosir_consumer_inbox WHERE event_id='rollback'").Scan(&n); e != nil || n != 0 {
		t.Fatalf("inbox survived rollback %d %v", n, e)
	}
	if e = repo.CommitMessage(ctx, "consumer", "rollback", h[:], []models.Kline{good}); e != nil {
		t.Fatalf("retry failed %v", e)
	}
	for i := 0; i < 2; i++ {
		if e = repo.Quarantine(ctx, "c", "id", "invalid_json", []byte("bad")); e != nil {
			t.Fatal(e)
		}
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM infosir_quarantine").Scan(&n); e != nil || n != 1 {
		t.Fatalf("quarantine dedup %d %v", n, e)
	}
}
