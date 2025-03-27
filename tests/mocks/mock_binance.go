package mocks

import (
	"context"

	"infosir/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockBinanceClient is a testify-based mock for the BinanceClient interface.
type MockBinanceClient struct {
	mock.Mock
}

// FetchKlines is the mock implementation for fetching klines from an exchange.
func (m *MockBinanceClient) FetchKlines(
	ctx context.Context,
	pair, interval string,
	limit int64,
) ([]models.Kline, error) {

	args := m.Called(ctx, pair, interval, limit)
	// The return signature is ([]model.Kline, error)
	klines, _ := args.Get(0).([]models.Kline)
	return klines, args.Error(1)
}
