package rest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/dudakovict/anvil/internal/config"
)

func NewServer(cfg config.HTTP, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.InfoContext(ctx, "shutdown signal received, draining connections")

		// The signal ctx is already cancelled so Shutdown needs a fresh one.
		shCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout) //nolint:contextcheck // intentional: parent ctx is cancelled
		defer cancel()

		if err := srv.Shutdown(shCtx); err != nil { //nolint:contextcheck // fresh deadline ctx, see above
			return err
		}
		// Drain ListenAndServe's ErrServerClosed so the goroutine exits.
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		slog.InfoContext(ctx, "server stopped gracefully")

		return nil
	}
}
