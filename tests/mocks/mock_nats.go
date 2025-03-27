package mocks

import (
	"context"

	"infosir/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockNatsClient is a testify-based mock for the NatsClient interface.
type MockNatsClient struct {
	mock.Mock
}

// PublishKlines mocks the method to publish klines to a JetStream subject.
func (m *MockNatsClient) PublishKlines(ctx context.Context, klines []models.Kline) error {
	args := m.Called(ctx, klines)
	return args.Error(0)
}
