package util

import "go.uber.org/zap"

var Logger *zap.Logger

func InitLogger() {
	Logger = zap.Must(zap.NewProduction())
}

//func InitLogger(logLevel string) error {
//	var cfg zap.Config
//	if logLevel == "debug" || logLevel == "development" {
//		cfg = zap.NewDevelopmentConfig()
//	} else {
//		cfg = zap.NewProductionConfig()
//	}
//	level := zap.DebugLevel
//	if err := level.Set(logLevel); err != nil {
//		return err
//	}
//	cfg.Level = zap.NewAtomicLevelAt(level)
//	logger, err := cfg.Build()
//	if err != nil {
//		return err
//	}
//	Logger = logger
//
//	return nil
//
//}
