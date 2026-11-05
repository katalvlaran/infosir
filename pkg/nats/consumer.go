package nats

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"infosir/internal/db/repository"
	"infosir/internal/models"
	"infosir/internal/srv"
	"infosir/internal/utils"

	gonats "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// MessageStore commits inbox and bars together; quarantine must be durable.
type MessageStore interface {
	CommitMessage(context.Context, string, string, []byte, []models.Kline) error
	Quarantine(context.Context, string, string, string, []byte) error
}

// Delivery makes acknowledgement ordering testable without a broker.
type Delivery interface {
	Payload() []byte
	Header(string) string
	Ack() error
	Retry() error
	Terminate() error
}

// ConsumerProfile is the immutable identity of one legacy storage database.
type ConsumerProfile struct {
	Consumer, Market, Interval string
	Clock                      func() time.Time
	FinalityLag                time.Duration
}

// ProcessDelivery acknowledges only a known committed transaction. Permanent
// rejects are quarantined before termination; all storage errors cause retry.
func ProcessDelivery(ctx context.Context, d Delivery, store MessageStore, p ConsumerProfile) error {
	data := d.Payload()
	id := d.Header(gonats.MsgIdHdr)
	hash := sha256.Sum256(data)
	if id == "" {
		id = "missing:" + hex.EncodeToString(hash[:])
	}
	reject := func(reason string) error {
		// Large payloads retain a digest and size, not an unbounded database blob.
		raw := data
		if len(raw) > 1<<20 {
			raw = []byte(fmt.Sprintf("oversize bytes=%d sha256=%x", len(data), hash))
		}
		if err := store.Quarantine(ctx, p.Consumer, id, reason, raw); err != nil {
			return errors.Join(err, d.Retry())
		}
		return d.Terminate()
	}
	if len(data) > 1<<20 {
		return reject("payload_too_large")
	}
	if d.Header("Infosir-Schema") != "1" || d.Header("Infosir-Market") != p.Market || d.Header("Infosir-Interval") != p.Interval || d.Header(gonats.MsgIdHdr) == "" {
		return reject("invalid_envelope")
	}
	klines, err := decodeKlines(data)
	if err != nil {
		return reject("invalid_json")
	}
	if len(klines) == 0 || len(klines) > 1500 {
		return reject("invalid_batch_size")
	}
	now := time.Now()
	if p.Clock != nil {
		now = p.Clock()
	}
	seen := make(map[string]struct{}, len(klines))
	for _, k := range klines {
		if err = k.Validate(); err != nil {
			return reject("invalid_kline")
		}
		end, e := srv.NextOpen(k.Time, p.Interval)
		if e != nil || end.After(now.Add(-p.FinalityLag)) {
			return reject("unaligned_or_open_bar")
		}
		key := k.Symbol + "/" + k.Time.UTC().Format(time.RFC3339Nano)
		if _, ok := seen[key]; ok {
			return reject("duplicate_bar")
		}
		seen[key] = struct{}{}
	}
	if err = store.CommitMessage(ctx, p.Consumer, id, hash[:], klines); err != nil {
		if errors.Is(err, repository.ErrPayloadConflict) {
			return reject("payload_conflict")
		}
		return errors.Join(err, d.Retry())
	}
	return d.Ack()
}

type brokerDelivery struct{ msg *gonats.Msg }

func (d brokerDelivery) Payload() []byte        { return d.msg.Data }
func (d brokerDelivery) Header(k string) string { return d.msg.Header.Get(k) }
func (d brokerDelivery) Ack() error             { return d.msg.AckSync(gonats.AckWait(3 * time.Second)) }
func (d brokerDelivery) Retry() error           { return d.msg.NakWithDelay(time.Second) }
func (d brokerDelivery) Terminate() error       { return d.msg.Term() }

// StartJetStreamConsumer uses explicit acknowledgement and bounded in-flight work.
// The caller drains the NATS connection before cancelling ctx during shutdown.
func StartJetStreamConsumer(ctx context.Context, js gonats.JetStreamContext, repo *repository.KlineRepository) error {
	cfg := utils.GetConfig()
	profile := ConsumerProfile{Consumer: cfg.NATS.ConsumerName, Market: cfg.Crypto.Market, Interval: cfg.Crypto.KlineInterval, FinalityLag: 2 * time.Second}
	// Explicit creation plus Bind keeps the durable consumer across Drain/Unsubscribe.
	info, err := js.ConsumerInfo(cfg.NATS.StreamName, cfg.NATS.ConsumerName)
	if errors.Is(err, gonats.ErrConsumerNotFound) {
		info, err = js.AddConsumer(cfg.NATS.StreamName, &gonats.ConsumerConfig{Durable: cfg.NATS.ConsumerName, DeliverSubject: gonats.NewInbox(), FilterSubject: cfg.NATS.Subject, AckPolicy: gonats.AckExplicitPolicy, AckWait: 30 * time.Second, MaxAckPending: 64})
	}
	if err != nil {
		return err
	}
	if err = validateConsumerConfig(info.Config, cfg.NATS.Subject, cfg.NATS.ConsumerName); err != nil {
		return err
	}

	_, err = js.Subscribe(cfg.NATS.Subject, func(msg *gonats.Msg) {
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := ProcessDelivery(c, brokerDelivery{msg}, repo, profile); err != nil {
			utils.Logger.Error("message processing or acknowledgement failed", zap.Error(err))
		}
	}, gonats.Bind(cfg.NATS.StreamName, cfg.NATS.ConsumerName), gonats.ManualAck())
	return err
}
func decodeKlines(data []byte) ([]models.Kline, error) {
	var klines []models.Kline
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&klines); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return klines, nil
}

// validateConsumerConfig keeps delivery state durable and retries unbounded.
// BackOff overrides AckWait on the server, so every interval must allow the
// 20-second processing deadline plus the 3-second synchronous acknowledgement.
func validateConsumerConfig(c gonats.ConsumerConfig, subject, durable string) error {
	if c.Durable != durable || c.FilterSubject != subject || len(c.FilterSubjects) != 0 ||
		c.DeliverSubject == "" || c.AckPolicy != gonats.AckExplicitPolicy ||
		c.AckWait < 25*time.Second || c.MaxAckPending < 1 || c.MaxAckPending > 64 ||
		c.MaxDeliver > 0 || c.MaxDeliver < -1 || c.InactiveThreshold != 0 || c.MemoryStorage {
		return errors.New("consumer durability/retry profile differs; migrate explicitly")
	}
	for _, delay := range c.BackOff {
		if delay < 25*time.Second {
			return errors.New("consumer backoff is shorter than processing deadline; migrate explicitly")
		}
	}
	return nil
}
