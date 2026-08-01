package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// DatabaseHealthHandler exposes a separate readiness signal for optional
// authenticated capabilities. The core process health route stays independent.
type DatabaseHealthHandler struct {
	configured bool
	health     port.DatabaseHealth
	responder  response.Responder
}

type databaseHealthResponse struct {
	Configured bool   `json:"configured"`
	Status     string `json:"status"`
}

// NewDatabaseHealthHandler creates the optional persistence readiness handler.
func NewDatabaseHealthHandler(
	health port.DatabaseHealth,
	configured bool,
	responder response.Responder,
) DatabaseHealthHandler {
	return DatabaseHealthHandler{
		configured: configured,
		health:     health,
		responder:  responder,
	}
}

// Check reports ready only when the configured database accepts a bounded
// probe. It never includes a driver error or connection parameter.
func (handler DatabaseHealthHandler) Check(ctx *gin.Context) {
	if !handler.configured || handler.health == nil {
		handler.unavailable(ctx)
		return
	}
	if err := handler.health.Ping(ctx.Request.Context()); err != nil {
		handler.unavailable(ctx)
		return
	}
	handler.responder.Data(ctx, http.StatusOK, databaseHealthResponse{
		Configured: true,
		Status:     "ready",
	})
}

func (handler DatabaseHealthHandler) unavailable(ctx *gin.Context) {
	handler.responder.Error(ctx, apperror.New(
		apperror.CodeDatabaseUnavailable,
		"Authenticated account storage is temporarily unavailable",
		http.StatusServiceUnavailable,
	))
}
