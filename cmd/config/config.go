package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/joho/godotenv"
)

// Cfg is the global instance of Config, filled by LoadConfig().
var Cfg Config

// Config holds all top-level configuration for the infosir application.
type Config struct {
	Database DatabaseConfig
	NATS     NATSConfig
	Crypto   CryptoConfig

	// AppEnv indicates the environment mode, e.g. "dev", "prod", or "staging".
	AppEnv string `env:"APP_ENV" envDefault:"dev"`
	// LogLevel sets the minimum log level. Possible values: "debug", "info", "warn", "error".
	LogLevel string `env:"LOG_LEVEL" envDefault:"debug"`

	// HTTPPort is the TCP port for the main HTTP server.
	HTTPPort int `env:"HTTP_PORT" envDefault:"8080"`

	// SyncEnabled toggles whether historical synchronization is run on startup.
	SyncEnabled bool `env:"SYNC_ENABLED" envDefault:"true"`
}

// DatabaseConfig holds all the required DB connection parameters.
type DatabaseConfig struct {
	// Host is the hostname or IP of the Timescale/Postgres server (e.g. "localhost").
	Host string `env:"DB_HOST" envDefault:"localhost"`
	// Port is the port number for connecting to the database (e.g. 5432).
	Port int `env:"DB_PORT" envDefault:"5432"`
	// User is the database username.
	User string `env:"DB_USER" envDefault:"root"`
	// Password is the database password.
	Password string `env:"DB_PASS" envDefault:""`
	// Name is the name of the database to connect to.
	Name string `env:"DB_NAME" envDefault:"infosir_db"`
}

// NATSConfig holds all fields required to connect to NATS/JetStream.
type NATSConfig struct {
	// URL is the connection string for NATS server, e.g. "nats://127.0.0.1:4222"
	URL string `env:"NATS_URL,required"`

	// Subject is the subject/channel used to publish new Kline data.
	Subject string `env:"NATS_SUBJECT" envDefault:"infosir_kline"`

	// StreamName is the name of the JetStream stream.
	StreamName string `env:"JETSTREAM_STREAM_NAME" envDefault:"infosir_kline_stream"`

	// ConsumerName is the name of the JetStream consumer (durable).
	ConsumerName string `env:"JETSTREAM_CONSUMER" envDefault:"infosir_kline_consumer"`
}

// CryptoConfig holds configuration for interacting with the Binance or other crypto APIs.
type CryptoConfig struct {
	// BinanceBaseURL is the base endpoint for Binance REST calls, e.g. "https://api.binance.com"
	BinanceBaseURL string `env:"BINANCE_BASE_URL" envDefault:"https://api.binance.com"`

	// BinanceKlinesPoint is the path to the klines endpoint, e.g. "api/v3/klines"
	BinanceKlinesPoint string `env:"KLINES_POINT" envDefault:"api/v3/klines"`

	// Pairs is a comma-separated list of trading pairs, e.g. "BTCUSDT,ETHUSDT"
	Pairs []string `env:"PAIRS" envSeparator:","`

	// KlineInterval is the default timeframe to fetch, e.g. "1m"
	KlineInterval string `env:"KLINE_INTERVAL" envDefault:"1m"`

	// KlineLimit is the number of klines to fetch in one request (max ~1000 for Binance).
	KlineLimit int `env:"KLINE_LIMIT" envDefault:"10"`
}

// Validate checks top-level config fields for correctness.
func (c *Config) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.HTTPPort, validation.Required, validation.Min(1)),
	)
}

// Validate checks all DatabaseConfig fields for correctness.
func (d DatabaseConfig) Validate() error {
	return validation.ValidateStruct(&d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.Port, validation.Required, validation.Min(1)),
		validation.Field(&d.User, validation.Required),
		validation.Field(&d.Password, validation.Required),
		validation.Field(&d.Name, validation.Required),
	)
}

// Validate checks NATS config fields for correctness.
func (n NATSConfig) Validate() error {
	return validation.ValidateStruct(&n,
		validation.Field(&n.URL, validation.Required),
		validation.Field(&n.StreamName, validation.Required),
		validation.Field(&n.ConsumerName, validation.Required),
	)
}

// Validate checks crypto config fields for correctness.
func (cc CryptoConfig) Validate() error {
	return validation.ValidateStruct(&cc,
		validation.Field(&cc.BinanceBaseURL, validation.Required),
		validation.Field(&cc.BinanceKlinesPoint, validation.Required),
		validation.Field(&cc.Pairs, validation.Required, validation.Length(1, 0)),
		validation.Field(&cc.KlineLimit, validation.Required, validation.Min(1)),
	)
}

// LoadConfig loads configuration from environment variables (and .env if present),
// parses them, and stores them in the package-level Cfg variable.
func LoadConfig() error {
	// Attempt to load from .env file if present. It's fine if it doesn't exist.
	if err := godotenv.Load(); err != nil {
		// Not necessarily fatal; we can log it. But let's just wrap it:
		fmt.Printf("Warning: .env load error: %v\n", err)
	}
	if err := env.Parse(&Cfg); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	if err := Cfg.Database.Validate(); err != nil {
		return fmt.Errorf("database config invalid: %w", err)
	}
	if err := Cfg.NATS.Validate(); err != nil {
		return fmt.Errorf("nats config invalid: %w", err)
	}
	if err := Cfg.Crypto.Validate(); err != nil {
		return fmt.Errorf("crypto config invalid: %w", err)
	}
	if err := Cfg.Validate(); err != nil {
		return fmt.Errorf("top-level config invalid: %w", err)
	}

	return nil
}

// String returns a debug-friendly representation of the Config struct.
func (c *Config) String() string {
	return fmt.Sprintf("Config{AppEnv=%s,LogLevel=%s,HTTPPort=%d,SyncEnabled=%v, DB=%+v, NATS=%+v, Crypto=%+v}",
		c.AppEnv, c.LogLevel, c.HTTPPort, c.SyncEnabled,
		c.Database, c.NATS, c.Crypto,
	)
}

// String returns a debug-friendly representation of DatabaseConfig.
func (d DatabaseConfig) String() string {
	return fmt.Sprintf("DatabaseConfig{Host=%s,Port=%d,User=%s,Name=%s}", d.Host, d.Port, d.User, d.Name)
}

// String returns a debug-friendly representation of NATSConfig.
func (n NATSConfig) String() string {
	return fmt.Sprintf("NATSConfig{URL=%s,Subject=%s,StreamName=%s,ConsumerName=%s}",
		n.URL, n.Subject, n.StreamName, n.ConsumerName)
}

// String returns a debug-friendly representation of CryptoConfig.
func (cc CryptoConfig) String() string {
	return fmt.Sprintf("CryptoConfig{BinanceBaseURL=%s,KlinesPoint=%s,Pairs=%v,KlineInterval=%s,KlineLimit=%d}",
		cc.BinanceBaseURL, cc.BinanceKlinesPoint, cc.Pairs, cc.KlineInterval, cc.KlineLimit)
}
