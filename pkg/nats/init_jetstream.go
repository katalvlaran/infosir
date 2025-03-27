package natsinfosir

import (
	"fmt"

	"infosir/cmd/config"
	"infosir/internal/util"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// We'll return (nc *nats.Conn, js nats.JetStreamContext, err error)
func InitNATSJetStream() (*nats.Conn, nats.JetStreamContext, error) {
	// connect plain NATS
	nc, err := InitNATS() // your existing function that returns *nats.Conn
	if err != nil {
		return nil, nil, err
	}

	// create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		util.Logger.Error("failed to get JetStream context", zap.Error(err))
		return nc, nil, fmt.Errorf("could not get JetStream context: %w", err)
	}

	streamName := config.Cfg.Nats.JetStreamStreamName
	subjectName := config.Cfg.Nats.NatsSubject

	// Add or update the stream
	info, err := js.StreamInfo(streamName)
	if err != nil {
		util.Logger.Info("StreamInfo returned an error, presumably stream doesn't exist", zap.Error(err))
		// so we attempt to create
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{subjectName},
			Retention: nats.LimitsPolicy, // indefinite retention
		})
		if err != nil {
			util.Logger.Error("Failed to create JetStream stream", zap.Error(err))
			return nc, nil, fmt.Errorf("failed to create stream: %w", err)
		}
		util.Logger.Info("JetStream stream created", zap.String("stream", streamName))
	} else {
		util.Logger.Info("Stream already exists", zap.String("stream", info.Config.Name))
	}

	return nc, js, nil
}

func InitNATS() (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("InfoSirClient"),
	}

	nc, err := nats.Connect(config.Cfg.Nats.NatsURL, opts...)
	if err != nil {
		util.Logger.Error("failed to connect to NATS", zap.Error(err))

		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return nc, nil
}
