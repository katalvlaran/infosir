package nats

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"infosir/internal/db/repository"
	"infosir/internal/models"

	gonats "github.com/nats-io/nats.go"
)

type fakeDelivery struct {
	data    []byte
	headers map[string]string
	calls   *[]string
}

func (d fakeDelivery) Payload() []byte        { return d.data }
func (d fakeDelivery) Header(k string) string { return d.headers[k] }
func (d fakeDelivery) Ack() error             { *d.calls = append(*d.calls, "ack"); return nil }
func (d fakeDelivery) Retry() error           { *d.calls = append(*d.calls, "retry"); return nil }
func (d fakeDelivery) Terminate() error       { *d.calls = append(*d.calls, "term"); return nil }

type fakeStore struct {
	calls                    *[]string
	commitErr, quarantineErr error
}

func (s fakeStore) CommitMessage(context.Context, string, string, []byte, []models.Kline) error {
	*s.calls = append(*s.calls, "commit")
	return s.commitErr
}
func (s fakeStore) Quarantine(context.Context, string, string, string, []byte) error {
	*s.calls = append(*s.calls, "quarantine")
	return s.quarantineErr
}
func TestAcknowledgementOrder(t *testing.T) {
	now := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	bar := models.Kline{Time: now.Add(-time.Hour), Symbol: "BTCUSDT", OpenPrice: 1, HighPrice: 1, LowPrice: 1, ClosePrice: 1}
	data, _ := json.Marshal([]models.Kline{bar})
	for _, tc := range []struct {
		name            string
		data            []byte
		commitErr, qErr error
		want            []string
	}{
		{"committed", data, nil, nil, []string{"commit", "ack"}},
		{"db_failure", data, errors.New("db unavailable"), nil, []string{"commit", "retry"}},
		{"conflict", data, repository.ErrPayloadConflict, nil, []string{"commit", "quarantine", "term"}},
		{"decode", []byte("bad"), nil, nil, []string{"quarantine", "term"}},
		{"quarantine_failure", []byte("bad"), nil, errors.New("db unavailable"), []string{"quarantine", "retry"}},
		{"trailing_json", append(append([]byte(nil), data...), []byte("{}")...), nil, nil, []string{"quarantine", "term"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			d := fakeDelivery{tc.data, map[string]string{gonats.MsgIdHdr: "id", "Infosir-Schema": "1", "Infosir-Market": "spot", "Infosir-Interval": "1m"}, &calls}
			_ = ProcessDelivery(context.Background(), d, fakeStore{&calls, tc.commitErr, tc.qErr}, ConsumerProfile{Consumer: "test", Market: "spot", Interval: "1m", Clock: func() time.Time { return now }})
			if !reflect.DeepEqual(calls, tc.want) {
				t.Fatalf("%v want %v", calls, tc.want)
			}
		})
	}
}

func TestConsumerRejectsOpenBarAndProfile(t *testing.T) {
	now := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, market string
		at           time.Time
	}{
		{"open", "spot", now}, {"wrong_market", "usdm", now.Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bar := models.Kline{Time: tc.at, Symbol: "BTCUSDT", OpenPrice: 1, HighPrice: 1, LowPrice: 1, ClosePrice: 1}
			data, _ := json.Marshal([]models.Kline{bar})
			calls := []string{}
			d := fakeDelivery{data, map[string]string{gonats.MsgIdHdr: "id", "Infosir-Schema": "1", "Infosir-Market": tc.market, "Infosir-Interval": "1m"}, &calls}
			if err := ProcessDelivery(context.Background(), d, fakeStore{calls: &calls}, ConsumerProfile{Consumer: "test", Market: "spot", Interval: "1m", Clock: func() time.Time { return now }}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, []string{"quarantine", "term"}) {
				t.Fatalf("unexpected persistence %v", calls)
			}
		})
	}
}
