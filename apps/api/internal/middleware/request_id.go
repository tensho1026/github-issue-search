package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
)

const requestIDHeader = "X-Request-ID"

var fallbackRequestIDCounter atomic.Uint64

type RequestIDGenerator func() string

func RequestID() gin.HandlerFunc {
	return RequestIDWithGenerator(generateRequestID)
}

func RequestIDWithGenerator(generate RequestIDGenerator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := strings.TrimSpace(ctx.GetHeader(requestIDHeader))
		if !validRequestID(requestID) {
			requestID = generate()
		}

		ctx.Request = ctx.Request.WithContext(
			requestcontext.WithRequestID(ctx.Request.Context(), requestID),
		)
		ctx.Header(requestIDHeader, requestID)
		ctx.Next()
	}
}

func validRequestID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func generateRequestID() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err == nil {
		return "req_" + hex.EncodeToString(random[:])
	}

	return fmt.Sprintf(
		"req_%d_%d",
		time.Now().UTC().UnixNano(),
		fallbackRequestIDCounter.Add(1),
	)
}
