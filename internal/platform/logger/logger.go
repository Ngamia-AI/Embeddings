package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog logger at the given level name (debug, info, warn, error).
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

// LevelFromEnv reads LOG_LEVEL.
func LevelFromEnv() string {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		return v
	}
	return "info"
}
