// Command server runs the demo HTTP API that this repository's CI/CD pipeline
// lints, tests, builds, containerises, releases and deploys.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ValentinoTriadi/ci-cd-example/internal/api"
	"github.com/ValentinoTriadi/ci-cd-example/internal/store"
	"github.com/ValentinoTriadi/ci-cd-example/internal/version"
)

// drainDelay is how long /readyz reports "draining" before the listener
// closes, giving a load balancer time to stop sending new requests.
const drainDelay = 500 * time.Millisecond

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(os.Getenv("LOG_LEVEL"))}))

	if err := run(log); err != nil {
		log.Error("server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := loadConfig(os.Getenv)
	info := version.Get()

	log.Info("starting server",
		slog.String("addr", cfg.Addr),
		slog.String("version", info.Version),
		slog.String("commit", info.Commit),
		slog.String("buildDate", info.BuildDate),
	)

	handler := api.NewHandler(store.New(), log)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	// Signal handling: SIGTERM is what a container runtime sends on rollout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	}

	// Fail readiness first, then wait out the drain window before shutting
	// the listener down, so in-flight requests finish cleanly.
	handler.SetReady(false)
	time.Sleep(drainDelay)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Info("server stopped cleanly")

	return nil
}
