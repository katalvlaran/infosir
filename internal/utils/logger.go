package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"infosir/cmd/config"
)

// Logger is the global zap.Logger used throughout the app for structured logging.
var Logger *zap.Logger

// InitLogger initializes the Logger with a configuration derived from config.Cfg.LogLevel.
// On success, it populates the global Logger variable.
func InitLogger() {
	logLevel := zapcore.InfoLevel
	switch config.Cfg.LogLevel {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	}

	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(logLevel),
		Development: (config.Cfg.AppEnv == "dev"),
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json", // or "console"
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	l, err := cfg.Build()
	if err != nil {
		panic("failed to build zap logger: " + err.Error())
	}

	Logger = l
}
