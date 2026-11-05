package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"infosir/cmd/config"
	"infosir/internal/db"
	"infosir/internal/db/repository"
	"infosir/internal/jobs"
	"infosir/internal/srv"
	"infosir/internal/utils"
	"infosir/pkg/crypto"
	natsinfosir "infosir/pkg/nats"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if err := config.LoadConfig(); err != nil {
		return err
	}
	if _, err := srv.IntervalStart(time.Now(), config.Cfg.Crypto.KlineInterval); err != nil {
		return err
	}
	utils.InitLogger()
	defer utils.Logger.Sync()
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancelStartup := context.WithTimeout(stopCtx, 30*time.Second)
	defer cancelStartup()
	pool, err := db.InitDatabaseContext(startup)
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := repository.NewKlineRepository(pool)
	if err = repo.BindProfile(startup, config.Cfg.Crypto.Market, config.Cfg.Crypto.KlineInterval, config.Cfg.ConfirmLegacyProfile); err != nil {
		return err
	}
	nc, js, err := natsinfosir.InitNATSJetStream()
	if err != nil {
		return err
	}
	defer nc.Close()
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()
	if err = natsinfosir.StartJetStreamConsumer(consumerCtx, js, repo); err != nil {
		return err
	}
	client := crypto.NewBinanceClient()
	service := srv.NewInfoSirService(client, natsinfosir.NewNatsJetStreamClient(js))
	producerCtx, cancelProducers := context.WithCancel(stopCtx)
	defer cancelProducers()
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	wg.Add(1)
	go func() { defer wg.Done(); jobs.RunScheduledRequests(producerCtx, service, time.Minute) }()
	if config.Cfg.SyncEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := jobs.RunHistoricalSyncChecked(producerCtx, repo, client); e != nil && !errors.Is(e, context.Canceled) {
				failures <- fmt.Errorf("historical sync: %w", e)
			}
		}()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("alive\n")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if !nc.IsConnected() || pool.Ping(ctx) != nil {
			http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("dependencies ready; data completeness not asserted\n"))
	})
	httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", config.Cfg.HTTPPort), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return producerCtx }}
	go func() {
		if e := httpSrv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			failures <- e
		}
	}()
	var failure error
	select {
	case <-stopCtx.Done():
	case failure = <-failures:
	}
	cancelProducers()
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	httpErr := httpSrv.Shutdown(shutdown)
	if httpErr != nil {
		_ = httpSrv.Close()
	}
	jobsDone := make(chan struct{})
	go func() { wg.Wait(); close(jobsDone) }()
	select {
	case <-jobsDone:
	case <-shutdown.Done():
		return errors.Join(failure, httpErr, shutdown.Err())
	}
	// Consumers keep their independent context until all accepted callbacks drain.
	drainErr := nc.Drain()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for !nc.IsClosed() {
		select {
		case <-ticker.C:
		case <-shutdown.Done():
			return errors.Join(failure, httpErr, drainErr, shutdown.Err())
		}
	}
	cancelConsumer()
	return errors.Join(failure, httpErr, drainErr)
}
