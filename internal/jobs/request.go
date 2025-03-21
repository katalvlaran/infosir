package jobs

import (
	"context"
	"time"

	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/internal/srv"
)

// RunScheduledRequests запускает планировщик, который каждые 1 (или N) минут
// вызывает service.GetKlines() и service.PublishKlines() по каждому инструменту из cfg.Pairs.
// Вызывается из main.go в отдельной горутине.
func RunScheduledRequests(ctx context.Context, logger *zap.Logger, cfg *config.Config, service srv.InfoSirService) {
	ticker := time.NewTicker(1 * time.Minute) // каждые 60 секунд
	defer ticker.Stop()

	logger.Info("Scheduled job started",
		zap.Duration("interval", 1*time.Minute),
	)

	for {
		select {
		case <-ticker.C:
			// Для каждой пары в cfg.Pairs получаем свечи и публикуем их
			for _, pair := range cfg.Pairs {
				klines, err := service.GetKlines(ctx, pair, cfg.KlineInterval, int64(cfg.KlineLimit))
				if err != nil {
					logger.Error("Failed to get klines from Binance",
						zap.String("pair", pair),
						zap.Error(err),
					)
					continue
				}
				err = service.PublishKlines(ctx, klines)
				if err != nil {
					logger.Error("Failed to publish klines to NATS",
						zap.String("pair", pair),
						zap.Error(err),
					)
					continue
				}
				logger.Debug("Successfully fetched & published klines",
					zap.String("pair", pair),
					zap.Int("klinesCount", len(klines)),
				)
			}

		case <-ctx.Done():
			// Контекст завершён (graceful shutdown) — выходим
			logger.Info("Scheduled job received stop signal")
			return
		}
	}
}
