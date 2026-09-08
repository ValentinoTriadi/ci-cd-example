package main

import (
	"log/slog"
	"testing"
	"time"
)

func envFunc(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestAddrFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default", nil, ":8080"},
		{"explicit addr wins", map[string]string{"ADDR": "127.0.0.1:9000", "PORT": "3000"}, "127.0.0.1:9000"},
		{"port fallback", map[string]string{"PORT": "3000"}, ":3000"},
		{"blank values ignored", map[string]string{"ADDR": "  ", "PORT": " "}, ":8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addrFromEnv(envFunc(tt.env)); got != tt.want {
				t.Errorf("addrFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDurationFromEnv(t *testing.T) {
	fallback := 10 * time.Second

	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"valid", "30s", 30 * time.Second},
		{"empty falls back", "", fallback},
		{"garbage falls back", "soon", fallback},
		{"non-positive falls back", "-5s", fallback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := durationFromEnv(tt.raw, fallback); got != tt.want {
				t.Errorf("durationFromEnv(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		raw  string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{" error ", slog.LevelError}, // surrounding whitespace is trimmed
		{"", slog.LevelInfo},
		{"chatty", slog.LevelInfo},
	}

	for _, tt := range tests {
		if got := logLevel(tt.raw); got != tt.want {
			t.Errorf("logLevel(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg := loadConfig(envFunc(map[string]string{"SHUTDOWN_TIMEOUT": "42s"}))

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.ShutdownTimeout != 42*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 42s", cfg.ShutdownTimeout)
	}
	if cfg.ReadHeaderTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.IdleTimeout <= 0 {
		t.Errorf("timeouts must be positive: %+v", cfg)
	}
}
