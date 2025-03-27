package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"infosir/cmd/config"
	"infosir/internal/db"
	"infosir/internal/db/repository"
	"infosir/internal/models"
	"infosir/internal/utils"

	_ "github.com/lib/pq" // example: if we do a direct DB test
	"github.com/stretchr/testify/assert"
)

// TestIntegration_DatabaseInsert is a pseudo-integration test that checks if we
// can actually insert and retrieve klines from a real or test database.
func TestIntegration_DatabaseInsert(t *testing.T) {
	// 1. Possibly load config if not already done
	err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize logger (or do once in a TestMain)
	utils.InitLogger()

	// 3. Connect to DB
	dbPool, err := db.InitDatabase()
	if err != nil {
		t.Fatalf("Cannot init DB: %v", err)
	}
	defer dbPool.Close()

	// 4. Create the repository
	kRepo := repository.NewKlineRepository(dbPool)

	openTime, err := time.Parse("2006-01-02T15:04:05Z", "2024-01-02T15:04:05Z")
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	// 5. Insert a kline
	testKline := models.Kline{
		Symbol:              "TESTPAIR",
		Time:                openTime,
		OpenPrice:           123.45,
		HighPrice:           130.00,
		LowPrice:            120.11,
		ClosePrice:          128.00,
		Volume:              123.456,
		QuoteVolume:         999.999,
		Trades:              456,
		TakerBuyBaseVolume:  50.0,
		TakerBuyQuoteVolume: 60.0,
	}

	ctx := context.Background()
	err = kRepo.InsertKline(ctx, testKline)
	assert.NoError(t, err, "InsertKline should succeed")

	// 6. Retrieve the last kline for "TESTPAIR"
	retrieved, err := kRepo.FindLast(ctx, "testpair") // note the lowercase, if you store symbol in lower
	assert.NoError(t, err, "FindLast should succeed")
	assert.Equal(t, testKline.ClosePrice, retrieved.ClosePrice, "ClosePrice mismatch")
	assert.Equal(t, testKline.Trades, retrieved.Trades, "Trades mismatch")
}

// TestIntegration_Connection ensures DB migrations are done properly
func TestIntegration_Connection(t *testing.T) {
	// Possibly you want to check that the table "futures_klines" is created, etc.
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.Cfg.Database.User,
		config.Cfg.Database.Password,
		config.Cfg.Database.Host,
		config.Cfg.Database.Port,
		config.Cfg.Database.Name,
	)

	dbConn, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to open raw DB conn: %v", err)
	}
	defer dbConn.Close()

	// ping
	err = dbConn.Ping()
	assert.NoError(t, err, "Ping to DB should succeed")

	// check if table "futures_klines" exists
	var tableName string
	err = dbConn.QueryRow(`
		SELECT tablename
		FROM pg_catalog.pg_tables
		WHERE tablename = 'futures_klines';
	`).Scan(&tableName)

	assert.NoError(t, err, "Should find futures_klines in pg_tables")
	assert.Equal(t, "futures_klines", tableName, "Table name mismatch")
}

// Optionally we can define a TestMain() for integration tests set up
func TestMain(m *testing.M) {
	// e.g. do some start up or docker container management
	code := m.Run()
	os.Exit(code)
}
