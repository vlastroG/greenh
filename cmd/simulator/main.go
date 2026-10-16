// Command simulator acts as an ESP32 node with soil attached.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vlastrog/greenh/sim"
)

func main() {
	path := flag.String("config", "/etc/simulator.yaml", "path to simulator config")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := sim.LoadConfig(*path)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := sim.NewRunner(cfg, sim.NewHTTPBackend(cfg.Backend, nil), time.Now, log)
	log.Info("simulator started", "backend", cfg.Backend, "interval", cfg.Interval.Duration, "speed", cfg.Speed)
	if err := r.Run(ctx); err != nil {
		log.Error("run", "err", err)
		os.Exit(1)
	}
}
