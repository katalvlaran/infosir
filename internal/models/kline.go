package models

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
)

// Kline represents a single candlestick (OHLCV) data point for a given symbol
// at a specific timestamp. These are typically retrieved from Binance or
// another exchange, then stored in TimescaleDB.
//
// Fields:
//   - Time: The timestamp associated with this kline (UTC).
//   - Symbol: The trading pair, e.g. "BTCUSDT".
//   - OpenPrice, HighPrice, LowPrice, ClosePrice: Standard OHLC values.
//   - Volume: The base asset volume in this interval.
//   - QuoteVolume: The quote asset volume in this interval.
//   - Trades: The number of trades during this interval.
//   - TakerBuyBaseVolume: The base volume where takers were the buyers.
//   - TakerBuyQuoteVolume: The quote volume where takers were the buyers.
type Kline struct {
	Time                time.Time `json:"time"`
	Symbol              string    `json:"symbol"`
	OpenPrice           float64   `json:"open"`
	HighPrice           float64   `json:"high"`
	LowPrice            float64   `json:"low"`
	ClosePrice          float64   `json:"close"`
	Volume              float64   `json:"volume"`
	QuoteVolume         float64   `json:"quote_volume"`
	Trades              int64     `json:"trades"`
	TakerBuyBaseVolume  float64   `json:"taker_buy_base_volume"`
	TakerBuyQuoteVolume float64   `json:"taker_buy_quote_volume"`
}

// Validate checks if the Kline struct fields are within acceptable ranges
// (e.g., no negative prices or volumes). If anything is invalid, returns an error.
func (k Kline) Validate() error {
	return validation.ValidateStruct(&k,
		validation.Field(&k.Time, validation.Required),
		validation.Field(&k.Symbol, validation.Required),
		validation.Field(&k.OpenPrice, validation.Required, validation.Min(0.0)),
		validation.Field(&k.HighPrice, validation.Required, validation.Min(0.0)),
		validation.Field(&k.LowPrice, validation.Required, validation.Min(0.0)),
		validation.Field(&k.ClosePrice, validation.Required, validation.Min(0.0)),
		validation.Field(&k.Volume, validation.Required, validation.Min(0.0)),
		validation.Field(&k.QuoteVolume, validation.Required, validation.Min(0.0)),
		validation.Field(&k.Trades, validation.Required, validation.Min(1)),
		validation.Field(&k.TakerBuyBaseVolume, validation.Required, validation.Min(0.0)),
		validation.Field(&k.TakerBuyQuoteVolume, validation.Required, validation.Min(0.0)),
	)
}
