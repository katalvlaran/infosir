package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"infosir/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPayloadConflict = errors.New("message ID reused with different payload")
var ErrStorageProfile = errors.New("legacy storage profile mismatch or unconfirmed history")

// KlineRepository retains the legacy single-market/single-interval schema.
// It does not infer historical finality or implement the future revision journal.
type KlineRepository struct{ db *pgxpool.Pool }

func NewKlineRepository(db *pgxpool.Pool) *KlineRepository { return &KlineRepository{db: db} }

const insertKline = `INSERT INTO futures_klines(time,symbol,open_price,high_price,low_price,close_price,volume,quote_volume,trades,taker_buy_base_volume,taker_buy_quote_volume) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(symbol,time) DO NOTHING`

// BindProfile prevents the legacy natural key from mixing markets or intervals.
// Existing rows require an explicit operator assertion of their provenance.
func (r *KlineRepository) BindProfile(ctx context.Context, market, interval string, confirmExisting bool) error {
	if (market != "spot" && market != "usdm") || interval == "" {
		return ErrStorageProfile
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, "LOCK TABLE infosir_storage_profile IN EXCLUSIVE MODE"); err != nil {
		return err
	}
	var m, i string
	err = tx.QueryRow(ctx, "SELECT market,interval_code FROM infosir_storage_profile WHERE singleton").Scan(&m, &i)
	if err == nil {
		if m != market || i != interval {
			return ErrStorageProfile
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM futures_klines)").Scan(&exists); err != nil {
		return err
	}
	if exists && !confirmExisting {
		return ErrStorageProfile
	}
	if _, err = tx.Exec(ctx, "INSERT INTO infosir_storage_profile(singleton,market,interval_code) VALUES(true,$1,$2)", market, interval); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func validate(ks []models.Kline) error {
	for i, k := range ks {
		if err := k.Validate(); err != nil {
			return fmt.Errorf("kline %d: %w", i, err)
		}
	}
	return nil
}

// InsertKline preserves legacy duplicate semantics and validates the input.
func (r *KlineRepository) InsertKline(ctx context.Context, k models.Kline) error {
	return r.BatchInsertKlines(ctx, []models.Kline{k})
}

// BatchInsertKlines commits all rows together and observes every batch result.
func (r *KlineRepository) BatchInsertKlines(ctx context.Context, ks []models.Kline) error {
	if err := validate(ks); err != nil {
		return err
	}
	if len(ks) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err = writeBatch(ctx, tx, ks); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func writeBatch(ctx context.Context, tx pgx.Tx, ks []models.Kline) error {
	if len(ks) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for _, k := range ks {
		b.Queue(insertKline, k.Time, k.Symbol, k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice, k.Volume, k.QuoteVolume, k.Trades, k.TakerBuyBaseVolume, k.TakerBuyQuoteVolume)
	}
	results := tx.SendBatch(ctx, b)
	var first error
	for range ks {
		if _, err := results.Exec(); err != nil && first == nil {
			first = err
		}
	}
	return errors.Join(first, results.Close())
}

// CommitMessage atomically claims a logical delivery and persists its batch.
// A commit error is ambiguous: redelivery with the same ID resolves via inbox.
func (r *KlineRepository) CommitMessage(ctx context.Context, consumer, id string, hash []byte, ks []models.Kline) error {
	if consumer == "" || id == "" || len(hash) != sha256.Size {
		return errors.New("invalid delivery identity")
	}
	if err := validate(ks); err != nil {
		return err
	}
	h := hex.EncodeToString(hash)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	tag, err := tx.Exec(ctx, `INSERT INTO infosir_consumer_inbox(consumer_id,event_id,payload_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, consumer, id, h)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var previous string
		if err = tx.QueryRow(ctx, `SELECT payload_hash FROM infosir_consumer_inbox WHERE consumer_id=$1 AND event_id=$2`, consumer, id).Scan(&previous); err != nil {
			return err
		}
		if previous != h {
			return ErrPayloadConflict
		}
		return tx.Commit(ctx)
	}
	if err = writeBatch(ctx, tx, ks); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Quarantine makes permanent rejection durable before delivery termination.
func (r *KlineRepository) Quarantine(ctx context.Context, consumer, id, reason string, payload []byte) error {
	if len(payload) > 1<<20 {
		return errors.New("quarantine payload exceeds bound")
	}
	hash := sha256.Sum256([]byte(consumer + "\x00" + id + "\x00" + reason + "\x00" + string(payload)))
	_, err := r.db.Exec(ctx, `INSERT INTO infosir_quarantine(quarantine_id,consumer_id,event_id,reason_code,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, hex.EncodeToString(hash[:]), consumer, id, reason, payload)
	return err
}
func (r *KlineRepository) FindLast(ctx context.Context, symbol string) (models.Kline, error) {
	var k models.Kline
	err := r.db.QueryRow(ctx, `SELECT time,symbol,open_price,high_price,low_price,close_price,volume,quote_volume,trades,taker_buy_base_volume,taker_buy_quote_volume FROM futures_klines WHERE symbol=$1 ORDER BY time DESC LIMIT 1`, strings.ToUpper(strings.TrimSpace(symbol))).Scan(&k.Time, &k.Symbol, &k.OpenPrice, &k.HighPrice, &k.LowPrice, &k.ClosePrice, &k.Volume, &k.QuoteVolume, &k.Trades, &k.TakerBuyBaseVolume, &k.TakerBuyQuoteVolume)
	return k, err
}
