package models

import (
	"fmt"
	"math"
	"strings"
	"time"
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
	if k.Time.IsZero() || k.Time.Year() < 1970 || k.Time.Year() > 9999 {
		return fmt.Errorf("invalid kline time")
	}
	if k.Symbol == "" || k.Symbol != strings.TrimSpace(k.Symbol) || k.Symbol != strings.ToUpper(k.Symbol) {
		return fmt.Errorf("invalid canonical symbol")
	}
	for _, r := range k.Symbol {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("invalid symbol character")
		}
	}
	for _, v := range []float64{k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return fmt.Errorf("invalid price")
		}
	}
	if k.LowPrice > k.HighPrice || k.OpenPrice < k.LowPrice || k.OpenPrice > k.HighPrice || k.ClosePrice < k.LowPrice || k.ClosePrice > k.HighPrice {
		return fmt.Errorf("inconsistent OHLC prices")
	}
	for _, v := range []float64{k.Volume, k.QuoteVolume, k.TakerBuyBaseVolume, k.TakerBuyQuoteVolume} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("invalid volume")
		}
	}
	if k.Trades < 0 {
		return fmt.Errorf("negative trade count")
	}
	if k.TakerBuyBaseVolume > k.Volume || k.TakerBuyQuoteVolume > k.QuoteVolume {
		return fmt.Errorf("taker volume exceeds total")
	}
	return nil
}
