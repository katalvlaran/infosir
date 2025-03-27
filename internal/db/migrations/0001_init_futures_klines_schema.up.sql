-- 0001_init_futures_klines_schema.up.sql

BEGIN;

CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS futures_klines (
                                              time TIMESTAMPTZ NOT NULL,
                                              symbol TEXT NOT NULL,
                                              open_price DOUBLE PRECISION NOT NULL,
                                              high_price DOUBLE PRECISION NOT NULL,
                                              low_price DOUBLE PRECISION NOT NULL,
                                              close_price DOUBLE PRECISION NOT NULL,
                                              volume DOUBLE PRECISION NOT NULL,
                                              quote_volume DOUBLE PRECISION NOT NULL,
                                              trades BIGINT NOT NULL,
                                              taker_buy_base_volume DOUBLE PRECISION NOT NULL,
                                              taker_buy_quote_volume DOUBLE PRECISION NOT NULL,
                                              PRIMARY KEY (symbol, time)
    );

SELECT create_hypertable('futures_klines', 'time', if_not_exists => TRUE, migrate_data => TRUE);

ALTER TABLE futures_klines
    SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC'
    );

SELECT add_compression_policy('futures_klines', INTERVAL '30 days');

CREATE INDEX IF NOT EXISTS idx_futures_klines_symbol ON futures_klines(symbol);

COMMIT;
