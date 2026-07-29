package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/router"
	"github.com/tensho1026/github-issue-search/apps/api/internal/server"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid API configuration", "error", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	httpHandler, err := router.New(router.Dependencies{
		Config:    cfg,
		Logger:    logger,
		Responder: response.NewResponder(),
	})
	if err != nil {
		logger.Error("compose API router", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpHandler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		logger.Error("listen for API traffic", "error", err)
		os.Exit(1)
	}
	logger.Info("starting IssueScout API", "address", listener.Addr().String())

	processContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := server.Run(
		processContext,
		httpServer,
		listener,
		cfg.ShutdownTimeout,
		logger,
	); err != nil {
		logger.Error("IssueScout API stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
