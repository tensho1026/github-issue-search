package config

import (
	"os"
	"time"
)

const (
	defaultPort = "8080"
)

// Config contains the process-level settings required to run the API.
//
// Issue #2 will extend this type with validated GitHub and CORS settings. The
// server timeouts live here from the beginning so the HTTP server is never
// started with Go's unbounded defaults.
type Config struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load reads configuration from the process environment and applies safe
// development defaults.
func Load() Config {
	return Config{
		Port:              valueOrDefault("PORT", defaultPort),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
