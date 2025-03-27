package repository

import (
	"context"

	"infosir/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// KlineRepository manages read/write operations for the "futures_klines" hypertable.
type KlineRepository struct {
	db *pgxpool.Pool
}

// NewKlineRepository constructs a repository with the given pgx pool.
func NewKlineRepository(db *pgxpool.Pool) *KlineRepository {
	return &KlineRepository{db: db}
}

// InsertKline inserts a single Kline into the hypertable.
// If the (symbol, time) key already exists, it is skipped.
func (r *KlineRepository) InsertKline(ctx context.Context, k models.Kline) error {
	query := `
		INSERT INTO futures_klines (
			time, symbol, open_price, high_price, low_price, close_price,
			volume, quote_volume, trades, taker_buy_base_volume, taker_buy_quote_volume
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (symbol, time) DO NOTHING;
	`

	_, err := r.db.Exec(ctx, query,
		k.Time, k.Symbol, k.OpenPrice, k.HighPrice, k.LowPrice,
		k.ClosePrice, k.Volume, k.QuoteVolume, k.Trades,
		k.TakerBuyBaseVolume, k.TakerBuyQuoteVolume,
	)

	return err
}

// BatchInsertKlines performs a bulk insert of many klines in a single batch.
// Duplicate entries are ignored via ON CONFLICT DO NOTHING.
func (r *KlineRepository) BatchInsertKlines(ctx context.Context, klines []models.Kline) error {
	if len(klines) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO futures_klines (
			time, symbol, open_price, high_price, low_price, close_price,
			volume, quote_volume, trades, taker_buy_base_volume, taker_buy_quote_volume
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (symbol, time) DO NOTHING;
	`

	for _, k := range klines {
		batch.Queue(query,
			k.Time, k.Symbol, k.OpenPrice, k.HighPrice, k.LowPrice,
			k.ClosePrice, k.Volume, k.QuoteVolume, k.Trades,
			k.TakerBuyBaseVolume, k.TakerBuyQuoteVolume,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	_, err := br.Exec()
	return err
}

// FindLast retrieves the most recent Kline for the given symbol.
func (r *KlineRepository) FindLast(ctx context.Context, symbol string) (models.Kline, error) {
	query := `
		SELECT time, symbol, open_price, high_price, low_price, close_price,
		       volume, quote_volume, trades, taker_buy_base_volume, taker_buy_quote_volume
		FROM futures_klines
		WHERE symbol = $1
		ORDER BY time DESC
		LIMIT 1;
	`

	var k models.Kline
	err := r.db.QueryRow(ctx, query, symbol).Scan(
		&k.Time, &k.Symbol, &k.OpenPrice, &k.HighPrice, &k.LowPrice,
		&k.ClosePrice, &k.Volume, &k.QuoteVolume, &k.Trades,
		&k.TakerBuyBaseVolume, &k.TakerBuyQuoteVolume,
	)

	return k, err
}
