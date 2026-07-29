package config

import "testing"

func TestLoadUsesDefaultPort(t *testing.T) {
	t.Setenv("PORT", "")

	cfg := Load()

	if cfg.Port != defaultPort {
		t.Fatalf("Load().Port = %q, want %q", cfg.Port, defaultPort)
	}
}

func TestLoadUsesConfiguredPort(t *testing.T) {
	t.Setenv("PORT", "9090")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("Load().Port = %q, want %q", cfg.Port, "9090")
	}
}
