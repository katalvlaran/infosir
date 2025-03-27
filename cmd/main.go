package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.uber.org/zap"

	"infosir/cmd/config"
	"infosir/internal/db"
	"infosir/internal/db/repository"
	"infosir/internal/jobs"
	"infosir/internal/srv"
	"infosir/internal/util"
	"infosir/pkg/crypto"
	natsinfosir "infosir/pkg/nats"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// 1. Load config
	config.LoadConfig()
	fmt.Printf("Loaded config: %+v\n", config.Cfg)

	// 2. init logger
	util.InitLogger()
	defer util.Logger.Sync() //nolint:errcheck

	util.Logger.Info("Logger initialized", zap.String("logLevel", config.Cfg.LogLevel))

	// 3. init DB
	pool, err := db.InitDatabase()
	if err != nil {
		util.Logger.Fatal("Could not init DB", zap.Error(err))
	}
	klineRepo := repository.NewKlineRepository(pool)

	// 4. init NATS
	natsConn, js, err := natsinfosir.InitNATSJetStream()
	if err != nil {
		util.Logger.Fatal("Failed to init NATS JetStream", zap.Error(err))
	}
	// store natsConn somewhere, maybe keep it as before
	defer natsConn.Close()
	util.Logger.Info("Connected to NATS", zap.String("url", config.Cfg.Nats.NatsURL))

	// start consumer
	err = natsinfosir.StartJetStreamConsumer(ctx, js, klineRepo)
	if err != nil {
		util.Logger.Fatal("Failed to start JetStream consumer", zap.Error(err))
	}

	// 5. create real binanceClient & natsClient
	binanceClient := crypto.NewBinanceClient()
	natsClient := natsinfosir.NewNatsJetStreamClient(js)

	// 6. create service
	service := srv.NewInfoSirService(binanceClient, natsClient)

	// 7. start sync worker
	if config.Cfg.SyncEnabled {
		go jobs.RunHistoricalSync(ctx, klineRepo, binanceClient)
	}

	// 8. start scheduled job
	go jobs.RunScheduledRequests(ctx, service, time.Minute)

	// 9. start HTTP server
	srv := startHTTPServer(util.Logger, service)
	util.Logger.Info("HTTP server started", zap.Int("port", config.Cfg.HTTPPort))

	// 10. graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)
	<-stopChan
	util.Logger.Info("Shutting down gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		util.Logger.Error("Error shutting down HTTP server", zap.Error(err))
	}
	util.Logger.Info("Service stopped.")
}

func startHTTPServer(logger *zap.Logger, service srv.InfoSirService) *http.Server {
	mux := http.NewServeMux()
	//mux.Handle("/orchestrator/fetch" /* orchestrator handler */)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server crashed", zap.Error(err))
		}
	}()

	return srv
}
