-- 0001_init_futures_klines.sql

BEGIN;

-- 1. Подключаем расширение TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 2. Создаём основную таблицу
-- Обратите внимание: поле называется time TIMESTAMP
-- (в Вашей задаче: "open_time TIMESTAMP WITHOUT TIME ZONE" => может называться 'time',
--  если вы хотите один в один match create_hypertable(..., 'time')
-- для ясности ниже поле называется "time" (не open_time).
CREATE TABLE IF NOT EXISTS futures_klines (
                                               time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
                                               symbol TEXT NOT NULL,
                                               open_price DOUBLE PRECISION NOT NULL,
                                               high_price DOUBLE PRECISION NOT NULL,
                                               low_price DOUBLE PRECISION NOT NULL,
                                               close_price DOUBLE PRECISION NOT NULL,
                                               volume DOUBLE PRECISION NOT NULL,
                                               quote_volume DOUBLE PRECISION NOT NULL,
                                               trades BIGINT,
                                               taker_buy_base_volume DOUBLE PRECISION,
                                               taker_buy_quote_volume DOUBLE PRECISION,

                                               PRIMARY KEY (symbol, time)
    );

-- 3. Преобразуем таблицу в hypertable
--   - 'futures_klines' - имя таблицы
--   - 'time' - колонка, по которой идёт тайм-партиционирование
--   - 'symbol' - дополнительный space partition
--   - number_partitions => 20 (примерно)
SELECT create_hypertable(
               'futures_klines',
               'time',
               'symbol',
               20,
               create_default_indexes => TRUE
       );

-- 4. Включим политику сжатия (compression)
--   Укажем, что у нас compress_orderby = 'time DESC'
ALTER TABLE futures_klines
    SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC'
    );

-- Политика автокомпрессии (через 30 дней)
SELECT add_compression_policy(
               'futures_klines',
               INTERVAL '30 days'
       );

-- ================================================
-- создаём Continuous aggregates под разные таймфреймы
-- ================================================

-- 15m
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

SELECT add_continuous_aggregate_policy('klines_15m',
                                       start_offset => INTERVAL '1 day',
                                       end_offset => INTERVAL '1 minute',
                                       schedule_interval => INTERVAL '5 minutes');


-- 30m
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

SELECT add_continuous_aggregate_policy('klines_30m',
                                       start_offset => INTERVAL '2 days',
                                       end_offset => INTERVAL '1 minute',
                                       schedule_interval => INTERVAL '5 minutes');


-- 1h
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

SELECT add_continuous_aggregate_policy('klines_1h',
                                       start_offset => INTERVAL '7 days',
                                       end_offset => INTERVAL '1 minute',
                                       schedule_interval => INTERVAL '5 minutes');


-- 4h
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

SELECT add_continuous_aggregate_policy('klines_4h',
                                       start_offset => INTERVAL '14 days',
                                       end_offset => INTERVAL '1 minute',
                                       schedule_interval => INTERVAL '10 minutes');


-- 1d
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

SELECT add_continuous_aggregate_policy('klines_1d',
                                       start_offset => INTERVAL '30 days',
                                       end_offset => INTERVAL '5 minutes',
                                       schedule_interval => INTERVAL '15 minutes');

COMMIT;
