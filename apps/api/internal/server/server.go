package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Run serves requests until the process context is cancelled or the server
// fails. Cancellation starts a bounded graceful shutdown for in-flight work.
func Run(
	ctx context.Context,
	httpServer *http.Server,
	listener net.Listener,
	shutdownTimeout time.Duration,
	logger *slog.Logger,
) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		return normalizeServeError(err)
	case <-ctx.Done():
		logger.Info("shutting down IssueScout API")
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		if err := httpServer.Shutdown(shutdownContext); err != nil {
			closeErr := httpServer.Close()
			return errors.Join(
				fmt.Errorf("gracefully shut down API server: %w", err),
				closeErr,
			)
		}

		return normalizeServeError(<-serveErrors)
	}
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve API: %w", err)
}
