package jobs

import (
	"context"
	"time"

	"infosir/internal/util"

	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/internal/srv"
)

// RunScheduledRequests запускает планировщик, который каждые 1 (или N) минут
// вызывает service.GetKlines() и service.PublishKlines() по каждому инструменту из cfg.Pairs.
// Вызывается из main.go в отдельной горутине.
func RunScheduledRequests(ctx context.Context, service srv.InfoSirService, interval time.Duration) {
	ticker := time.NewTicker(interval) // каждые 60 секунд
	defer ticker.Stop()

	util.Logger.Info("Scheduled job started",
		zap.Duration("interval", 1*time.Minute),
	)

	for {
		select {
		case <-ticker.C:
			// Для каждой пары в cfg.Pairs получаем свечи и публикуем их
			for _, pair := range config.Cfg.Crypto.Pairs {
				klines, err := service.GetKlines(ctx, pair, config.Cfg.Crypto.KlineInterval, int64(config.Cfg.Crypto.KlineLimit))
				if err != nil {
					util.Logger.Error("Failed to get klines from Binance",
						zap.String("pair", pair),
						zap.Error(err),
					)
					continue
				}
				err = service.PublishKlinesJS(ctx, klines)
				if err != nil {
					util.Logger.Error("Failed to publish klines to NATS",
						zap.String("pair", pair),
						zap.Error(err),
					)
					continue
				}
				util.Logger.Debug("Successfully fetched & published klines",
					zap.String("pair", pair),
					zap.Int("klinesCount", len(klines)),
				)
			}

		case <-ctx.Done():
			// Контекст завершён (graceful shutdown) — выходим
			util.Logger.Info("Scheduled job received stop signal")
			return
		}
	}
}
