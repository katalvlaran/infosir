package jobs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"infosir/internal/db/repository"
	"infosir/internal/models"
	"infosir/internal/srv"
	"infosir/internal/utils"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// KlineWriter is the persistence boundary for the minimal legacy backfill.
// An error must mean the caller cannot advance; unknown commit outcomes are
// replayed through idempotent writes. It does not provide durable checkpoints.
type KlineWriter interface {
	BatchInsertKlines(context.Context, []models.Kline) error
}

// BackfillPlan fixes a bounded range at job start. MaxAttempts applies to each
// page; MaxPages bounds the whole job. Empty pages and gaps stop the job.
type BackfillPlan struct {
	Query              srv.RangeQuery
	MaxAttempts        int
	MaxPages           int
	RetryDelay         time.Duration
	MinRequestInterval time.Duration
}

// BackfillResult describes committed progress within this invocation only.
// NextOpen is never persisted by this function or inferred from MAX(time).
type BackfillResult struct {
	NextOpen time.Time
	Pages    int
	Bars     int
}

// RunBackfill performs range pagination and stops before advancing on any write
// failure or unexplained gap. Callers must explicitly choose the initial range.
func RunBackfill(ctx context.Context, plan BackfillPlan, source srv.RangeSource, store KlineWriter) (BackfillResult, error) {
	result := BackfillResult{NextOpen: plan.Query.Start}
	if source == nil || store == nil {
		return result, srv.ErrInvalidQuery
	}
	if err := srv.ValidateRange(plan.Query); err != nil {
		return result, err
	}
	if plan.Query.Symbol == "" {
		return result, srv.ErrInvalidQuery
	}
	plan.Query.Symbol = strings.ToUpper(strings.TrimSpace(plan.Query.Symbol))
	if plan.MaxAttempts == 0 {
		plan.MaxAttempts = 3
	}
	if plan.MaxPages == 0 {
		plan.MaxPages = 10000
	}
	if plan.RetryDelay == 0 {
		plan.RetryDelay = 250 * time.Millisecond
	}
	if plan.MaxAttempts < 1 || plan.MaxAttempts > 16 || plan.MaxPages < 1 || plan.RetryDelay < 0 || plan.RetryDelay > time.Minute || plan.MinRequestInterval < 0 {
		return result, srv.ErrInvalidQuery
	}
	for result.NextOpen.Before(plan.Query.End) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if result.Pages >= plan.MaxPages {
			return result, fmt.Errorf("backfill page budget exceeded")
		}
		q := plan.Query
		q.Start = result.NextOpen
		page, err := fetchPage(ctx, source, q, plan)
		if err != nil {
			return result, err
		}
		if len(page.Klines) == 0 {
			return result, fmt.Errorf("%w: empty page at %s", srv.ErrGap, q.Start.Format(time.RFC3339))
		}
		if len(page.Klines) > q.Limit || len(page.CloseExclusive) != len(page.Klines) {
			return result, srv.ErrInvalidPayload
		}
		if !page.NextOpen.After(q.Start) {
			return result, srv.ErrPaginationStalled
		}
		expected := q.Start
		for i, k := range page.Klines {
			if k.Symbol != q.Symbol || !k.Time.Equal(expected) || !k.Time.Before(q.End) {
				return result, fmt.Errorf("%w: noncontiguous page", srv.ErrGap)
			}
			if err := k.Validate(); err != nil {
				return result, fmt.Errorf("%w: %v", srv.ErrInvalidPayload, err)
			}
			next, err := srv.NextOpen(k.Time, q.Interval)
			if err != nil || !next.Equal(page.CloseExclusive[i]) {
				return result, srv.ErrInvalidPayload
			}
			expected = next
		}
		if !expected.After(q.Start) || !expected.Equal(page.NextOpen) {
			return result, srv.ErrPaginationStalled
		}
		if err := store.BatchInsertKlines(ctx, page.Klines); err != nil {
			return result, fmt.Errorf("persist backfill page: %w", err)
		}
		result.NextOpen = expected
		result.Pages++
		result.Bars += len(page.Klines)
		if result.NextOpen.Before(q.End) {
			if err := waitContext(ctx, plan.MinRequestInterval); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func fetchPage(ctx context.Context, source srv.RangeSource, q srv.RangeQuery, plan BackfillPlan) (srv.RangePage, error) {
	var lastErr error
	for attempt := 0; attempt < plan.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return srv.RangePage{}, err
		}
		page, err := source.FetchRange(ctx, q)
		if err == nil {
			return page, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return srv.RangePage{}, ctx.Err()
		}
		var se *srv.SourceError
		var ne net.Error
		transient := (errors.As(err, &se) && se.Transient) || (errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()))
		if !transient || attempt+1 == plan.MaxAttempts {
			break
		}
		delay := plan.RetryDelay * time.Duration(1<<attempt)
		if delay > time.Minute {
			delay = time.Minute
		}
		if se != nil && se.RetryAfter > time.Minute {
			// Do not violate a long server deferral or park the job indefinitely.
			return srv.RangePage{}, se
		}
		if se != nil && se.RetryAfter > delay {
			delay = se.RetryAfter
		}
		if err := waitContext(ctx, delay); err != nil {
			return srv.RangePage{}, err
		}
	}
	return srv.RangePage{}, lastErr
}

func waitContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RunHistoricalSync preserves the legacy entry point. New lifecycle code can
// use RunHistoricalSyncChecked to observe failure rather than relying on logs.
func RunHistoricalSync(ctx context.Context, repo *repository.KlineRepository, client srv.BinanceClient) {
	if err := RunHistoricalSyncChecked(ctx, repo, client); err != nil && utils.Logger != nil {
		utils.Logger.Error("Historical sync incomplete", zap.Error(err))
	}
}

// RunHistoricalSyncChecked explicitly distinguishes a missing row from DB
// failure. SYNC_START takes precedence and is required for an empty database.
// Without it, the latest stored row is replayed as a recovery overlap only;
// this cannot prove or repair gaps earlier in the legacy table.
func RunHistoricalSyncChecked(ctx context.Context, repo *repository.KlineRepository, client srv.BinanceClient) error {
	cfg := utils.GetConfig()
	if !cfg.SyncEnabled {
		return nil
	}
	source, ok := client.(srv.RangeSource)
	if !ok {
		return fmt.Errorf("range retrieval unsupported by client")
	}
	var explicitStart time.Time
	var err error
	if cfg.SyncStart != "" {
		explicitStart, err = time.Parse(time.RFC3339, cfg.SyncStart)
		if err != nil {
			return fmt.Errorf("SYNC_START: %w", err)
		}
	}
	// Two seconds is an explicit inferred-closure lag, not exchange finality.
	end, err := srv.IntervalStart(time.Now().UTC().Add(-2*time.Second), cfg.Crypto.KlineInterval)
	if err != nil {
		return err
	}
	for _, pair := range cfg.Crypto.Pairs {
		pair = strings.ToUpper(strings.TrimSpace(pair))
		start := explicitStart
		if start.IsZero() {
			last, err := repo.FindLast(ctx, pair)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("SYNC_START required for empty series %s", pair)
			}
			if err != nil {
				return fmt.Errorf("load legacy recovery start: %w", err)
			}
			start = last.Time
		}
		if !start.Before(end) {
			continue
		}
		_, err = RunBackfill(ctx, BackfillPlan{Query: srv.RangeQuery{Symbol: pair, Interval: cfg.Crypto.KlineInterval, Start: start, End: end, Limit: 720}, MinRequestInterval: 200 * time.Millisecond}, source, repo)
		if err != nil {
			return fmt.Errorf("backfill %s: %w", pair, err)
		}
	}
	return nil
}
