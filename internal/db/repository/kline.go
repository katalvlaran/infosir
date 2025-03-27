package repository

import (
	"context"

	"infosir/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type KlineRepository struct {
	pool *pgxpool.Pool
}

func NewKlineRepository(pool *pgxpool.Pool) *KlineRepository {
	return &KlineRepository{pool: pool}
}

// insertion
func (r *KlineRepository) InsertKline(ctx context.Context, c model.Kline) error {
	sql := `
    INSERT INTO futures_klines (
      time, symbol, open_price, high_price, low_price, close_price, volume, quote_volume, trades,
      taker_buy_base_volume, taker_buy_quote_volume
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    ON CONFLICT (symbol, time) DO NOTHING
  `
	_, err := r.pool.Exec(ctx, sql,
		c.Time, c.Symbol, c.OpenPrice, c.HighPrice, c.LowPrice, c.ClosePrice, c.Volume, c.QuoteVolume,
		c.Trades, c.TakerBuyBaseVolume, c.TakerBuyQuoteVolume,
	)
	return err
}

func (r *KlineRepository) BatchInsertKlines(ctx context.Context, klines []model.Kline) error {
	if len(klines) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	insertQuery := `
    INSERT INTO futures_klines (
      time, symbol, open_price, high_price, low_price, close_price, volume, quote_volume, trades,
      taker_buy_base_volume, taker_buy_quote_volume
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    ON CONFLICT (symbol, time) DO NOTHING
  `
	for _, c := range klines {
		batch.Queue(insertQuery,
			c.Time, c.Symbol, c.OpenPrice, c.HighPrice, c.LowPrice, c.ClosePrice,
			c.Volume, c.QuoteVolume, c.Trades, c.TakerBuyBaseVolume, c.TakerBuyQuoteVolume,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	_, err := br.Exec()
	if err != nil {
		_ = br.Close()
		return err
	}
	return br.Close()
}

func (r *KlineRepository) FindLast(ctx context.Context, symbol string) (model.Kline, error) {
	sql := `
    SELECT time, symbol, open_price, high_price, low_price, close_price, volume, quote_volume, trades,
           taker_buy_base_volume, taker_buy_quote_volume
    FROM futures_klines
    WHERE symbol = $1
    ORDER BY time DESC
    LIMIT 1
  `
	row := r.pool.QueryRow(ctx, sql, symbol)
	var c model.Kline
	err := row.Scan(
		&c.Time, &c.Symbol, &c.OpenPrice, &c.HighPrice, &c.LowPrice,
		&c.ClosePrice, &c.Volume, &c.QuoteVolume, &c.Trades,
		&c.TakerBuyBaseVolume, &c.TakerBuyQuoteVolume,
	)
	if err != nil {
		return c, err
	}
	return c, nil
}
