package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/joho/godotenv"
)

var Cfg Config

type Config struct {
	DB     DBConfig
	Nats   NATSConfig
	Crypto CryptoConfig

	// Общие настройки
	AppEnv   string `env:"APP_ENV" envDefault:"dev"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"debug"`
	// HTTP-сервер (при обработке запросов от оркестратора)
	HTTPPort int `env:"HTTP_PORT" envDefault:"8080"`
	// synchronization historical data
	SyncEnabled bool `env:"SYNC_ENABLED" envDefault:"true"`
}

type DBConfig struct {
	// DBHost is the Postgres/TimescaleDB server host, e.g. "localhost"
	DBHost string `env:"DB_HOST" envDefault:"localhost"`
	DBPort int    `env:"DB_PORT" envDefault:"5432"`
	DBUser string `env:"DB_USER" envDefault:"root"`
	DBPass string `env:"DB_PASS" envDefault:""`
	DBName string `env:"DB_NAME" envDefault:"telgaram_chess"` // infosir_db
}

type NATSConfig struct {
	NatsURL             string `env:"NATS_URL,required"`
	NatsSubject         string `env:"NATS_SUBJECT" envDefault:"infosir_kline"`
	JetStreamStreamName string `env:"JETSTREAM_STREAM_NAME" envDefault:"infosir_kline_stream"`
	JetStreamConsumer   string `env:"JETSTREAM_CONSUMER" envDefault:"infosir_kline_consumer"`
}

type CryptoConfig struct {
	// Binance
	BinanceBaseURL     string `env:"BINANCE_BASE_URL" envDefault:"https://api.binance.com"`
	BinanceKlinesPoint string `env:"KLINES_POINT" envDefault:"api/v3/klines"`
	// Список пар. Например: BTCUSDT,ETHUSDT
	// Значения разделяются запятой при загрузке (envSeparator:",").
	Pairs []string `env:"PAIRS" envSeparator:","`
	// Параметры периодического запроса
	KlineInterval string `env:"KLINE_INTERVAL" envDefault:"1m"` // интервал свечей
	KlineLimit    int    `env:"KLINE_LIMIT" envDefault:"10"`    // сколько свечей за раз
}

// Validate проводит базовую проверку корректности полей.
func (c *Config) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.HTTPPort, validation.Required, validation.Min(1)),
	)
}

// Validate проводит базовую проверку корректности полей.
func (c *NATSConfig) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.NatsURL, validation.Required),
		validation.Field(&c.JetStreamStreamName, validation.Required),
		validation.Field(&c.JetStreamConsumer, validation.Required),
	)
}

// Validate проводит базовую проверку корректности полей.
func (c *CryptoConfig) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.BinanceBaseURL, validation.Required),
		validation.Field(&c.BinanceKlinesPoint, validation.Required),
		validation.Field(&c.Pairs, validation.Required, validation.Length(1, 0)), // как минимум одна пара
		validation.Field(&c.KlineLimit, validation.Required, validation.Min(1)),
	)
}

// Validate проводит базовую проверку корректности полей.
func (c *DBConfig) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.DBHost, validation.Required),
		validation.Field(&c.DBPort, validation.Min(1)),
		validation.Field(&c.DBUser, validation.Required),
		validation.Field(&c.DBPass, validation.Required),
		validation.Field(&c.DBName, validation.Required),
	)
}

func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("env, load: %w", err)
	}
	if err := env.Parse(&Cfg); err != nil {
		return fmt.Errorf("env, parse: %w", err)
	}
	if err := Cfg.DB.Validate(); err != nil {
		return fmt.Errorf("DB config, validate: %w", err)
	}
	if err := Cfg.Nats.Validate(); err != nil {
		return fmt.Errorf("Nats config, validate: %w", err)
	}
	if err := Cfg.Crypto.Validate(); err != nil {
		return fmt.Errorf("Crypto config, validate: %w", err)
	}
	if err := Cfg.Validate(); err != nil {
		return fmt.Errorf("Main config, validate: %w", err)
	}

	return nil
}
