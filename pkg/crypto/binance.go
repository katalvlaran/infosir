package crypto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"infosir/internal/models"
	"infosir/internal/srv"
	"infosir/internal/utils"
)

const maxResponseBytes int64 = 4 << 20

// ClientOptions fixes the market profile explicitly. Custom hosts permit testnet
// and local fixtures, but the endpoint path must match the declared market.
type ClientOptions struct {
	BaseURL     string
	KlinesPath  string
	Market      string // spot or usdm
	HTTPClient  *http.Client
	Clock       func() time.Time
	FinalityLag time.Duration // REST inferred closure, never observed WS finality.
}

type binanceClientImpl struct {
	httpClient  *http.Client
	endpoint    *url.URL
	market      string
	maxLimit    int
	initErr     error
	clock       func() time.Time
	finalityLag time.Duration
}

// NewBinanceClient preserves the legacy constructor. Invalid configuration is
// reported by every request; new callers should use the checked constructor.
func NewBinanceClient() *binanceClientImpl {
	cfg := utils.GetConfig().Crypto
	c, err := NewBinanceClientWithOptions(ClientOptions{
		BaseURL: cfg.BinanceBaseURL, KlinesPath: cfg.BinanceKlinesPoint, Market: cfg.Market, FinalityLag: 2 * time.Second,
	})
	if err != nil {
		return &binanceClientImpl{initErr: err}
	}
	return c
}

// NewBinanceClientWithOptions validates a market-specific, UTC trade-kline adapter.
func NewBinanceClientWithOptions(o ClientOptions) (*binanceClientImpl, error) {
	if o.FinalityLag < 0 {
		return nil, srv.ErrInvalidQuery
	}
	if o.Clock == nil {
		o.Clock = time.Now
	}
	profile := map[string]string{"spot": "api/v3/klines", "usdm": "fapi/v1/klines"}
	path, ok := profile[o.Market]
	if !ok || strings.Trim(o.KlinesPath, "/") != path {
		return nil, fmt.Errorf("%w: market/endpoint mismatch", srv.ErrInvalidQuery)
	}
	u, err := url.Parse(o.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("%w: invalid base URL", srv.ErrInvalidQuery)
	}
	u.Path = "/" + path
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// Keep an upper bound even when a caller injects a client without a timeout.
	copyClient := *client
	if copyClient.Timeout <= 0 {
		copyClient.Timeout = 10 * time.Second
	}
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	limit := 1000
	if o.Market == "usdm" {
		limit = 1500
	}
	return &binanceClientImpl{httpClient: &copyClient, endpoint: u, market: o.Market, maxLimit: limit, clock: o.Clock, finalityLag: o.FinalityLag}, nil
}

// Market returns the configured market profile, allowing lifecycle diagnostics.
func (b *binanceClientImpl) Market() string { return b.market }

func canonicalSymbol(symbol string) (string, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if len(symbol) == 0 || len(symbol) > 64 {
		return "", srv.ErrInvalidQuery
	}
	for _, c := range symbol {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return "", srv.ErrInvalidQuery
		}
	}
	return symbol, nil
}

func (b *binanceClientImpl) validate(symbol, interval string, limit int) (string, error) {
	if b.initErr != nil {
		return "", b.initErr
	}
	if b.endpoint == nil || limit < 1 || limit > b.maxLimit {
		return "", srv.ErrInvalidQuery
	}
	if b.market == "usdm" && interval == "1s" {
		return "", fmt.Errorf("%w: 1s unsupported by usdm", srv.ErrInvalidQuery)
	}
	if _, err := srv.IntervalStart(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), interval); err != nil {
		return "", err
	}
	return canonicalSymbol(symbol)
}

// FetchKlines returns only bars whose close boundary has elapsed before the
// captured local clock minus FinalityLag. This is inferred REST closure, not
// observed finality, and the legacy result cannot represent later corrections.
func (b *binanceClientImpl) FetchKlines(ctx context.Context, pair, interval string, limit int64) ([]models.Kline, error) {
	if limit < 1 || limit > 1500 {
		return nil, srv.ErrInvalidQuery
	}
	symbol, err := b.validate(pair, interval, int(limit))
	if err != nil {
		return nil, err
	}
	cutoff := b.clock().UTC().Add(-b.finalityLag)
	page, err := b.fetch(ctx, symbol, interval, int(limit), nil)
	if err != nil {
		return nil, err
	}
	closed := make([]models.Kline, 0, len(page.Klines))
	for i, k := range page.Klines {
		if !page.CloseExclusive[i].After(cutoff) {
			closed = append(closed, k)
		}
	}
	return closed, nil
}

// FetchRange selects only open times in [Start, End). REST finality remains an
// assumption outside this adapter: CloseExclusive is a boundary, not a flag.
func (b *binanceClientImpl) FetchRange(ctx context.Context, q srv.RangeQuery) (srv.RangePage, error) {
	if err := srv.ValidateRange(q); err != nil {
		return srv.RangePage{}, err
	}
	symbol, err := b.validate(q.Symbol, q.Interval, q.Limit)
	if err != nil {
		return srv.RangePage{}, err
	}
	if q.End.After(b.clock().UTC().Add(-b.finalityLag)) {
		return srv.RangePage{}, fmt.Errorf("%w: range extends beyond inferred closed horizon", srv.ErrInvalidQuery)
	}
	return b.fetch(ctx, symbol, q.Interval, q.Limit, &q)
}

func (b *binanceClientImpl) fetch(ctx context.Context, symbol, interval string, limit int, q *srv.RangeQuery) (srv.RangePage, error) {
	u := *b.endpoint
	query := url.Values{"symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	if q != nil {
		query.Set("startTime", strconv.FormatInt(q.Start.UnixMilli(), 10))
		query.Set("endTime", strconv.FormatInt(q.End.UnixMilli()-1, 10))
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return srv.RangePage{}, err
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return srv.RangePage{}, fmt.Errorf("market-data request: %w", err)
	}
	defer resp.Body.Close()
	headersReceived := b.clock().UTC()
	if resp.StatusCode != http.StatusOK {
		return srv.RangePage{}, &srv.SourceError{StatusCode: resp.StatusCode,
			RetryAfter: retryAfter(resp.Header.Get("Retry-After"), headersReceived),
			Transient:  resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return srv.RangePage{}, fmt.Errorf("read market-data response: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return srv.RangePage{}, fmt.Errorf("%w: response too large", srv.ErrInvalidPayload)
	}
	page, err := parseKlines(body, symbol, interval, limit)
	if err != nil {
		return srv.RangePage{}, err
	}
	// Receipt is the completed, validated payload, not the earlier HTTP headers.
	received := b.clock().UTC()
	page.ReceivedAt = received
	if q == nil {
		return page, nil
	}
	out := srv.RangePage{ReceivedAt: received, NextOpen: q.Start}
	for i, k := range page.Klines {
		// A boundary leak is rejected instead of silently treating a malformed
		// range response as a complete page.
		if k.Time.Before(q.Start) || !k.Time.Before(q.End) {
			return srv.RangePage{}, fmt.Errorf("%w: row outside requested range", srv.ErrInvalidPayload)
		}
		out.Klines = append(out.Klines, k)
		out.CloseExclusive = append(out.CloseExclusive, page.CloseExclusive[i])
	}
	if len(out.Klines) > 0 {
		out.NextOpen = out.CloseExclusive[len(out.CloseExclusive)-1]
	}
	return out, nil
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}

func parseKlines(body []byte, symbol, interval string, limit int) (srv.RangePage, error) {
	var rows [][]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&rows); err != nil {
		return srv.RangePage{}, fmt.Errorf("%w: JSON shape", srv.ErrInvalidPayload)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return srv.RangePage{}, fmt.Errorf("%w: trailing JSON", srv.ErrInvalidPayload)
	}
	if rows == nil || len(rows) > limit {
		return srv.RangePage{}, fmt.Errorf("%w: row count", srv.ErrInvalidPayload)
	}
	page := srv.RangePage{Klines: make([]models.Kline, 0, len(rows)), CloseExclusive: make([]time.Time, 0, len(rows))}
	for i, row := range rows {
		if len(row) != 12 {
			return srv.RangePage{}, fmt.Errorf("%w: row %d tuple length", srv.ErrInvalidPayload, i)
		}
		open, err := parseInteger(row[0])
		if err != nil {
			return srv.RangePage{}, rowError(i, "open time")
		}
		close, err := parseInteger(row[6])
		if err != nil {
			return srv.RangePage{}, rowError(i, "close time")
		}
		trades, err := parseInteger(row[8])
		if err != nil || trades < 0 {
			return srv.RangePage{}, rowError(i, "trades")
		}
		if open < 0 || close < 0 || open > 253402300799999 || close >= 253402300799999 {
			return srv.RangePage{}, rowError(i, "timestamp bounds")
		}
		openTime := time.UnixMilli(open).UTC()
		next, err := srv.NextOpen(openTime, interval)
		if err != nil || next.UnixMilli()-1 != close {
			return srv.RangePage{}, rowError(i, "interval boundary")
		}
		values := make([]float64, 8)
		for j, index := range []int{1, 2, 3, 4, 5, 7, 9, 10} {
			values[j], err = parseDecimal(row[index])
			if err != nil {
				return srv.RangePage{}, rowError(i, "decimal field")
			}
		}
		k := models.Kline{Time: openTime, Symbol: symbol, OpenPrice: values[0], HighPrice: values[1], LowPrice: values[2], ClosePrice: values[3], Volume: values[4], QuoteVolume: values[5], Trades: trades, TakerBuyBaseVolume: values[6], TakerBuyQuoteVolume: values[7]}
		if err := k.Validate(); err != nil {
			return srv.RangePage{}, fmt.Errorf("%w: row %d: %v", srv.ErrInvalidPayload, i, err)
		}
		if len(page.Klines) > 0 {
			last := page.Klines[len(page.Klines)-1]
			if k.Time.Before(last.Time) {
				return srv.RangePage{}, rowError(i, "row order")
			}
			if k.Time.Equal(last.Time) {
				// Compare raw semantic fields before float conversion to avoid
				// hiding distinct decimals that round to the same float64.
				if !sameRow(row, rows[i-1]) {
					return srv.RangePage{}, rowError(i, "conflicting duplicate")
				}
				continue
			}
		}
		page.Klines = append(page.Klines, k)
		page.CloseExclusive = append(page.CloseExclusive, next)
	}
	if len(page.CloseExclusive) > 0 {
		page.NextOpen = page.CloseExclusive[len(page.CloseExclusive)-1]
	}
	return page, nil
}

func sameRow(a, b []json.RawMessage) bool {
	for i := 0; i < 11; i++ {
		if !bytes.Equal(bytes.TrimSpace(a[i]), bytes.TrimSpace(b[i])) {
			return false
		}
	}
	return true
}
func rowError(index int, field string) error {
	return fmt.Errorf("%w: row %d %s", srv.ErrInvalidPayload, index, field)
}
func parseInteger(raw json.RawMessage) (int64, error) { return strconv.ParseInt(string(raw), 10, 64) }
func parseDecimal(raw json.RawMessage) (float64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || len(s) == 0 || len(s) > 128 {
		return 0, srv.ErrInvalidPayload
	}
	// Exchange decimal fields are decimal strings, not exponent/NaN literals.
	dot, digits := false, 0
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' && !dot {
			dot = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, srv.ErrInvalidPayload
		}
		digits++
	}
	if digits == 0 {
		return 0, srv.ErrInvalidPayload
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, srv.ErrInvalidPayload
	}
	return v, nil
}
