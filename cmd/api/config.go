package main

import (
	"log/slog"
	"os"
)

// config se lee una sola vez, aquí, desde variables de entorno.
type config struct {
	databaseURL string
	httpAddr    string
	logLevel    slog.Level
	corsOrigin  string
}

func loadConfig() config {
	return config{
		databaseURL: env("DATABASE_URL", "postgres://puntitos:puntitos@localhost:5432/puntitos?sslmode=disable"),
		httpAddr:    env("HTTP_ADDR", ":8080"),
		logLevel:    parseLevel(env("LOG_LEVEL", "info")),
		corsOrigin:  os.Getenv("CORS_ORIGIN"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
