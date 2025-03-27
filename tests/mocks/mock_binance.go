package mocks

import (
	"context"
	"fmt"

	"infosir/internal/model"
)

// Scenario описывает конкретный сценарий: какие входные данные -> какой выход (klines/err)
type Scenario struct {
	Pair     string
	Interval string
	Limit    int64

	ReturnKlines []model.Kline
	ReturnError  error
}

type MockBinanceClient struct {
	// scenarios — список или map для удобства
	Scenarios []Scenario

	// Для отладки: можем хранить "фактические" вызовы
	Calls []struct {
		Pair     string
		Interval string
		Limit    int64
	}
}

func NewMockBinanceClient(scenarios []Scenario) *MockBinanceClient {
	return &MockBinanceClient{Scenarios: scenarios}
}

func (m *MockBinanceClient) FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
	// Логируем вызов
	m.Calls = append(m.Calls, struct {
		Pair     string
		Interval string
		Limit    int64
	}{pair, interval, limit})

	// Ищем подходящий сценарий
	for _, sc := range m.Scenarios {
		if sc.Pair == pair && sc.Interval == interval && sc.Limit == limit {
			return sc.ReturnKlines, sc.ReturnError
		}
	}

	// Если не нашли, можно вернуть default
	return nil, fmt.Errorf("no mock scenario found for pair=%s interval=%s limit=%d", pair, interval, limit)
}

/*
	scenarioList := []mocks.Scenario{
	    {
	        Pair: "BTCUSDT", Interval: "1m", Limit: 5,
	        ReturnKlines: []model.Kline{
	            {OpenTime: 1000, CloseTime: 1059}, // ...
	        },
	        ReturnError: nil,
	    },
	    {
	        Pair: "BTCUSDT", Interval: "1m", Limit: 10,
	        // допустим хотим вернуть ошибку
	        ReturnKlines: nil,
	        ReturnError: fmt.Errorf("simulate network fail"),
	    },
	}

mockBinance := mocks.NewMockBinanceClient(scenarioList)
*/
func (m *MockBinanceClient) FetchKlinesRange(ctx context.Context, pair, interval string, startTime, endTime int64, limit int64) ([]model.Kline, error) {
	//TODO implement me
	panic("implement me")
}
