package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"infosir/cmd/config"
	"infosir/internal/jobs"
	"infosir/internal/model"
	"infosir/internal/srv"
	"infosir/pkg/crypto"
	"infosir/tests/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// simple logger stub
func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// sample config stub (replace with your actual config struct as needed)
var testConfig = &config.Config{
	Nats: config.NATSConfig{
		NatsURL:             "nats://127.0.0.1:4222",
		NatsSubject:         "infosir_kline",
		JetStreamStreamName: "infosir_kline_stream",
		JetStreamConsumer:   "infosir_kline_consumer",
	},
	Crypto: config.CryptoConfig{
		BinanceBaseURL:     "https://api.binance.com",
		BinanceKlinesPoint: "api/v3/klines",
		Pairs:              []string{"BTCUSDT"},
		KlineInterval:      "1m",
		KlineLimit:         1,
	},
	HTTPPort: 8080,
}

// Kline validation test
func TestKlineValidation(t *testing.T) {
	k := model.Kline{
		Time:       crypto.MsToTime(1234567890000),
		OpenPrice:  21000.55,
		HighPrice:  21050.10,
		LowPrice:   20900.15,
		ClosePrice: 21010.01,
		Volume:     12.3456,
	}

	err := k.Validate() // assume there's a Validate() method
	assert.NoError(t, err, "Valid Kline should pass validation")

	// an invalid example
	k.HighPrice = 0
	err2 := k.Validate()
	assert.Error(t, err2, "High=0 might be invalid if >0 is required")
}

func TestInfoSirService_GetKlines_Success(t *testing.T) {
	// 1) define scenario
	scenarioList := []mocks.Scenario{
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 5,
			ReturnKlines: []model.Kline{
				{Time: crypto.MsToTime(1000_000), OpenPrice: 1.0, HighPrice: 2.0, LowPrice: 0.5, ClosePrice: 1.5},
			},
		},
	}
	mockBinance := mocks.NewMockBinanceClient(scenarioList)

	natsScenarios := []mocks.NatsPublishScenario{
		{Subject: "infosir_kline", ReturnError: nil},
	}
	mockNats := mocks.NewMockNatsClient(natsScenarios)

	service := srv.NewInfoSirService(mockBinance, mockNats)
	// 2) actual test
	klines, err := service.GetKlines(context.Background(), "BTCUSDT", "1m", 5)
	require.NoError(t, err)
	require.Len(t, klines, 1)
	// check the content
	assert.Equal(t, int64(1000_000), klines[0].Time.UnixNano())

	// 3) verify mock calls
	assert.Len(t, mockBinance.Calls, 1)
	call := mockBinance.Calls[0]
	assert.Equal(t, "BTCUSDT", call.Pair)
	assert.Equal(t, "1m", call.Interval)
	assert.Equal(t, int64(5), call.Limit)
}

func TestInfoSirService_GetKlines_RetrySuccessOnThirdAttempt(t *testing.T) {
	// 1) scenarios: пусть первые 2 вызова вернут ошибку, 3-й вернёт success
	scenarios := []mocks.Scenario{
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 5,
			ReturnError: fmt.Errorf("attempt 1 error"),
		},
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 5,
			ReturnError: fmt.Errorf("attempt 2 error"),
		},
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 5,
			ReturnKlines: []model.Kline{
				{Time: crypto.MsToTime(1111_000)},
			},
			ReturnError: nil,
		},
	}
	mockBinance := mocks.NewMockBinanceClient(scenarios)

	// nats mock не используется в GetKlines, но пусть будет пустым
	mockNats := mocks.NewMockNatsClient(nil)

	service := srv.NewInfoSirService(mockBinance, mockNats)

	// 2) выполняем сам тест
	klines, err := service.GetKlines(context.Background(), "BTCUSDT", "1m", 5)

	// 3) проверяем, что на 3-й попытке вернулся успех
	require.NoError(t, err)
	require.Len(t, klines, 1)

	// Проверим, что mockBinance вызывался ровно 3 раза с одинаковыми параметрами
	require.Len(t, mockBinance.Calls, 3, "We expect 3 calls (2 fails, 1 success)")

	for i, c := range mockBinance.Calls {
		assert.Equal(t, "BTCUSDT", c.Pair)
		assert.Equal(t, "1m", c.Interval)
		assert.Equal(t, int64(5), c.Limit)
		// each call is the same param, but scenario differs in the result
		t.Logf("Call #%d: pair=%s, interval=%s, limit=%d", i+1, c.Pair, c.Interval, c.Limit)
	}
}

func TestInfoSirService_GetKlines_AllAttemptsFail(t *testing.T) {
	// допустим 3 сценария, все возвращают ошибку
	scenarios := []mocks.Scenario{
		{Pair: "BTCUSDT", Interval: "1m", Limit: 5, ReturnError: fmt.Errorf("attempt1 fail")},
		{Pair: "BTCUSDT", Interval: "1m", Limit: 5, ReturnError: fmt.Errorf("attempt2 fail")},
		{Pair: "BTCUSDT", Interval: "1m", Limit: 5, ReturnError: fmt.Errorf("attempt3 fail")},
	}
	mockBinance := mocks.NewMockBinanceClient(scenarios)
	mockNats := mocks.NewMockNatsClient(nil)

	service := srv.NewInfoSirService(mockBinance, mockNats)

	klines, err := service.GetKlines(context.Background(), "BTCUSDT", "1m", 5)
	require.Error(t, err, "expected final error after 3 fails")
	require.Nil(t, klines)
	assert.Contains(t, err.Error(), "could not fetch klines after 3 attempts")
	// verify calls length
	require.Len(t, mockBinance.Calls, 3)
}

func TestInfoSirService_PublishKlinesJS_Success(t *testing.T) {
	mockBinance := mocks.NewMockBinanceClient(nil)
	natsScenarios := []mocks.NatsPublishScenario{
		{Subject: "infosir_kline", ReturnError: nil},
	}
	mockNats := mocks.NewMockNatsClient(natsScenarios)

	service := srv.NewInfoSirService(mockBinance, mockNats)

	inputKlines := []model.Kline{
		{Time: crypto.MsToTime(1000_000), OpenPrice: 1.0, HighPrice: 2.0, LowPrice: 1.0, ClosePrice: 1.5},
	}

	err := service.PublishKlinesJS(context.Background(), inputKlines)
	require.NoError(t, err)

	// verify
	require.Len(t, mockNats.PublishCalls, 1)
	call := mockNats.PublishCalls[0]
	assert.Equal(t, "infosir_kline", call.Subject)

	// Декодируем JSON?
	// ...
}

func TestInfoSirService_PublishKlinesJS_Error(t *testing.T) {
	mockBinance := mocks.NewMockBinanceClient(nil)
	natsScenarios := []mocks.NatsPublishScenario{
		{Subject: "infosir_kline", ReturnError: fmt.Errorf("nats error: no space")},
	}
	mockNats := mocks.NewMockNatsClient(natsScenarios)

	service := srv.NewInfoSirService(mockBinance, mockNats)

	inputKlines := []model.Kline{
		{Time: crypto.MsToTime(100_000)},
	}
	err := service.PublishKlinesJS(context.Background(), inputKlines)
	require.Error(t, err)
	// confirm it has "could not publish" or similar
	assert.Contains(t, err.Error(), "could not publish klines")

	// check calls
	require.Len(t, mockNats.PublishCalls, 1)
	call := mockNats.PublishCalls[0]
	assert.Equal(t, "infosir_kline", call.Subject)
}

func TestRunScheduledRequests_SingleTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1) Mock InfoSirService
	mockSvc := &mocks.MockInfoSirService{
		GetKlinesFn: func(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
			return []model.Kline{{Time: crypto.MsToTime(111_000)}}, nil
		},
		PublishKlinesFn: func(ctx context.Context, kl []model.Kline) error {
			return nil
		},
	}

	// suppose config.Cfg.Crypto.Pairs has 2 pairs: [BTCUSDT, ETHUSDT]
	// or you can override them for the test.

	// 2) Запускаем джобу в горутине
	go func() {
		jobs.RunScheduledRequests(ctx, mockSvc, 50*time.Millisecond) // small interval for test
	}()

	// 3) ждем 120 ms => 2 тика
	time.Sleep(120 * time.Millisecond)
	cancel() // stop job

	// 4) проверить, что mockSvc.GetKlinesFn вызывался 2+2=4 раза? (2 pair * 2 ticks) — зависит от timing.
	//    но в реальности может быть 1 или 2 ticks, так что либо sync approach, либо relaxed check >= 2 calls

	t.Logf("CallsGetKlines = %d", len(mockSvc.CallsGetKlines))
	// etc: assert.GreaterOrEqual(t, len(mockSvc.CallsGetKlines), 2, "Expect at least 2 calls")
}

func TestRunHistoricalSync_Basic(t *testing.T) {
	ctx := context.Background()

	// 1) Mock repo
	mockRepo := mocks.NewMockKlineRepository(
		[]mocks.FindLastScenario{
			{
				TableName:   "btcusdt",
				ReturnKline: model.Kline{Time: crypto.MsToTime(1000_000)}, // pretend last known openTime is 1000
			},
		},
		[]mocks.BatchInsertScenario{
			{
				TableName:    "btcusdt",
				KlinesToFail: false,
			},
		})

	//mockRepo := &mocks.MockKlineRepository{
	//	FindLastScenarios: []mocks.FindLastScenario{
	//		{
	//			TableName:   "btcusdt",
	//			ReturnKline: model.Kline{OpenTime: 1000}, // pretend last known openTime is 1000
	//		},
	//	},
	//	BatchInsertScenarios: []mocks.BatchInsertScenario{
	//		{
	//			TableName:    "btcusdt",
	//			KlinesToFail: false,
	//		},
	//	},
	//}

	// 2) Mock binance
	mockBinance := &mocks.MockBinanceClient{
		Scenarios: []mocks.Scenario{
			// We'll presumably see multiple calls. For now let's define 2 calls:
			{Pair: "BTCUSDT", Interval: "1m", Limit: 720, ReturnKlines: []model.Kline{
				{Time: crypto.MsToTime(1100_000)}, {Time: crypto.MsToTime(1160_000)},
			}},
			// second time maybe empty => stop
			{Pair: "BTCUSDT", Interval: "1m", Limit: 720, ReturnKlines: []model.Kline{}},
		},
	}

	// 3) Запуск
	jobs.RunHistoricalSync(ctx, mockRepo, mockBinance)

	// 4) Проверяем, что FindLast вызывался для "btcusdt"
	require.Len(t, mockRepo.FindLastCalls, 1)
	assert.Equal(t, "btcusdt", mockRepo.FindLastCalls[0])

	// 5) Проверяем, что binanceClient вызвался >=2 раза (пока не получил пустые klines)
	calls := mockBinance.Calls
	require.Len(t, calls, 2)
	assert.Equal(t, "BTCUSDT", calls[0].Pair)
	assert.Equal(t, "1m", calls[0].Interval)
	assert.Equal(t, int64(720), calls[0].Limit)

	assert.Equal(t, "BTCUSDT", calls[1].Pair)
	assert.Equal(t, "1m", calls[1].Interval)
	assert.Equal(t, int64(720), calls[1].Limit)

	// 6) Check batch inserts
	require.Len(t, mockRepo.BatchInsertCalls, 1, "only 1 non-empty klines insert, second batch = empty => presumably not inserted or inserted but empty?")
	call1 := mockRepo.BatchInsertCalls[0]
	assert.Equal(t, "btcusdt", call1.TableName)
	assert.Equal(t, 2, call1.Count)
}
