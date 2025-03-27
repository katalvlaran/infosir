package natsinfosir

import (
	"context"
	"encoding/json"
	"fmt"

	"infosir/cmd/config"
	"infosir/internal/db/repository"
	"infosir/internal/model"
	"infosir/internal/util"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func StartJetStreamConsumer(
	ctx context.Context,
	js nats.JetStreamContext,
	repo *repository.KlineRepository,
) error {

	streamName := config.Cfg.Nats.JetStreamStreamName
	consumerName := config.Cfg.Nats.JetStreamConsumer
	subject := config.Cfg.Nats.NatsSubject

	// ensure the consumer is created
	// or do js.AddConsumer(...) if needed
	// but we can just do a subscribe with Durable + Bind
	sub, err := js.Subscribe(subject, func(m *nats.Msg) {
		// parse
		var klines []model.Kline
		if err := json.Unmarshal(m.Data, &klines); err != nil {
			util.Logger.Error("Failed to unmarshal klines", zap.Error(err))
			m.Nak() // or don't ack so it can be retried
			return
		}
		// we do not necessarily know which pair? Possibly store the pair in the JSON.
		// or we rely on klines themselves having that info.
		// for demonstration, let's guess we only publish one pair at a time.
		// or we can do a single table if that was the design.
		// We'll assume each klines set belongs to a single pair (like "BTCUSDT"?).
		// If you need that info, you can embed it in the JSON or do a separate approach.

		// for example if we store pair name in each kline? That means we add "Pair" to the struct or something
		// if we don't have it, maybe we skip.
		if len(klines) == 0 {
			util.Logger.Warn("Received empty klines array, acking anyway")
			m.Ack()
			return
		}

		// insert
		err := repo.BatchInsertKlines(context.Background(), klines)
		if err != nil {
			util.Logger.Error("Error inserting klines from JetStream", zap.Error(err))
			// no ack => we can see if the message is re-delivered.
			// But be mindful of repeated inserts
			m.Nak()
			return
		}
		m.Ack()
		util.Logger.Debug("JetStream consumer inserted klines", zap.Int("count", len(klines)))
	},
		nats.Durable(consumerName),
		nats.ManualAck(),
		nats.BindStream(streamName),
		// possibly nats.DeliverNew() or DeliverAll()
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe with JS: %w", err)
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe() // or sub.Drain()
	}()
	return nil
}
