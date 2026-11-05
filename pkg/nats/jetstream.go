package nats

import (
	"errors"
	"fmt"
	"time"

	"infosir/internal/utils"

	"github.com/nats-io/nats.go"
)

// InitNATSJetStream creates only missing streams and rejects incompatible storage.
func InitNATSJetStream() (*nats.Conn, nats.JetStreamContext, error) {
	c := utils.GetConfig().NATS
	nc, err := nats.Connect(c.URL, nats.Timeout(5*time.Second), nats.DrainTimeout(15*time.Second), nats.Name("infosir"))
	if err != nil {
		return nil, nil, fmt.Errorf("NATS connection failed")
	}
	ok := false
	defer func() {
		if !ok {
			nc.Close()
		}
	}()
	js, err := nc.JetStream(nats.MaxWait(5 * time.Second))
	if err != nil {
		return nil, nil, err
	}
	info, err := js.StreamInfo(c.StreamName)
	if errors.Is(err, nats.ErrStreamNotFound) {
		info, err = js.AddStream(&nats.StreamConfig{Name: c.StreamName, Subjects: []string{c.Subject}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, Discard: nats.DiscardNew, MaxBytes: 1 << 30, MaxMsgSize: 1 << 20})
	}
	if err != nil {
		return nil, nil, fmt.Errorf("stream setup: %w", err)
	}
	if err = validateStreamConfig(info.Config, c.Subject); err != nil {
		return nil, nil, err
	}
	ok = true
	return nc, js, nil
}

// validateStreamConfig accepts only retention that applies backpressure instead
// of evicting unacknowledged messages. Server defaults use -1 for unlimited;
// zero is also accepted as the unspecified/unlimited configuration value.
func validateStreamConfig(c nats.StreamConfig, subject string) error {
	if c.Storage != nats.FileStorage || len(c.Subjects) != 1 || c.Subjects[0] != subject ||
		c.Retention != nats.LimitsPolicy || c.Discard != nats.DiscardNew || c.MaxAge != 0 ||
		c.MaxMsgsPerSubject > 0 || c.MaxMsgsPerSubject < -1 {
		return errors.New("existing stream retention/profile differs; migrate explicitly")
	}
	return nil
}
