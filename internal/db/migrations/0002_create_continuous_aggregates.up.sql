-- +goose NO TRANSACTION
---------------------------------------------------------
-- Step 6: Optionally define continuous aggregates for various timeframes.
--  e.g. 15m, 30m, 1h, 4h, 1d
-- We'll define them as materialized views with timescaledb.continuous.
-- This can significantly improve read performance on aggregated intervals.
---------------------------------------------------------

------------------- 15m
CREATE MATERIALIZED VIEW IF NOT EXISTS klines_15m
            WITH (timescaledb.continuous) AS
SELECT
    time_bucket('15 minutes', time) AS bucket,
    symbol,
    first(open_price, time) AS open,
    max(high_price) AS high,
    min(low_price) AS low,
    last(close_price, time) AS close,
    sum(volume) AS volume,
    sum(quote_volume) AS quote_volume,
    sum(trades) AS trades,
    sum(taker_buy_base_volume) AS taker_buy_base_volume,
    sum(taker_buy_quote_volume) AS taker_buy_quote_volume
FROM futures_klines
GROUP BY bucket, symbol;

SELECT add_continuous_aggregate_policy(
               'klines_15m',
               start_offset => INTERVAL '1 day',
               end_offset => INTERVAL '1 minute',
               schedule_interval => INTERVAL '5 minutes'
       );

------------------- 30m
CREATE MATERIALIZED VIEW IF NOT EXISTS klines_30m
            WITH (timescaledb.continuous) AS
SELECT
    time_bucket('30 minutes', time) AS bucket,
    symbol,
    first(open_price, time) AS open,
    max(high_price) AS high,
    min(low_price) AS low,
    last(close_price, time) AS close,
    sum(volume) AS volume,
    sum(quote_volume) AS quote_volume,
    sum(trades) AS trades,
    sum(taker_buy_base_volume) AS taker_buy_base_volume,
    sum(taker_buy_quote_volume) AS taker_buy_quote_volume
FROM futures_klines
GROUP BY bucket, symbol;

SELECT add_continuous_aggregate_policy(
               'klines_30m',
               start_offset => INTERVAL '2 days',
               end_offset => INTERVAL '1 minute',
               schedule_interval => INTERVAL '5 minutes'
       );

------------------- 1h
CREATE MATERIALIZED VIEW IF NOT EXISTS klines_1h
            WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    symbol,
    first(open_price, time) AS open,
    max(high_price) AS high,
    min(low_price) AS low,
    last(close_price, time) AS close,
    sum(volume) AS volume,
    sum(quote_volume) AS quote_volume,
    sum(trades) AS trades,
    sum(taker_buy_base_volume) AS taker_buy_base_volume,
    sum(taker_buy_quote_volume) AS taker_buy_quote_volume
FROM futures_klines
GROUP BY bucket, symbol;

SELECT add_continuous_aggregate_policy(
               'klines_1h',
               start_offset => INTERVAL '7 days',
               end_offset => INTERVAL '1 minute',
               schedule_interval => INTERVAL '5 minutes'
       );

------------------- 4h
CREATE MATERIALIZED VIEW IF NOT EXISTS klines_4h
            WITH (timescaledb.continuous) AS
SELECT
    time_bucket('4 hours', time) AS bucket,
    symbol,
    first(open_price, time) AS open,
    max(high_price) AS high,
    min(low_price) AS low,
    last(close_price, time) AS close,
    sum(volume) AS volume,
    sum(quote_volume) AS quote_volume,
    sum(trades) AS trades,
    sum(taker_buy_base_volume) AS taker_buy_base_volume,
    sum(taker_buy_quote_volume) AS taker_buy_quote_volume
FROM futures_klines
GROUP BY bucket, symbol;

SELECT add_continuous_aggregate_policy(
               'klines_4h',
               start_offset => INTERVAL '14 days',
               end_offset => INTERVAL '1 minute',
               schedule_interval => INTERVAL '10 minutes'
       );

------------------- 1d
CREATE MATERIALIZED VIEW IF NOT EXISTS klines_1d
            WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    symbol,
    first(open_price, time) AS open,
    max(high_price) AS high,
    min(low_price) AS low,
    last(close_price, time) AS close,
    sum(volume) AS volume,
    sum(quote_volume) AS quote_volume,
    sum(trades) AS trades,
    sum(taker_buy_base_volume) AS taker_buy_base_volume,
    sum(taker_buy_quote_volume) AS taker_buy_quote_volume
FROM futures_klines
GROUP BY bucket, symbol;

SELECT add_continuous_aggregate_policy(
               'klines_1d',
               start_offset => INTERVAL '30 days',
               end_offset => INTERVAL '5 minutes',
               schedule_interval => INTERVAL '15 minutes'
       );