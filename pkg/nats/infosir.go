package nats

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"infosir/internal/models"
	"infosir/internal/utils"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// natsJetStreamClient implements the NatsClient interface from the service layer
// by using a JetStreamContext under the hood.
type natsJetStreamClient struct {
	js               nats.JetStreamContext
	stream           string
	subj             string
	market, interval string
}

// NewNatsJetStreamClient constructs a new natsJetStreamClient using the provided js context.
func NewNatsJetStreamClient(js nats.JetStreamContext) *natsJetStreamClient {
	return &natsJetStreamClient{
		js:     js,
		market: utils.GetConfig().Crypto.Market, interval: utils.GetConfig().Crypto.KlineInterval,
		stream: utils.GetConfig().NATS.StreamName,
		subj:   utils.GetConfig().NATS.Subject,
	}
}

// PublishKlines publishes the given slice of Klines as JSON to the configured subject.
func (c *natsJetStreamClient) PublishKlines(ctx context.Context, klines []models.Kline) error {
	if len(klines) == 0 {
		return nil
	}

	if len(klines) > 1500 {
		return fmt.Errorf("too many klines")
	}
	for i, k := range klines {
		if err := k.Validate(); err != nil {
			return fmt.Errorf("kline %d: %w", i, err)
		}
	}
	data, err := json.Marshal(klines)
	if err != nil {
		return fmt.Errorf("json.Marshal klines error: %w", err)
	}

	// Profile participates in identity; retries reuse the same stable message ID.
	hash := sha256.Sum256(append([]byte(c.market+"\x00"+c.interval+"\x00"), data...))
	msg := nats.NewMsg(c.subj)
	msg.Data = data
	msg.Header.Set(nats.MsgIdHdr, hex.EncodeToString(hash[:]))
	msg.Header.Set("Infosir-Schema", "1")
	msg.Header.Set("Infosir-Market", c.market)
	msg.Header.Set("Infosir-Interval", c.interval)
	ack, err := c.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("js.Publish error: %w", err)
	}

	utils.Logger.Debug("Published klines to JetStream",
		zap.String("subject", c.subj),
		zap.Int("count", len(klines)),
		zap.String("stream", c.stream),
		zap.String("ackStream", ack.Stream))
	return nil
}
