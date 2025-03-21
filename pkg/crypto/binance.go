package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"infosir/internal/model"
)

// BinanceClient определяет методы для работы с публичными эндпоинтами Binance
type BinanceClient interface {
	FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
}

// binanceClientImpl — реализация интерфейса BinanceClient
type binanceClientImpl struct {
	baseURL    string
	httpClient *http.Client
}

// NewBinanceClient — конструктор, принимающий базовый URL (например, "https://api.binance.com")
// и создающий http.Client с дефолтным таймаутом (можно расширить при необходимости).
func NewBinanceClient(baseURL string) BinanceClient {
	return &binanceClientImpl{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Подходящий таймаут
		},
	}
}

// FetchKlines отправляет GET-запрос на /api/v3/klines?symbol=...&interval=...&limit=...
// и формирует массив []model.Kline. Возвращает ошибку при недоступности сети/неправильном ответе.
func (b *binanceClientImpl) FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
	// Пример URL: https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1m&limit=10
	url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		b.baseURL, pair, interval, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request to binance: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа (ожидаем 200)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API returned non-200 status: %d", resp.StatusCode)
	}

	// Парсим тело ответа
	// Пример структуры ответа: [][]interface{}
	// Нам нужны поля:
	//  [0] -> openTime (int64)
	//  [1] -> open (string -> float64)
	//  [2] -> high (string -> float64)
	//  [3] -> low (string -> float64)
	//  [4] -> close (string -> float64)
	//  [5] -> volume (string -> float64)
	//  [6] -> closeTime (int64)
	var rawKlines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
		return nil, fmt.Errorf("failed to decode binance klines: %w", err)
	}

	klines := make([]model.Kline, 0, len(rawKlines))
	for _, item := range rawKlines {
		// Проверяем длину массива item
		if len(item) < 7 {
			continue // некорректный формат
		}

		openTime, _ := toInt64(item[0])
		open, _ := toFloat64(item[1])
		high, _ := toFloat64(item[2])
		low, _ := toFloat64(item[3])
		closeVal, _ := toFloat64(item[4])
		volume, _ := toFloat64(item[5])
		closeTime, _ := toInt64(item[6])

		k := model.Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeVal,
			Volume:    volume,
			CloseTime: closeTime,
		}
		klines = append(klines, k)
	}

	return klines, nil
}

// toInt64 пытается преобразовать значение из JSON (float64, string, int) к int64.
func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case int:
		return int64(val), nil
	default:
		return 0, fmt.Errorf("unexpected type for int64 conversion: %T", v)
	}
}

// toFloat64 пытается преобразовать значение из JSON (float64, string, int) к float64.
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	case int:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unexpected type for float64 conversion: %T", v)
	}
}
