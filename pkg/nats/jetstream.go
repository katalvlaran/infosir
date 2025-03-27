package nats

import (
	"fmt"

	"infosir/internal/utils"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// InitNATSJetStream connects to the NATS server, ensures the stream is created, and returns
// the raw NATS conn plus a JetStream context.
func InitNATSJetStream() (*nats.Conn, nats.JetStreamContext, error) {
	url := utils.GetConfig().NATS.URL
	streamName := utils.GetConfig().NATS.StreamName
	subject := utils.GetConfig().NATS.Subject

	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, fmt.Errorf("nats.Connect error: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, nil, fmt.Errorf("nc.JetStream error: %w", err)
	}

	// ensure the stream is present
	if _, err := js.StreamInfo(streamName); err != nil {
		// maybe it doesn't exist yet, attempt to create
		_, errCreate := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
		})
		if errCreate != nil {
			return nil, nil, fmt.Errorf("AddStream error: %w", errCreate)
		}
		utils.Logger.Info("Created new JetStream stream", zap.String("stream", streamName))
	}

	return nc, js, nil
}
