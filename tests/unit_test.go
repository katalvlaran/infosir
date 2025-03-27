package tests

import (
	"context"
	"testing"

	"infosir/internal/models"
	"infosir/internal/srv"
	"infosir/tests/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestInfoSirService_GetKlines is an example unit test that verifies
// InfoSirService’s GetKlines method using a mocked BinanceClient.
func TestInfoSirService_GetKlines(t *testing.T) {
	// 1. Create a context
	ctx := context.Background()

	// 2. Setup mock dependencies
	mockBinance := &mocks.MockBinanceClient{}
	mockNats := &mocks.MockNatsClient{} // not used in this test, but we pass in to satisfy the constructor

	// 3. Construct the service with the mocks
	service := srv.NewInfoSirService(mockBinance, mockNats)

	// 4. Define test input and expected output
	pair := "BTCUSDT"
	interval := "1m"
	limit := int64(5)
	expectedKlines := []models.Kline{
		{
			Symbol:     pair,
			OpenPrice:  100.0,
			ClosePrice: 110.0,
		},
		{
			Symbol:     pair,
			OpenPrice:  110.0,
			ClosePrice: 120.0,
		},
	}

	// 5. Setup your mock with expected call
	mockBinance.
		On("FetchKlines", mock.Anything, pair, interval, limit).
		Return(expectedKlines, nil)

	// 6. Call the method
	actual, err := service.GetKlines(ctx, pair, interval, limit)

	// 7. Validate
	assert.NoError(t, err, "GetKlines should not return an error")
	assert.Equal(t, expectedKlines, actual, "Klines should match the expected slice")

	// 8. Assert mock expectations
	mockBinance.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}

// TestInfoSirService_PublishKlinesJS is an example of testing the publish method using a mock.
func TestInfoSirService_PublishKlinesJS(t *testing.T) {
	ctx := context.Background()
	mockBinance := &mocks.MockBinanceClient{} // not used in this test
	mockNats := &mocks.MockNatsClient{}

	service := srv.NewInfoSirService(mockBinance, mockNats)

	klines := []models.Kline{
		{Symbol: "ETHUSDT", ClosePrice: 200.0},
	}

	// We expect the nats mock to be called with these klines
	mockNats.
		On("PublishKlines", mock.Anything, klines).
		Return(nil)

	err := service.PublishKlinesJS(ctx, klines)
	assert.NoError(t, err, "PublishKlinesJS should not fail")

	mockBinance.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}
