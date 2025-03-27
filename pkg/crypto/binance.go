package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"infosir/internal/models"
	"infosir/internal/utils"

	"go.uber.org/zap"
)

// binanceClientImpl is a concrete implementation of a Binance-like client.
type binanceClientImpl struct {
	httpClient *http.Client
	baseURL    string
	klinesPath string
}

// NewBinanceClient constructs a new Binance-like client using default baseURL and path from config.
func NewBinanceClient() *binanceClientImpl {
	cfg := utils.GetConfig().Crypto

	return &binanceClientImpl{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:    cfg.BinanceBaseURL,
		klinesPath: cfg.BinanceKlinesPoint,
	}
}

// FetchKlines retrieves up to 'limit' klines for the given trading pair and interval from the Binance API.
// The returned slice of model.Kline is in ascending time order.
func (b *binanceClientImpl) FetchKlines(
	ctx context.Context,
	pair, interval string,
	limit int64,
) ([]models.Kline, error) {

	endpoint := fmt.Sprintf(
		"%s/%s?symbol=%s&interval=%s&limit=%d",
		b.baseURL,
		b.klinesPath,
		pair,
		interval,
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchKlines received status %d from binance", resp.StatusCode)
	}

	// Binance returns klines as an array of arrays:
	// e.g. [
	//   [ 1499040000000, "0.01634790", "0.80000000", "0.01575800", "0.01577100", "148976.11427815", ... ],
	//   ...
	// ]
	// We'll parse them accordingly.
	var rawKlines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
		return nil, fmt.Errorf("failed to decode binance klines JSON: %w", err)
	}

	klines := make([]models.Kline, 0, len(rawKlines))

	for _, raw := range rawKlines {
		// We expect each raw to have length >= 11, e.g. openTime, openPrice, highPrice, lowPrice, closePrice, volume ...
		// For reference, see Binance docs for the full structure
		if len(raw) < 11 {
			utils.Logger.Warn("Skipping malformed kline record", zap.Any("raw", raw))
			continue
		}

		openTimeMs, _ := toInt64(raw[0])
		openStr, _ := toString(raw[1])
		highStr, _ := toString(raw[2])
		lowStr, _ := toString(raw[3])
		closeStr, _ := toString(raw[4])
		volStr, _ := toString(raw[5])
		// index 6 is quote volume
		quoteVolStr, _ := toString(raw[7])
		tradesCount, _ := toInt64(raw[8])
		takerBuyBaseStr, _ := toString(raw[9])
		takerBuyQuoteStr, _ := toString(raw[10])

		// Convert strings to float64
		openF, _ := strconv.ParseFloat(openStr, 64)
		highF, _ := strconv.ParseFloat(highStr, 64)
		lowF, _ := strconv.ParseFloat(lowStr, 64)
		closeF, _ := strconv.ParseFloat(closeStr, 64)
		volF, _ := strconv.ParseFloat(volStr, 64)
		quoteVolF, _ := strconv.ParseFloat(quoteVolStr, 64)
		takerBuyBaseF, _ := strconv.ParseFloat(takerBuyBaseStr, 64)
		takerBuyQuoteF, _ := strconv.ParseFloat(takerBuyQuoteStr, 64)

		openTime := time.Unix(0, openTimeMs*int64(time.Millisecond))

		k := models.Kline{
			Time:                openTime,
			Symbol:              pair, // we store the exact pair as given
			OpenPrice:           openF,
			HighPrice:           highF,
			LowPrice:            lowF,
			ClosePrice:          closeF,
			Volume:              volF,
			QuoteVolume:         quoteVolF,
			Trades:              tradesCount,
			TakerBuyBaseVolume:  takerBuyBaseF,
			TakerBuyQuoteVolume: takerBuyQuoteF,
		}
		klines = append(klines, k)
	}

	utils.Logger.Debug("Fetched klines from binance",
		zap.String("pair", pair),
		zap.Int("count", len(klines)),
	)

	return klines, nil
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case float32:
		return int64(val), true
	case int:
		return int64(val), true
	case int64:
		return val, true
	case string:
		res, err := strconv.ParseInt(val, 10, 64)
		return res, (err == nil)
	default:
		return 0, false
	}
}

func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	default:
		return fmt.Sprintf("%v", v), true
	}
}
