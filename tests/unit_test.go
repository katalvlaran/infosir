package tests

import (
	"context"
	"testing"

	"infosir/cmd/config"
	"infosir/internal/model"
	"infosir/internal/srv"
	"infosir/pkg/crypto"
	"infosir/pkg/nats"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// simple logger stub
func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// sample config stub (replace with your actual config struct as needed)
var testConfig = &config.Config{
	NatsURL:        "nats://127.0.0.1:4222",
	NatsSubject:    "infosir_kline",
	BinanceBaseURL: "https://api.binance.com",
	Pairs:          []string{"BTCUSDT"},
	KlineInterval:  "1m",
	KlineLimit:     5,
	HTTPPort:       8080,
}

// Kline validation test
func TestKlineValidation(t *testing.T) {
	k := model.Kline{
		OpenTime:  1234567890000,
		Open:      21000.55,
		High:      21050.10,
		Low:       20900.15,
		Close:     21010.01,
		Volume:    12.3456,
		CloseTime: 1234567895999,
	}

	err := k.Validate() // assume there's a Validate() method
	assert.NoError(t, err, "Valid Kline should pass validation")

	// an invalid example
	k.High = 0
	err2 := k.Validate()
	assert.Error(t, err2, "High=0 might be invalid if >0 is required")
}

// sample service test
func TestInfoSirService_GetKlines_Real(t *testing.T) {
	// NOTE: This requires a real or test binance environment or mocking

	binanceClient := crypto.NewBinanceClient(testConfig.BinanceBaseURL)
	natsClient := natsinfosir.NewNatsClient(nil) // if your natsClient takes *nats.Conn param or something
	logger := testLogger()

	service := srv.NewInfoSirService(logger, testConfig, binanceClient, natsClient)

	klines, err := service.GetKlines(context.Background(), "BTCUSDT", "1m", 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, klines, "Should fetch at least 1 Kline from real Binance (if net is up)")
}
