package mocks

import (
	"context"

	"infosir/internal/models"
	"infosir/internal/srv"

	"github.com/stretchr/testify/mock"
)

// Ensure MockInfoSirService implements the InfoSirService interface
var _ srv.InfoSirService = (*MockInfoSirService)(nil)

// MockInfoSirService is a testify-based mock for the InfoSirService interface.
type MockInfoSirService struct {
	mock.Mock
}

// GetKlines mocks the retrieval of klines from the underlying binance-like client.
func (m *MockInfoSirService) GetKlines(
	ctx context.Context,
	pair string,
	interval string,
	limit int64,
) ([]models.Kline, error) {

	args := m.Called(ctx, pair, interval, limit)
	klines, _ := args.Get(0).([]models.Kline)
	return klines, args.Error(1)
}

// PublishKlinesJS mocks the publishing of klines to NATS JetStream.
func (m *MockInfoSirService) PublishKlinesJS(
	ctx context.Context,
	klines []models.Kline,
) error {
	args := m.Called(ctx, klines)
	return args.Error(0)
}
