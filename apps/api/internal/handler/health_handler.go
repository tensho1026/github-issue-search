package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

type HealthHandler struct {
	responder response.Responder
}

func NewHealthHandler(responder response.Responder) HealthHandler {
	return HealthHandler{responder: responder}
}

func (h HealthHandler) Check(ctx *gin.Context) {
	h.responder.Data(ctx, http.StatusOK, gin.H{"status": "ok"})
}
