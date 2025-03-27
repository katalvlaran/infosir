package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"infosir/internal/db/repository"
	"infosir/internal/models"
	"infosir/internal/utils"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// StartJetStreamConsumer sets up a durable consumer on the configured stream/subject
// and processes messages by deserializing klines, then storing them in DB.
func StartJetStreamConsumer(
	ctx context.Context,
	js nats.JetStreamContext,
	klineRepo *repository.KlineRepository,
) error {
	subject := utils.GetConfig().NATS.Subject
	durableName := utils.GetConfig().NATS.ConsumerName

	// We create a pull subscription or push subscription? Let's suppose push:
	sub, err := js.Subscribe(subject, func(msg *nats.Msg) {
		err := handleKlinesMsg(ctx, msg, klineRepo)
		if err != nil {
			utils.Logger.Error("handleKlinesMsg error", zap.Error(err))
			// we can either NAK or let the message fail. For now let's Ack anyway.
		}
		_ = msg.Ack() // ack to JetStream
	}, nats.Durable(durableName), nats.ManualAck())
	if err != nil {
		return fmt.Errorf("js.Subscribe error: %w", err)
	}

	utils.Logger.Info("JetStream consumer started",
		zap.String("subject", subject),
		zap.String("durableName", durableName),
		zap.String("queue", ""), // if we used a queue group, specify
	)

	// Wait in a goroutine for context cancellation, unsub when done
	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
		utils.Logger.Info("Unsubscribed from JetStream consumer", zap.String("subject", subject))
	}()

	return nil
}

// handleKlinesMsg handles a single NATS message, deserializing klines and writing them to DB.
func handleKlinesMsg(
	ctx context.Context,
	msg *nats.Msg,
	klineRepo *repository.KlineRepository,
) error {
	klines, err := decodeKlines(msg.Data)
	if err != nil {
		return fmt.Errorf("decodeKlines: %w", err)
	}

	if len(klines) == 0 {
		utils.Logger.Debug("Received empty klines array, ignoring.")
		return nil
	}

	if err := klineRepo.BatchInsertKlines(ctx, klines); err != nil {
		return fmt.Errorf("BatchInsertKlines: %w", err)
	}

	utils.Logger.Debug("Successfully inserted klines from message",
		zap.Int("count", len(klines)))

	return nil
}

// decodeKlines attempts to unmarshal JSON data into a slice of model.Kline.
func decodeKlines(data []byte) ([]models.Kline, error) {
	var klines []models.Kline
	err := json.Unmarshal(data, &klines)
	return klines, err
}
