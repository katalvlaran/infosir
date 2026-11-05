package crypto

import (
	"context"
	"infosir/internal/srv"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type receiptTransport struct{ response *http.Response }

func (r receiptTransport) RoundTrip(*http.Request) (*http.Response, error) { return r.response, nil }

type receiptBody struct {
	io.Reader
	complete func()
}

func (r *receiptBody) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	if e == io.EOF {
		r.complete()
	}
	return n, e
}
func (*receiptBody) Close() error { return nil }

func TestReceivedAtIsAfterCompleteResponseBody(t *testing.T) {
	headersAt := fixtureStart.Add(2 * time.Minute)
	bodyAt := headersAt.Add(5 * time.Second)
	clock := headersAt
	body := &receiptBody{Reader: strings.NewReader(fixtureBody(fixtureRow(fixtureStart, "1m"))), complete: func() { clock = bodyAt }}
	client, err := NewBinanceClientWithOptions(ClientOptions{BaseURL: "https://fixture.invalid", KlinesPath: "api/v3/klines", Market: "spot", Clock: func() time.Time { return clock }, HTTPClient: &http.Client{Transport: receiptTransport{&http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}}}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.FetchRange(context.Background(), srv.RangeQuery{Symbol: "BTCUSDT", Interval: "1m", Start: fixtureStart, End: fixtureStart.Add(time.Minute), Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !page.ReceivedAt.Equal(bodyAt) {
		t.Fatalf("receipt=%v; payload became available at %v", page.ReceivedAt, bodyAt)
	}
}
