package srv

import (
	"context"

	"infosir/internal/models"
)

// BinanceClient is an interface that represents the minimal set of methods
// needed from a Binance (or other exchange) client to fetch Klines data.
type BinanceClient interface {
	// FetchKlines retrieves up to 'limit' klines for the given trading pair and interval.
	FetchKlines(ctx context.Context, pair, interval string, limit int64) ([]models.Kline, error)
}

// NatsClient is an interface representing publishing capabilities to NATS (JetStream).
type NatsClient interface {
	// PublishKlines publishes the given klines to the configured subject/stream in JetStream.
	PublishKlines(ctx context.Context, klines []models.Kline) error
}

// InfoSirService defines the high-level service interface for retrieving klines and
// publishing them to JetStream or other messaging systems.
type InfoSirService interface {
	// GetKlines obtains the latest (limit) klines from the BinanceClient for the pair and interval.
	GetKlines(ctx context.Context, pair, interval string, limit int64) ([]models.Kline, error)
	// PublishKlinesJS publishes the given klines to NATS JetStream.
	PublishKlinesJS(ctx context.Context, klines []models.Kline) error
}

// infoSirServiceImpl is the internal struct implementing the InfoSirService interface.
type infoSirServiceImpl struct {
	binanceClient BinanceClient
	natsClient    NatsClient
}

// NewInfoSirService constructs an InfoSirService with the given Binance and NATS clients.
func NewInfoSirService(
	binanceClient BinanceClient,
	natsClient NatsClient,
) InfoSirService {
	return &infoSirServiceImpl{
		binanceClient: binanceClient,
		natsClient:    natsClient,
	}
}

// GetKlines obtains the latest klines from the exchange for the specified pair, interval, and limit.
func (s *infoSirServiceImpl) GetKlines(
	ctx context.Context,
	pair string,
	interval string,
	limit int64,
) ([]models.Kline, error) {
	klines, err := s.binanceClient.FetchKlines(ctx, pair, interval, limit)
	if err != nil {
		return nil, err
	}

	// Optionally validate each returned kline if needed
	// e.g. for i, k := range klines { if err := k.Validate(); err != nil { ... } }

	return klines, nil
}

// PublishKlinesJS publishes klines to NATS JetStream via the underlying natsClient.
func (s *infoSirServiceImpl) PublishKlinesJS(
	ctx context.Context,
	klines []models.Kline,
) error {
	return s.natsClient.PublishKlines(ctx, klines)
}
