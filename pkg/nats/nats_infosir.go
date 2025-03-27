package natsinfosir

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// NatsClient — временный или будущий интерфейс, который будет находиться в pkg/nats (Шаг 9).
// Сигнатура PublishJS(subject string, data []byte) error
type NatsClient interface {
	PublishJS(subject string, data []byte) error
}

type natsJetStreamClientImpl struct {
	js nats.JetStreamContext
}

func NewNatsJetStreamClient(js nats.JetStreamContext) NatsClient {
	return &natsJetStreamClientImpl{js: js}
}

func (n *natsJetStreamClientImpl) PublishJS(subject string, data []byte) error {
	// do up to 3 attempts
	maxAttempts := 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := n.js.Publish(subject, data)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("JetStream publish failed after 3 attempts: %w", lastErr)
}
