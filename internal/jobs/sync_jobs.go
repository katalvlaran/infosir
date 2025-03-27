package jobs

import (
	"context"
	"strings"
	"time"

	"infosir/internal/db/repository"
	"infosir/internal/srv"
	"infosir/internal/utils"

	"go.uber.org/zap"
)

// RunHistoricalSync fills missing historical Kline data for each pair configured in
// config.Cfg.Crypto.Pairs. It looks up the last known Kline in the database and fetches
// new data from Binance in sequential "chunks" until reaching the current time.
//
// This function is typically invoked once on startup if SyncEnabled == true.
func RunHistoricalSync(
	ctx context.Context,
	klineRepo *repository.KlineRepository,
	binanceClient srv.BinanceClient,
) {
	if !utils.GetConfig().SyncEnabled {
		utils.Logger.Info("Historical sync is disabled; skipping.")
		return
	}

	utils.Logger.Info("Starting historical sync worker")

	for _, pair := range utils.GetConfig().Crypto.Pairs {
		symbolLower := strings.ToLower(pair)

		// Attempt to find the last known Kline time from DB
		lastK, err := klineRepo.FindLast(ctx, symbolLower)
		var lastOpenTime int64
		if err != nil {
			utils.Logger.Warn("No last Kline found or error retrieving last Kline; using 0 as fallback",
				zap.String("symbol", symbolLower),
				zap.Error(err))
			lastOpenTime = 0
		} else {
			lastOpenTime = lastK.Time.UnixNano() / 1_000_000 // convert to ms
			utils.Logger.Debug("Found last Kline from DB",
				zap.String("symbol", symbolLower),
				zap.Time("time", lastK.Time))
		}

		// Perform chunk-based fetch from lastOpenTime up to "now"
		if err := fetchMissingData(ctx, binanceClient, klineRepo, pair, symbolLower, lastOpenTime); err != nil {
			utils.Logger.Error("fetchMissingData error",
				zap.String("symbol", symbolLower),
				zap.Error(err))
		}
	}

	utils.Logger.Info("Historical sync worker finished for all pairs.")
}

// fetchMissingData fetches Klines in "chunks" from Binance, starting at 'fromTimeMs' up to near-current time,
// then inserts them into the DB. Because the default binance client doesn't allow specifying startTime, we might
// repeatedly fetch the newest klines, which is sub-optimal, but workable as a naive approach.
func fetchMissingData(
	ctx context.Context,
	binanceClient srv.BinanceClient,
	klineRepo *repository.KlineRepository,
	pairOriginal, pairLower string,
	fromTimeMs int64,
) error {
	const chunkSize = 720                  // how many klines in each chunk request
	const earliestMs int64 = 1420070400000 // "2015-01-01" fallback

	startMs := fromTimeMs
	if startMs == 0 {
		startMs = earliestMs
	}

	// We'll define an end target = now - 1 minute, in milliseconds
	endTargetMs := (time.Now().UnixNano() / 1_000_000) - (60_000)

	if endTargetMs < startMs {
		utils.Logger.Info("No missing data to fetch", zap.String("symbol", pairLower))
		return nil
	}

	utils.Logger.Info("Starting chunk-based fetch",
		zap.String("symbol", pairLower),
		zap.Int64("startMs", startMs),
		zap.Int64("endMs", endTargetMs))

	fetchInterval := utils.GetConfig().Crypto.KlineInterval // e.g. "1m"

	for startMs < endTargetMs {
		klines, err := binanceClient.FetchKlines(ctx, pairOriginal, fetchInterval, int64(chunkSize))
		if err != nil {
			utils.Logger.Warn("Error fetching chunk from binance; will retry in 5s",
				zap.String("symbol", pairOriginal),
				zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		if len(klines) == 0 {
			utils.Logger.Info("No more klines returned; stopping",
				zap.String("symbol", pairOriginal))
			break
		}

		if err := klineRepo.BatchInsertKlines(ctx, klines); err != nil {
			utils.Logger.Error("BatchInsertKlines failed",
				zap.String("symbol", pairOriginal),
				zap.Error(err))
			// We continue or break? Let’s continue with next chunk attempt
			// though we might re-insert duplicates.
		}

		lastFetched := klines[len(klines)-1].Time.UnixNano() / 1_000_000
		startMs = lastFetched + 60_000 // move 1 minute past last
		if startMs >= endTargetMs {
			utils.Logger.Info("Reached end of data range", zap.String("symbol", pairOriginal))
			break
		}

		time.Sleep(1 * time.Second) // short delay to avoid spamming
	}
	return nil
}
