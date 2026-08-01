package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// HealthHandler exposes process liveness without probing GitHub or PostgreSQL.
type HealthHandler struct {
	responder response.Responder
}

// NewHealthHandler binds the shared response writer to the liveness endpoint.
func NewHealthHandler(responder response.Responder) HealthHandler {
	return HealthHandler{responder: responder}
}

// Check writes an immediate successful process-liveness response.
func (h HealthHandler) Check(ctx *gin.Context) {
	h.responder.Data(ctx, http.StatusOK, gin.H{"status": "ok"})
}
