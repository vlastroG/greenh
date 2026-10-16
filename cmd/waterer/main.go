// Command waterer is the plant-waterer backend: API, scheduler and storage.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vlastrog/greenh/advisor/openaicompat"
	"github.com/vlastrog/greenh/api"
	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/ingest"
	"github.com/vlastrog/greenh/scheduler"
	"github.com/vlastrog/greenh/store/sqlite"
	"github.com/vlastrog/greenh/web"
)

const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log.Info("starting", "config", cfg)
	for _, w := range cfg.Warnings {
		log.Warn(w)
	}

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	var adv core.Advisor = core.RulesAdvisor{}
	if cfg.Provider == providerOpenAICompat {
		adv = openaicompat.New(openaicompat.Config{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey})
	}

	now := time.Now
	checker := scheduler.NewChecker(scheduler.CheckerConfig{
		Store:          store,
		Advisor:        adv,
		Now:            now,
		OfflineAfter:   cfg.OfflineAfter,
		AttemptTimeout: cfg.AdvisorTimeout,
		Log:            log,
	})
	sched := scheduler.NewScheduler(scheduler.SchedulerConfig{
		Pots:    store,
		Advice:  store,
		Checker: checker,
		Now:     now,
		Log:     log,
	})

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api.NewHandler(api.Deps{
		Store:   store,
		Ingest:  ingest.New(store, now, log),
		Checker: checker,
		Advisor: adv.Info(),
		Now:     now,
		Log:     log,
	}))
	mux.Handle("/", web.Handler())
	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		sched.Run(ctx)
	}()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr)

	select {
	case err := <-serveErr:
		stop()
		<-schedDone
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("shutdown", "err", err)
	}
	<-schedDone
	return nil
}
