package srv

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/internal/model"
)

// BinanceClient — временный или будущий интерфейс, который будет находиться в pkg/crypto (Шаг 8).
// Сигнатура FetchKlines(...) должна возвращать []model.Kline, error.
type BinanceClient interface {
	FetchKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
}

// NatsClient — временный или будущий интерфейс, который будет находиться в pkg/nats (Шаг 9).
// Сигнатура Publish(subject string, data []byte) error
type NatsClient interface {
	Publish(subject string, data []byte) error
}

// InfoSirService — основной интерфейс сервиса. Методы:
// 1) GetKlines: запросить свечи у Binance
// 2) PublishKlines: отправить свечи в очередь NATS
type InfoSirService interface {
	GetKlines(ctx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
	PublishKlines(ctx context.Context, klines []model.Kline) error
}

// infoSirServiceImpl — структура с зависимостями:
// - logger для логирования
// - cfg для конфигурации (NATS_SUBJECT, лимиты и т.д.)
// - binanceClient (реализуется в pkg/crypto/binance.go)
// - natsClient (реализуется в pkg/nats/nats_infosir.go)
type infoSirServiceImpl struct {
	logger        *zap.Logger
	cfg           *config.Config
	binanceClient BinanceClient
	natsClient    NatsClient
}

// NewInfoSirService — конструктор, принимает все необходимые зависимости.
func NewInfoSirService(
	logger *zap.Logger,
	cfg *config.Config,
	binanceClient BinanceClient,
	natsClient NatsClient,
) InfoSirService {
	return &infoSirServiceImpl{
		logger:        logger,
		cfg:           cfg,
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

// PublishKlines — сериализует свечи в JSON и отправляет в NATS.
// Для этого использует natsClient.Publish().
func (s *infoSirServiceImpl) PublishKlines(ctx context.Context, klines []model.Kline) error {
	data, err := json.Marshal(klines)
	if err != nil {
		s.logger.Error("Failed to marshal klines to JSON", zap.Error(err))
		return fmt.Errorf("could not marshal klines: %w", err)
	}

	subject := s.cfg.NatsSubject
	if subject == "" {
		subject = "infosir_kline"
	}

	err = s.natsClient.Publish(subject, data)
	if err != nil {
		s.logger.Error("Failed to publish klines to NATS",
			zap.String("subject", subject),
			zap.Error(err),
		)
		return fmt.Errorf("could not publish klines: %w", err)
	}

	s.logger.Debug("Published klines to NATS",
		zap.String("subject", subject),
		zap.Int("count", len(klines)),
	)
	return nil
}
