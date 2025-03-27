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
	"infosir/internal/utils"
	"infosir/pkg/crypto"
	natsinfosir "infosir/pkg/nats"
)

// main is the entry point of the infosir application.
func main() {
	// 1. Create a base context that we can cancel on OS interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Load configuration from environment variables or .env file
	if err := config.LoadConfig(); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded config: %s\n", config.Cfg.String())

	// 3. Initialize a global logger
	utils.InitLogger()
	defer utils.Logger.Sync() // flush logs on exit
	utils.Logger.Info("Logger initialized",
		zap.String("environment", config.Cfg.AppEnv),
		zap.String("logLevel", config.Cfg.LogLevel),
	)
	files, _ := os.ReadDir("/app/migrations")
	for _, f := range files {
		fmt.Printf("Found migration file: %s\n", f.Name())
	}

	// 4. Initialize Database with migrations
	dbPool, err := db.InitDatabase()
	if err != nil {
		utils.Logger.Fatal("Could not initialize DB with migrations", zap.Error(err))
	}
	defer dbPool.Close()

	utils.Logger.Info("Database connected & migrations completed",
		zap.String("dbName", config.Cfg.Database.Name),
	)

	// 5. Initialize the repository
	klineRepo := repository.NewKlineRepository(dbPool)

	// 6. Initialize NATS + JetStream
	nc, js, err := natsinfosir.InitNATSJetStream()
	if err != nil {
		utils.Logger.Fatal("Failed to init NATS JetStream", zap.Error(err))
	}
	defer nc.Close()

	utils.Logger.Info("Connected to NATS JetStream",
		zap.String("stream", config.Cfg.NATS.StreamName),
		zap.String("subject", config.Cfg.NATS.Subject),
	)

	// 7. Start the consumer that reads from JetStream and writes to DB
	if err := natsinfosir.StartJetStreamConsumer(ctx, js, klineRepo); err != nil {
		utils.Logger.Fatal("Failed to start JetStream consumer", zap.Error(err))
	}

	// 8. Create real binance client & nats client
	binanceClient := crypto.NewBinanceClient()
	natsClient := natsinfosir.NewNatsJetStreamClient(js)

	// 9. Create the InfoSir service
	infoSirService := srv.NewInfoSirService(binanceClient, natsClient)

	// 10. Possibly start the historical sync if enabled
	if config.Cfg.SyncEnabled {
		go jobs.RunHistoricalSync(ctx, klineRepo, binanceClient)
	}

	// 11. Start scheduled job to fetch/publish klines periodically
	go jobs.RunScheduledRequests(ctx, infoSirService, time.Minute)

	// 12. Start the HTTP server
	httpSrv := startHTTPServer(infoSirService)

	utils.Logger.Info("HTTP server started",
		zap.Int("port", config.Cfg.HTTPPort),
	)

	// 13. Wait for interrupt signals to shut down gracefully
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)
	<-stopChan

	utils.Logger.Info("Shutting down gracefully...")
	cancel() // Cancel the main context

	// 14. Attempt graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
		utils.Logger.Error("Error shutting down HTTP server", zap.Error(err))
	}

	utils.Logger.Info("Service stopped. Goodbye!")
}

// startHTTPServer sets up the necessary endpoints, wraps them in a mux, and starts listening.
func startHTTPServer(service srv.InfoSirService) *http.Server {
	mux := http.NewServeMux()

	// Example healthcheck
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})

	// TODO: Register the orchestrator route if needed:
	// mux.Handle("/orchestrator/fetch", handler.OrchestratorHandler(service, util.Logger))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Logger.Fatal("HTTP server crashed", zap.Error(err))
		}
	}()

	return srv
}
