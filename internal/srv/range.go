package srv

import (
	"context"
	"errors"
	"fmt"
	"time"

	"infosir/internal/models"
)

var (
	ErrInvalidQuery      = errors.New("invalid market-data query")
	ErrInvalidPayload    = errors.New("invalid market-data payload")
	ErrPaginationStalled = errors.New("market-data pagination stalled")
	ErrGap               = errors.New("market-data gap")
)

// RangeQuery selects bars by open time in the UTC half-open interval [Start, End).
// Start and End must lie on the requested interval's UTC calendar boundaries.
type RangeQuery struct {
	Symbol   string
	Interval string
	Start    time.Time
	End      time.Time
	Limit    int
}

// RangePage is one validated page. NextOpen is a pagination cursor, not a proof
// of completeness. REST close times do not establish observed finality.
type RangePage struct {
	Klines         []models.Kline
	CloseExclusive []time.Time
	NextOpen       time.Time
	ReceivedAt     time.Time
}

// RangeSource is independent of BinanceClient, preserving existing mocks.
type RangeSource interface {
	FetchRange(context.Context, RangeQuery) (RangePage, error)
}

// SourceError exposes safe response metadata without logging response bodies.
type SourceError struct {
	StatusCode int
	RetryAfter time.Duration
	Transient  bool
}

func (e *SourceError) Error() string { return fmt.Sprintf("market-data HTTP status %d", e.StatusCode) }

// IntervalStart returns the UTC open boundary containing t. Supported intervals
// match the implemented UTC Binance profiles; month and week are calendar based.
func IntervalStart(t time.Time, interval string) (time.Time, error) {
	t = t.UTC()
	if t.Year() < 1970 || t.Year() > 9999 {
		return time.Time{}, ErrInvalidQuery
	}
	if interval == "1M" {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	if interval == "1w" {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		return d.AddDate(0, 0, -(int(d.Weekday())+6)%7), nil
	}
	d, ok := intervals[interval]
	if !ok {
		return time.Time{}, fmt.Errorf("%w: unsupported interval %q", ErrInvalidQuery, interval)
	}
	// Supported fixed intervals divide a UTC day; 3d is deliberately unsupported.
	seconds := t.Unix()
	step := int64(d / time.Second)
	return time.Unix(seconds-seconds%step, 0).UTC(), nil
}

var intervals = map[string]time.Duration{
	"1s": time.Second, "1m": time.Minute, "3m": 3 * time.Minute,
	"5m": 5 * time.Minute, "15m": 15 * time.Minute, "30m": 30 * time.Minute,
	"1h": time.Hour, "2h": 2 * time.Hour, "4h": 4 * time.Hour,
	"6h": 6 * time.Hour, "8h": 8 * time.Hour, "12h": 12 * time.Hour,
	"1d": 24 * time.Hour,
}

// NextOpen advances exactly one interval from an aligned UTC open time.
func NextOpen(t time.Time, interval string) (time.Time, error) {
	start, err := IntervalStart(t, interval)
	if err != nil || !start.Equal(t) {
		return time.Time{}, fmt.Errorf("%w: unaligned open time", ErrInvalidQuery)
	}
	var next time.Time
	switch interval {
	case "1M":
		next = start.AddDate(0, 1, 0)
	case "1w":
		next = start.AddDate(0, 0, 7)
	default:
		next = start.Add(intervals[interval])
	}
	if next.Year() > 9999 || !next.After(start) {
		return time.Time{}, ErrInvalidQuery
	}
	return next, nil
}

// ValidateRange rejects ambiguous sub-millisecond or unaligned boundaries.
func ValidateRange(q RangeQuery) error {
	if q.Limit < 1 || !q.End.After(q.Start) {
		return ErrInvalidQuery
	}
	for _, t := range []time.Time{q.Start, q.End} {
		start, err := IntervalStart(t, q.Interval)
		if err != nil || !start.Equal(t) {
			return fmt.Errorf("%w: range boundary", ErrInvalidQuery)
		}
	}
	return nil
}
