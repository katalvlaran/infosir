package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"infosir/internal/models"
	"infosir/internal/srv"
)

type rangeFunc func(context.Context, srv.RangeQuery) (srv.RangePage, error)

func (f rangeFunc) FetchRange(c context.Context, q srv.RangeQuery) (srv.RangePage, error) {
	return f(c, q)
}

type writerFunc func(context.Context, []models.Kline) error

func (f writerFunc) BatchInsertKlines(c context.Context, k []models.Kline) error { return f(c, k) }

var start = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func page(at time.Time, interval string) srv.RangePage {
	next, _ := srv.NextOpen(at, interval)
	return srv.RangePage{Klines: []models.Kline{{Time: at, Symbol: "BTCUSDT", OpenPrice: 10, HighPrice: 12, LowPrice: 9, ClosePrice: 11}}, CloseExclusive: []time.Time{next}, NextOpen: next}
}
func plan(interval string, end time.Time) BackfillPlan {
	return BackfillPlan{Query: srv.RangeQuery{Symbol: "BTCUSDT", Interval: interval, Start: start, End: end, Limit: 10}, RetryDelay: time.Millisecond}
}

func TestBackfillUsesRequestedCursorAndInterval(t *testing.T) {
	queries := []srv.RangeQuery{}
	source := rangeFunc(func(_ context.Context, q srv.RangeQuery) (srv.RangePage, error) {
		queries = append(queries, q)
		return page(q.Start, q.Interval), nil
	})
	writes := 0
	store := writerFunc(func(context.Context, []models.Kline) error { writes++; return nil })
	end := start.Add(8 * time.Hour)
	r, err := RunBackfill(context.Background(), plan("4h", end), source, store)
	if err != nil || writes != 2 || len(queries) != 2 || !queries[1].Start.Equal(start.Add(4*time.Hour)) || !r.NextOpen.Equal(end) {
		t.Fatalf("result %+v err=%v writes=%d queries=%+v", r, err, writes, queries)
	}
}

func TestWriteFailureCannotAdvance(t *testing.T) {
	failure := errors.New("storage unavailable")
	requests, writes := 0, 0
	source := rangeFunc(func(_ context.Context, q srv.RangeQuery) (srv.RangePage, error) {
		requests++
		return page(q.Start, q.Interval), nil
	})
	store := writerFunc(func(context.Context, []models.Kline) error {
		writes++
		if writes == 2 {
			return failure
		}
		return nil
	})
	r, err := RunBackfill(context.Background(), plan("1m", start.Add(3*time.Minute)), source, store)
	if !errors.Is(err, failure) || r.Pages != 1 || !r.NextOpen.Equal(start.Add(time.Minute)) || requests != 2 || writes != 2 {
		t.Fatalf("write failure skipped: %+v %v requests=%d", r, err, requests)
	}
}

func TestGapAndEmptyPagesAreNotCompletion(t *testing.T) {
	cases := []struct {
		name string
		p    srv.RangePage
		want error
	}{
		{"empty", srv.RangePage{}, srv.ErrGap},
		{"missingFirst", page(start.Add(time.Minute), "1m"), srv.ErrGap},
		{"stall", srv.RangePage{Klines: page(start, "1m").Klines, CloseExclusive: []time.Time{start.Add(time.Minute)}, NextOpen: start}, srv.ErrPaginationStalled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			r, err := RunBackfill(context.Background(), plan("1m", start.Add(3*time.Minute)), rangeFunc(func(context.Context, srv.RangeQuery) (srv.RangePage, error) { return tc.p, nil }), writerFunc(func(context.Context, []models.Kline) error { writes++; return nil }))
			if !errors.Is(err, tc.want) || writes != 0 || !r.NextOpen.Equal(start) {
				t.Fatalf("result %+v err=%v writes=%d", r, err, writes)
			}
		})
	}
}

func TestTransientRetriesDoNotMoveCursor(t *testing.T) {
	calls := 0
	p := plan("1m", start.Add(time.Minute))
	p.MaxAttempts = 3
	source := rangeFunc(func(_ context.Context, q srv.RangeQuery) (srv.RangePage, error) {
		calls++
		if !q.Start.Equal(start) {
			t.Fatal("retry cursor advanced")
		}
		if calls < 3 {
			return srv.RangePage{}, &srv.SourceError{StatusCode: 503, Transient: true}
		}
		return page(start, "1m"), nil
	})
	r, err := RunBackfill(context.Background(), p, source, writerFunc(func(context.Context, []models.Kline) error { return nil }))
	if err != nil || calls != 3 || r.Pages != 1 {
		t.Fatalf("%+v %v calls=%d", r, err, calls)
	}
	calls = 0
	_, err = RunBackfill(context.Background(), p, rangeFunc(func(context.Context, srv.RangeQuery) (srv.RangePage, error) {
		calls++
		return srv.RangePage{}, srv.ErrInvalidPayload
	}), writerFunc(func(context.Context, []models.Kline) error { t.Fatal("write on invalid response"); return nil }))
	if !errors.Is(err, srv.ErrInvalidPayload) || calls != 1 {
		t.Fatalf("permanent error retried: %v calls=%d", err, calls)
	}
}

func TestRetryAfterIsCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempt := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := RunBackfill(ctx, plan("1m", start.Add(time.Minute)), rangeFunc(func(context.Context, srv.RangeQuery) (srv.RangePage, error) {
			close(attempt)
			return srv.RangePage{}, &srv.SourceError{StatusCode: 429, Transient: true, RetryAfter: 30 * time.Second}
		}), writerFunc(func(context.Context, []models.Kline) error { return nil }))
		done <- err
	}()
	<-attempt
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry ignored cancellation")
	}
}

func TestLongRetryAfterReturnsDeferral(t *testing.T) {
	calls := 0
	refusal := &srv.SourceError{StatusCode: 429, RetryAfter: 24 * time.Hour, Transient: true}
	source := rangeFunc(func(context.Context, srv.RangeQuery) (srv.RangePage, error) { calls++; return srv.RangePage{}, refusal })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := RunBackfill(ctx, plan("1m", start.Add(time.Minute)), source, writerFunc(func(context.Context, []models.Kline) error { t.Fatal("unexpected write"); return nil }))
	if !errors.Is(err, refusal) || calls != 1 {
		t.Fatalf("long deferral did not stop cleanly: %d %v", calls, err)
	}
}
