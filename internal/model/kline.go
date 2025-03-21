package model

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

// Kline описывает одну свечу (candlestick) для криптопары на бирже Binance
type Kline struct {
	OpenTime  int64   `json:"openTime"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	CloseTime int64   `json:"closeTime"`
}

// Validate проверяет корректность данных внутри Kline.
// Здесь можно задать минимальные значения или логику,
// чтобы Open ≤ High ≥ Low ≤ Close, и т.д., если нужно.
func (k Kline) Validate() error {
	return validation.ValidateStruct(&k,
		// open, high, low, close, volume не должны быть отрицательными
		validation.Field(&k.Open, validation.Required, validation.Min(0.0)),
		validation.Field(&k.High, validation.Required, validation.Min(0.0)),
		validation.Field(&k.Low, validation.Required, validation.Min(0.0)),
		validation.Field(&k.Close, validation.Required, validation.Min(0.0)),
		validation.Field(&k.Volume, validation.Required, validation.Min(0.0)),

		// time поля могут быть в миллисекундах (Binance), > 0
		validation.Field(&k.OpenTime, validation.Min(1)),
		validation.Field(&k.CloseTime, validation.Min(1)),
	)
}
