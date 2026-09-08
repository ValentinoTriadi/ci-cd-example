package main

import (
	"log/slog"
	"strings"
	"time"
)

// config holds every tunable the server reads from the environment. Keeping it
// in one struct means the deploy stub has a single list to document.
type config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// loadConfig builds a config from the supplied lookup function, which is
// os.Getenv in production and a map in tests.
func loadConfig(getenv func(string) string) config {
	return config{
		Addr:              addrFromEnv(getenv),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   durationFromEnv(getenv("SHUTDOWN_TIMEOUT"), 10*time.Second),
	}
}

// addrFromEnv prefers an explicit ADDR, falls back to PORT (which is what most
// PaaS providers inject), and finally to the documented default.
func addrFromEnv(getenv func(string) string) string {
	if addr := strings.TrimSpace(getenv("ADDR")); addr != "" {
		return addr
	}
	if port := strings.TrimSpace(getenv("PORT")); port != "" {
		return ":" + port
	}
	return ":8080"
}

func durationFromEnv(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func logLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
