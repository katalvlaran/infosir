package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/joho/godotenv"
)

// Config определяет все необходимые поля для микросервиса InfoSir.
type Config struct {
	// Общие настройки
	AppEnv   string `env:"APP_ENV" envDefault:"dev"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"debug"`

	// NATS
	NatsURL     string `env:"NATS_URL,required"`
	NatsSubject string `env:"NATS_SUBJECT" envDefault:"infosir_kline"`

	// Binance
	BinanceBaseURL string `env:"BINANCE_BASE_URL" envDefault:"https://api.binance.com"`

	// Список пар. Например: BTCUSDT,ETHUSDT
	// Значения разделяются запятой при загрузке (envSeparator:",").
	Pairs []string `env:"PAIRS" envSeparator:","`

	// Параметры периодического запроса
	KlineInterval string `env:"KLINE_INTERVAL" envDefault:"1m"` // интервал свечей
	KlineLimit    int    `env:"KLINE_LIMIT" envDefault:"10"`    // сколько свечей за раз

	// HTTP-сервер (при обработке запросов от оркестратора)
	HTTPPort int `env:"HTTP_PORT" envDefault:"8080"`
}

// LoadConfig загружает .env (если есть), затем парсит переменные окружения
// в структуру Config и валидирует её.
func LoadConfig() (*Config, error) {
	// 1. Считываем из .env (если файла нет, просто игнорируем ошибку).
	_ = godotenv.Load()

	// 2. Создаём экземпляр конфигурации и заполняем значениями из ENV
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse env into config: %w", err)
	}

	// 3. Валидируем конфиг
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return cfg, nil
}

// Validate проводит базовую проверку корректности полей.
func (c *Config) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.NatsURL, validation.Required),
		validation.Field(&c.BinanceBaseURL, validation.Required),
		validation.Field(&c.Pairs, validation.Required, validation.Length(1, 0)), // как минимум одна пара
		validation.Field(&c.KlineLimit, validation.Required, validation.Min(1)),
		validation.Field(&c.HTTPPort, validation.Required, validation.Min(1)),
	)
}
