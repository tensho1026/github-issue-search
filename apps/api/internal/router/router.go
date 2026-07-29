package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// New composes the API transport. Feature packages register their routes here
// while handlers retain ownership of HTTP translation.
func New() http.Handler {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	return engine
}
