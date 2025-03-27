package mocks

import (
	"context"

	"infosir/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockKlineRepository is a testify-based mock for a repository that stores klines.
type MockKlineRepository struct {
	mock.Mock
}

// InsertKline mocks inserting a single Kline.
func (m *MockKlineRepository) InsertKline(ctx context.Context, k models.Kline) error {
	args := m.Called(ctx, k)
	return args.Error(0)
}

// BatchInsertKlines mocks inserting multiple klines in one batch.
func (m *MockKlineRepository) BatchInsertKlines(ctx context.Context, klines []models.Kline) error {
	args := m.Called(ctx, klines)
	return args.Error(0)
}

// FindLast mocks retrieving the most recent Kline for the given symbol.
func (m *MockKlineRepository) FindLast(ctx context.Context, symbol string) (models.Kline, error) {
	args := m.Called(ctx, symbol)
	k, _ := args.Get(0).(models.Kline)
	return k, args.Error(1)
}
