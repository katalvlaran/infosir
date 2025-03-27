package mocks

import (
	"context"

	"infosir/internal/model"
)

type MockInfoSirService struct {
	GetKlinesFn     func(context.Context, string, string, int64) ([]model.Kline, error)
	PublishKlinesFn func(context.Context, []model.Kline) error

	CallsGetKlines []struct {
		Pair, Interval string
		Limit          int64
	}
	CallsPublishKlines []int // or store the actual klines length
}

func (m *MockInfoSirService) GetKlines(ctx context.Context, pair, interval string, limit int64) ([]model.Kline, error) {
	m.CallsGetKlines = append(m.CallsGetKlines, struct {
		Pair, Interval string
		Limit          int64
	}{pair, interval, limit})
	if m.GetKlinesFn != nil {
		return m.GetKlinesFn(ctx, pair, interval, limit)
	}
	return nil, nil
}

func (m *MockInfoSirService) PublishKlinesJS(ctx context.Context, kl []model.Kline) error {
	m.CallsPublishKlines = append(m.CallsPublishKlines, len(kl))
	if m.PublishKlinesFn != nil {
		return m.PublishKlinesFn(ctx, kl)
	}
	return nil
}
