package router

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/handler"
	"github.com/tensho1026/github-issue-search/apps/api/internal/middleware"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

type Dependencies struct {
	Config    config.Config
	Logger    *slog.Logger
	Responder response.Responder
}

// New composes concrete HTTP dependencies. Feature handlers are constructed by
// the application composition root and registered here.
func New(dependencies Dependencies) (http.Handler, error) {
	if dependencies.Logger == nil {
		return nil, fmt.Errorf("compose router: logger is required")
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

	engine.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(),
		middleware.CORS(dependencies.Config.AllowedOrigins, dependencies.Responder),
		middleware.RequestLogger(dependencies.Logger),
		middleware.Recovery(dependencies.Logger, dependencies.Responder),
	)

	healthHandler := handler.NewHealthHandler(dependencies.Responder)
	api := engine.Group("/api")
	api.GET(
		"/health",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		healthHandler.Check,
	)

	engine.NoRoute(dependencies.Responder.NotFound)

	return engine, nil
}
