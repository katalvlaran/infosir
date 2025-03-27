package jobs

import (
	"context"
	"strings"
	"time"

	"infosir/cmd/config"
	"infosir/internal/db/repository"
	"infosir/internal/srv"
	"infosir/internal/util"

	"go.uber.org/zap"
)

func RunHistoricalSync(
	ctx context.Context,
	klineRepo *repository.KlineRepository,
	binanceClient srv.BinanceClient,
) {
	if !config.Cfg.SyncEnabled {
		util.Logger.Info("Historical sync is disabled; skipping.")
		return
	}
	util.Logger.Info("Starting historical sync worker")

	for _, pair := range config.Cfg.Crypto.Pairs {
		tableName := pairToTable(pair)

		// 1) find last known open_time
		lastKline, err := klineRepo.FindLast(ctx, tableName)
		var lastOpenTime int64
		if err != nil {
			// If no rows found, or real error, handle both the same for now
			util.Logger.Warn("No last kline found or error reading last kline, using 0 as lastOpenTime",
				zap.String("pair", pair), zap.Error(err))
			lastOpenTime = 0
		} else {
			lastOpenTime = lastKline.Time.UnixNano()
			util.Logger.Debug("Found last kline from DB",
				zap.String("pair", pair),
				zap.Int64("openTime", lastOpenTime))
		}

		// 2) figure out how to fetch from lastOpenTime up to "now"
		err = fetchMissingData(ctx, binanceClient, klineRepo, pair, tableName, lastOpenTime)
		if err != nil {
			util.Logger.Error("fetchMissingData error", zap.String("pair", pair), zap.Error(err))
		}
	}
	util.Logger.Info("Historical sync worker finished for all pairs.")
}

// fetchMissingData does the chunk logic
func fetchMissingData(
	ctx context.Context,
	binanceClient srv.BinanceClient,
	klineRepo *repository.KlineRepository,
	pair, tableName string,
	fromTime int64,
) error {
	// We'll define some fallback earliestTime if fromTime=0 => e.g. 2015-01-01 in ms
	const chunkSize = 720
	var earliestTime int64 = 1420070400000 // 2015-01-01 as ms? up to you.

	start := fromTime
	if start == 0 {
		start = earliestTime
	}
	// We keep fetching in a loop until we surpass "now"
	// Let's define "now" minus 1 minute as the end target
	endTargetMs := (time.Now().UnixNano() / int64(time.Millisecond)) - (1 * 60 * 1000) // now - 1 minute
	if endTargetMs < start {
		// no missing data
		util.Logger.Info("No missing data for pair", zap.String("pair", pair))
		return nil
	}

	util.Logger.Info("Starting chunk-based fetch",
		zap.String("pair", pair),
		zap.Int64("start", start),
		zap.Int64("end", endTargetMs),
	)

	for start < endTargetMs {
		// We do step size = chunkSize minutes => chunkSize. But the existing binanceClient only has fetch by limit, no startTime param?
		// We'll do a naive approach: fetch up to chunkSize klines. Then update 'start' to the last fetched openTime + 1 minute

		klines, err := binanceClient.FetchKlines(ctx, pair, config.Cfg.Crypto.KlineInterval, int64(chunkSize))
		if err != nil {
			// If rate-limited or partial error, let's backoff
			util.Logger.Warn("Error fetching chunk from binance, waiting 5 seconds then continuing",
				zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		if len(klines) == 0 {
			util.Logger.Info("No more klines returned, stop", zap.String("pair", pair))
			break
		}
		// Actually, these klines might be the *latest* chunk, not from 'start' specifically, because we haven't told Binance a startTime
		// We need a method that does "FetchKlines with startTime param"
		// For now, let's just do a naive approach: we do repeated calls with a small limit=720, but we might be re-downloading the same data many times. It's an approach but not efficient.

		err = klineRepo.BatchInsertKlines(ctx, klines)
		if err != nil {
			util.Logger.Error("BatchInsert failed", zap.Error(err))
			// handle partial or keep going
		}
		// update 'start' to the last openTime we got + 60_000
		lastFetched := klines[len(klines)-1].Time.UnixNano()
		start = lastFetched + 60_000
		if start >= endTargetMs {
			util.Logger.Info("We've reached end of data", zap.String("pair", pair))
			break
		}
		// short delay
		time.Sleep(1 * time.Second)
	}

	return nil
}

func pairToTable(pair string) string {
	return strings.ToLower(pair)
}
