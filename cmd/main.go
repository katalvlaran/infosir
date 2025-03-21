package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/cmd/handler"
	"infosir/internal/jobs"
	"infosir/internal/srv"
	"infosir/pkg/crypto"
	natsinfosir "infosir/pkg/nats"
)

// main — точка входа в приложение.
func main() {
	// 1. Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}
	fmt.Printf("Loaded config: %+v\n", cfg)

	// 2. Инициализируем логгер
	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("could not init logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck // Игнорируем ошибку при flush

	logger.Info("Logger initialized", zap.String("logLevel", cfg.LogLevel))

	// 3. Подключаемся к NATS
	natsConn, err := initNATS(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer natsConn.Close()
	logger.Info("Connected to NATS server", zap.String("url", cfg.NatsURL))

	// Инициализируем реальный binanceClient и natsClient
	binanceClient := crypto.NewBinanceClient(cfg.BinanceBaseURL)
	natsClient := natsinfosir.NewNatsClient(natsConn)

	// Создаём сервис
	service := srv.NewInfoSirService(logger, cfg, binanceClient, natsClient)

	// Запускаем HTTP-сервер:
	mux := http.NewServeMux()
	// Подключаем хендлер оркестратора, используя наш сервис:
	mux.Handle("/orchestrator/fetch", handler.OrchestratorHandler(service, logger))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		logger.Info("Starting HTTP server", zap.Int("port", cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server crashed", zap.Error(err))
		}
	}()

	// Запуск планировщика
	ctx, cancel := context.WithCancel(context.Background())
	go jobs.RunScheduledRequests(ctx, logger, cfg, service)

	// Ждём сигнала остановки
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, os.Kill)
	<-stopChan
	logger.Info("Shutting down gracefully...")
	cancel()

	// Останавливаем HTTP-сервер
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error shutting down HTTP server", zap.Error(err))
	}

	logger.Info("Service stopped.")
}

// initLogger создает zap-логгер в соответствии с уровнем логирования (debug, info и т.д.).
func initLogger(logLevel string) (*zap.Logger, error) {
	var cfg zap.Config
	if logLevel == "debug" || logLevel == "development" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	// Пример: можно выставить минимальный уровень логирования
	level := zap.DebugLevel
	if err := level.Set(logLevel); err != nil {
		// Если не смогли разобрать уровень, по умолчанию будет debug
	}
	cfg.Level = zap.NewAtomicLevelAt(level)

	return cfg.Build()
}

// initNATS подключается к NATS-серверу, используя параметры из конфигурации.
func initNATS(cfg *config.Config, logger *zap.Logger) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("InfoSirClient"), // имя клиента (для отладки в NATS-мониторинге)
		// Можно добавить опции реконнекта, задержки, логирования:
		// nats.ReconnectWait(2 * time.Second),
		// nats.MaxReconnects(5),
		// nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
		// 	logger.Warn("Disconnected from NATS", zap.Error(err))
		// }),
		// nats.ReconnectHandler(func(nc *nats.Conn) {
		// 	logger.Info("Reconnected to NATS", zap.String("url", nc.ConnectedUrl()))
		// }),
	}

	conn, err := nats.Connect(cfg.NatsURL, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
