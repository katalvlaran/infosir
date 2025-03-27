package nats

import (
	"context"
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
	js     nats.JetStreamContext
	stream string
	subj   string
}

// NewNatsJetStreamClient constructs a new natsJetStreamClient using the provided js context.
func NewNatsJetStreamClient(js nats.JetStreamContext) *natsJetStreamClient {
	return &natsJetStreamClient{
		js:     js,
		stream: utils.GetConfig().NATS.StreamName,
		subj:   utils.GetConfig().NATS.Subject,
	}
}

// PublishKlines publishes the given slice of Klines as JSON to the configured subject.
func (c *natsJetStreamClient) PublishKlines(ctx context.Context, klines []models.Kline) error {
	if len(klines) == 0 {
		return nil
	}

	data, err := json.Marshal(klines)
	if err != nil {
		return fmt.Errorf("json.Marshal klines error: %w", err)
	}

	// Publish via JetStream
	ack, err := c.js.Publish(c.subj, data, nats.Context(ctx))
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
