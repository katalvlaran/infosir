package model

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
)

// Kline описывает одну свечу (FuturesKline) для криптопары на бирже Binance
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

// Validate проверяет корректность данных внутри Kline.
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
