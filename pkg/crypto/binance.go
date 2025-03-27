package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"infosir/cmd/config"
	"infosir/internal/model"
)

// BinanceClient определяет методы для работы с публичными эндпоинтами Binance
type BinanceClient interface {
	FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
	FetchKlinesRange(ctx context.Context, pair, interval string, startTime, endTime int64, limit int64) ([]model.Kline, error)
}

// binanceClientImpl — реализация интерфейса BinanceClient
type binanceClientImpl struct {
	baseURL    string
	klinePoint string
	httpClient *http.Client
}

// NewBinanceClient — конструктор, принимающий базовый URL (например, "https://api.binance.com")
// и создающий http.Client с дефолтным таймаутом (можно расширить при необходимости).
func NewBinanceClient() BinanceClient {
	return &binanceClientImpl{
		baseURL:    config.Cfg.Crypto.BinanceBaseURL,
		klinePoint: config.Cfg.Crypto.BinanceKlinesPoint,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Подходящий таймаут
		},
	}
}

// FetchKlines отправляет GET-запрос на /fapi/v1/klines?symbol=...&interval=...&limit=...
// и формирует массив []model.Kline. Возвращает ошибку при недоступности сети/неправильном ответе.
func (b *binanceClientImpl) FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
	url := fmt.Sprintf("%s%s?symbol=%s&interval=%s&limit=%d",
		b.baseURL, b.klinePoint, pair, interval, limit)

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
	//  [0] -> time (int64)
	//  [1] -> symbol (string -> float64)
	//  [2] -> open (string -> float64)
	//  [3] -> high (string -> float64)
	//  [4] -> low (string -> float64)
	//  [5] -> close (string -> float64)
	//  [6] -> volume (int64)
	//  [7] -> quote_volume (float64)
	//  [8] -> trades (int64)
	//  [9] -> taker_buy_base_volume (float64)
	//  [10] -> taker_buy_quote_volume (float64)
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

		time, _ := toInt64(item[0])
		symbol, _ := toString(item[1])
		openP, _ := toFloat64(item[2])
		highP, _ := toFloat64(item[3])
		lowP, _ := toFloat64(item[4])
		closeP, _ := toFloat64(item[5])
		volume, _ := toFloat64(item[6])
		quote_volume, _ := toFloat64(item[7])
		trades, _ := toInt64(item[8])
		taker_buy_base_volume, _ := toFloat64(item[9])
		taker_buy_quote_volume, _ := toFloat64(item[10])

		k := model.Kline{
			Time:                MsToTime(time),
			Symbol:              symbol,
			OpenPrice:           openP,
			HighPrice:           highP,
			LowPrice:            lowP,
			ClosePrice:          closeP,
			Volume:              volume,
			QuoteVolume:         quote_volume,
			Trades:              trades,
			TakerBuyBaseVolume:  taker_buy_base_volume,
			TakerBuyQuoteVolume: taker_buy_quote_volume,
		}
		klines = append(klines, k)
	}

	return klines, nil
}

func (b *binanceClientImpl) FetchKlinesRange(ctx context.Context, pair, interval string, startTime, endTime int64, limit int64) ([]model.Kline, error) {
	url := fmt.Sprintf("%s%s?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=%d",
		b.baseURL, b.klinePoint, pair, interval, startTime, endTime, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request to binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API returned non-200: %d", resp.StatusCode)
	}

	var rawKlines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
		return nil, fmt.Errorf("failed to decode binance klines: %w", err)
	}

	klines := make([]model.Kline, 0, len(rawKlines))
	for _, item := range rawKlines {
		if len(item) < 7 {
			continue
		}
		time, _ := toInt64(item[0])
		symbol, _ := toString(item[1])
		openP, _ := toFloat64(item[2])
		highP, _ := toFloat64(item[3])
		lowP, _ := toFloat64(item[4])
		closeP, _ := toFloat64(item[5])
		volume, _ := toFloat64(item[6])
		quote_volume, _ := toFloat64(item[7])
		trades, _ := toInt64(item[8])
		taker_buy_base_volume, _ := toFloat64(item[9])
		taker_buy_quote_volume, _ := toFloat64(item[10])

		klines = append(klines, model.Kline{
			Time:                MsToTime(time),
			Symbol:              symbol,
			OpenPrice:           openP,
			HighPrice:           highP,
			LowPrice:            lowP,
			ClosePrice:          closeP,
			Volume:              volume,
			QuoteVolume:         quote_volume,
			Trades:              trades,
			TakerBuyBaseVolume:  taker_buy_base_volume,
			TakerBuyQuoteVolume: taker_buy_quote_volume,
		})
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

// toString пытается преобразовать значение из JSON (float64, string, int) к string.
func toString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	default:
		return "", fmt.Errorf("unexpected type for float64 conversion: %T", v)
	}
}

func MsToTime(ms int64) time.Time {
	return time.Unix(ms/1000, (ms%1000)*1_000_000).UTC()
}
