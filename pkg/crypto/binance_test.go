package crypto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"infosir/internal/srv"
)

var fixtureStart = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func fixtureRow(at time.Time, interval string) []any {
	next, _ := srv.NextOpen(at, interval)
	return []any{at.UnixMilli(), "10", "12", "9", "11", "0", next.UnixMilli() - 1, "0", int64(0), "0", "0", "0"}
}
func fixtureBody(rows ...[]any) string { b, _ := json.Marshal(rows); return string(b) }
func localClient(t *testing.T, handler http.HandlerFunc) *binanceClientImpl {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := NewBinanceClientWithOptions(ClientOptions{BaseURL: server.URL, KlinesPath: "api/v3/klines", Market: "spot", Clock: func() time.Time { return fixtureStart.Add(2 * time.Minute) }, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFetchRangeUsesHalfOpenBounds(t *testing.T) {
	calls := 0
	c := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/api/v3/klines" || q.Get("symbol") != "BTCUSDT" || q.Get("startTime") != fmt.Sprint(fixtureStart.UnixMilli()) || q.Get("endTime") != fmt.Sprint(fixtureStart.Add(time.Minute).UnixMilli()-1) {
			t.Errorf("unexpected range request %s", r.URL)
		}
		fmt.Fprint(w, fixtureBody(fixtureRow(fixtureStart, "1m")))
	})
	p, err := c.FetchRange(context.Background(), srv.RangeQuery{Symbol: "btcusdt", Interval: "1m", Start: fixtureStart, End: fixtureStart.Add(time.Minute), Limit: 5})
	if err != nil || len(p.Klines) != 1 || !p.NextOpen.Equal(fixtureStart.Add(time.Minute)) || p.Klines[0].Trades != 0 || p.Klines[0].Volume != 0 || calls != 1 {
		t.Fatalf("page=%+v err=%v calls=%d", p, err, calls)
	}
}

func TestLatestOmitsUnclosedBars(t *testing.T) {
	c := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("startTime") || r.URL.Query().Has("endTime") {
			t.Error("latest request gained hidden range")
		}
		fmt.Fprint(w, fixtureBody(fixtureRow(fixtureStart.Add(time.Minute), "1m"), fixtureRow(fixtureStart.Add(2*time.Minute), "1m")))
	})
	got, err := c.FetchKlines(context.Background(), "BTCUSDT", "1m", 10)
	if err != nil || len(got) != 1 || !got[0].Time.Equal(fixtureStart.Add(time.Minute)) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	c.finalityLag = time.Second
	got, err = c.FetchKlines(context.Background(), "BTCUSDT", "1m", 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("lag not respected: %+v %v", got, err)
	}
}

func TestStrictPayloadValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func([]any) []any
	}{
		{"short", func(r []any) []any { return r[:10] }},
		{"badDecimal", func(r []any) []any { r[1] = "garbage"; return r }},
		{"nan", func(r []any) []any { r[1] = "NaN"; return r }},
		{"infinity", func(r []any) []any { r[1] = "Infinity"; return r }},
		{"numericPrice", func(r []any) []any { r[1] = 10; return r }},
		{"invertedOHLC", func(r []any) []any { r[2] = "8"; return r }},
		{"negativeVolume", func(r []any) []any { r[5] = "-1"; return r }},
		{"fractionalTrades", func(r []any) []any { r[8] = 1.5; return r }},
		{"fractionalTime", func(r []any) []any { r[0] = float64(fixtureStart.UnixMilli()) + .5; return r }},
		{"negativeTime", func(r []any) []any { r[0] = int64(-1); return r }},
		{"badClose", func(r []any) []any { r[6] = fixtureStart.Add(time.Minute).UnixMilli(); return r }},
		{"excessTaker", func(r []any) []any { r[9] = "1"; return r }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseKlines([]byte(fixtureBody(tc.mutate(fixtureRow(fixtureStart, "1m")))), "BTCUSDT", "1m", 10)
			if !errors.Is(err, srv.ErrInvalidPayload) {
				t.Fatalf("expected invalid payload, got %v", err)
			}
		})
	}
	for _, body := range []string{"null", "{}", "[] []", "["} {
		if _, err := parseKlines([]byte(body), "BTCUSDT", "1m", 10); !errors.Is(err, srv.ErrInvalidPayload) {
			t.Errorf("accepted %q: %v", body, err)
		}
	}
}

func TestIntegerPrecisionAndDuplicateCollisions(t *testing.T) {
	row := fixtureRow(fixtureStart, "1m")
	row[8] = int64(9007199254740993)
	p, err := parseKlines([]byte(fixtureBody(row, row)), "BTCUSDT", "1m", 10)
	if err != nil || len(p.Klines) != 1 || p.Klines[0].Trades != 9007199254740993 {
		t.Fatalf("precision/dedup %+v %v", p, err)
	}
	a, b := fixtureRow(fixtureStart, "1m"), fixtureRow(fixtureStart, "1m")
	a[1] = "10.00000000000000001"
	b[1] = "10.00000000000000002"
	if _, err := parseKlines([]byte(fixtureBody(a, b)), "BTCUSDT", "1m", 10); !errors.Is(err, srv.ErrInvalidPayload) {
		t.Fatalf("rounded-away collision accepted: %v", err)
	}
}

func TestNoOutOfRangeOrFutureRequests(t *testing.T) {
	calls := 0
	c := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, fixtureBody(fixtureRow(fixtureStart.Add(time.Minute), "1m")))
	})
	q := srv.RangeQuery{Symbol: "BTCUSDT", Interval: "1m", Start: fixtureStart, End: fixtureStart.Add(time.Minute), Limit: 5}
	if _, err := c.FetchRange(context.Background(), q); !errors.Is(err, srv.ErrInvalidPayload) {
		t.Fatalf("boundary leak: %v", err)
	}
	q.End = fixtureStart.Add(3 * time.Minute)
	if _, err := c.FetchRange(context.Background(), q); !errors.Is(err, srv.ErrInvalidQuery) || calls != 1 {
		t.Fatalf("future request escaped: %v calls=%d", err, calls)
	}
}

func TestProfilesAndHTTPFailures(t *testing.T) {
	for _, o := range []ClientOptions{
		{BaseURL: "https://example.invalid", KlinesPath: "api/v3/klines"},
		{BaseURL: "https://example.invalid", KlinesPath: "api/v3/klines", Market: "usdm"},
		{BaseURL: "https://example.invalid/?credential=x", KlinesPath: "api/v3/klines", Market: "spot"},
	} {
		if _, err := NewBinanceClientWithOptions(o); err == nil {
			t.Error("ambiguous profile accepted")
		}
	}
	c := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(429)
		fmt.Fprint(w, "do not expose body")
	})
	_, err := c.FetchKlines(context.Background(), "BTCUSDT", "1m", 10)
	var se *srv.SourceError
	if !errors.As(err, &se) || !se.Transient || se.RetryAfter != 3*time.Second || strings.Contains(err.Error(), "expose") {
		t.Fatalf("HTTP error contract %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.FetchKlines(ctx, "BTCUSDT", "1m", 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel lost: %v", err)
	}
}

func TestResponseSizeBound(t *testing.T) {
	c := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBytes+1)))
	})
	if _, err := c.FetchKlines(context.Background(), "BTCUSDT", "1m", 1); !errors.Is(err, srv.ErrInvalidPayload) {
		t.Fatal(err)
	}
}
