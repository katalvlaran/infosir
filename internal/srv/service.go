package srv

import (
	"context"
	"encoding/json"
	"fmt"

	"infosir/internal/util"
	natsinfosir "infosir/pkg/nats"

	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/internal/model"
)

// BinanceClient — временный или будущий интерфейс, который будет находиться в pkg/crypto (Шаг 8).
// Сигнатура FetchKlines(...) должна возвращать []model.Kline, error.
type BinanceClient interface {
	FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
	FetchKlinesRange(ctx context.Context, pair, interval string, startTime, endTime int64, limit int64) ([]model.Kline, error)
}

// InfoSirService — основной интерфейс сервиса. Методы:
// 1) GetKlines: запросить свечи у Binance
// 2) PublishKlinesJS: отправить свечи в NATS JetStream
// InfoSirService — методы
type InfoSirService interface {
	GetKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
	PublishKlinesJS(ctx context.Context, klines []model.Kline) error
}

// infoSirServiceImpl — структура с зависимостями:
// - logger для логирования
// - cfg для конфигурации (NATS_SUBJECT, лимиты и т.д.)
// - binanceClient (реализуется в pkg/crypto/binance.go)
// - natsClient (new NatsClient that supports PublishJS, реализуется в pkg/nats/nats_infosir.go)
type infoSirServiceImpl struct {
	logger        *zap.Logger
	cfg           *config.Config
	binanceClient BinanceClient
	natsClient    natsinfosir.NatsClient
}

// NewInfoSirService — конструктор, принимает все необходимые зависимости.
func NewInfoSirService(
	binanceClient BinanceClient,
	natsClient natsinfosir.NatsClient,
) InfoSirService {
	return &infoSirServiceImpl{
		logger:        util.Logger,
		cfg:           &config.Cfg,
		binanceClient: binanceClient,
		natsClient:    natsClient,
	}
}

// GetKlines — делает до 3 попыток запроса свечей у Binance через binanceClient.
// При ошибках логирует и возвращает ошибку, если все 3 попытки не увенчались успехом.
func (s *infoSirServiceImpl) GetKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error) {
	var result []model.Kline
	var err error

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err = s.binanceClient.FetchKlines(ctx, pair, interval, limit)
		if err == nil {
			s.logger.Debug("Fetched klines successfully",
				zap.String("pair", pair),
				zap.String("interval", interval),
				zap.Int64("limit", limit),
				zap.Int("attempt", attempt),
			)
			break
		}
		s.logger.Warn("Failed to fetch klines",
			zap.String("pair", pair),
			zap.String("interval", interval),
			zap.Int64("limit", limit),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("could not fetch klines after %d attempts: %w", maxAttempts, err)
	}

	// (Необязательно) можно пробежаться по списку и валидировать каждую свечу:
	// for _, k := range result {
	//     if valErr := k.Validate(); valErr != nil {
	//         s.logger.Warn("Validation failed for a kline", zap.Error(valErr))
	//     }
	// }

	return result, nil
}

// In the PublishKlines step:
func (s *infoSirServiceImpl) PublishKlinesJS(ctx context.Context, klines []model.Kline) error {
	data, err := json.Marshal(klines)
	if err != nil {
		s.logger.Error("Failed to marshal klines", zap.Error(err))
		return err
	}
	subject := s.cfg.Nats.NatsSubject
	return s.natsClient.PublishJS(subject, data)
}
