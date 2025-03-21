package mocks

import (
	"context"

	"infosir/internal/model"
	"infosir/pkg/crypto"
)

type MockBinanceClient struct {
	FetchKlinesFn func(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
}

func (m *MockBinanceClient) FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
	if m.FetchKlinesFn != nil {
		return m.FetchKlinesFn(ctx, pair, interval, limit)
	}
	// Default - return a single Kline
	return []model.Kline{
		{
			OpenTime:  1695000000000,
			Open:      20000.0,
			High:      21000.0,
			Low:       19950.0,
			Close:     20500.0,
			Volume:    123.456,
			CloseTime: 1695000059999,
		},
	}, nil
}

// helper to create a new mock
func NewMockBinanceClient() crypto.BinanceClient {
	return &MockBinanceClient{}
}
