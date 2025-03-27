package jobs

import (
	"context"
	"time"

	"infosir/internal/srv"
	"infosir/internal/utils"

	"go.uber.org/zap"
)

// RunScheduledRequests starts a ticker that, on every interval, fetches a small set of
// new klines from the BinanceClient for each pair, then publishes them to NATS JetStream.
// This is used to keep data regularly updated in near-real-time.
func RunScheduledRequests(ctx context.Context, service srv.InfoSirService, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	utils.Logger.Info("Scheduled job started",
		zap.Duration("interval", interval))

	for {
		select {
		case <-ticker.C:
			for _, pair := range utils.GetConfig().Crypto.Pairs {
				klines, err := service.GetKlines(ctx, pair, utils.GetConfig().Crypto.KlineInterval,
					int64(utils.GetConfig().Crypto.KlineLimit))
				if err != nil {
					utils.Logger.Error("Failed to get klines from Binance",
						zap.String("pair", pair),
						zap.Error(err))
					continue
				}

				if err := service.PublishKlinesJS(ctx, klines); err != nil {
					utils.Logger.Error("Failed to publish klines to NATS",
						zap.String("pair", pair),
						zap.Error(err))
					continue
				}

				utils.Logger.Debug("Fetched & published klines successfully",
					zap.String("pair", pair),
					zap.Int("klinesCount", len(klines)))
			}

		case <-ctx.Done():
			utils.Logger.Info("Scheduled job context done; stopping.")
			return
		}
	}
}
