// Package config implements 12-factor environment variable loading for the
// services/api binary. Config is consumed by cmd/migrate (Plan 03) and
// cmd/api (Plan 06); the struct shape is locked by CONTEXT.md decisions
// D-02, D-14, and D-30.
//
// This package is leaf-level: it imports only the Go standard library and
// MUST NOT import any other internal/* package, to avoid import cycles.
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds every environment variable the API reads at startup.
// Field order matches the .env.example template for grep-ability.
type Config struct {
	DatabaseURL        string   // DATABASE_URL — required
	RedisURL           string   // REDIS_URL — required
	OTelExporter       string   // OTEL_EXPORTER — default "stdout"; accepts "otlp" (D-14)
	OTLPEndpoint       string   // OTEL_EXPORTER_OTLP_ENDPOINT — required when OTelExporter=="otlp" (D-14)
	OTLPProtocol       string   // OTEL_EXPORTER_OTLP_PROTOCOL — default "http/protobuf" (D-14)
	ListenAddr         string   // LISTEN_ADDR — default ":8080"
	ValidationMode     string   // ORGDB_VALIDATION_MODE — default "panic"; accepts "error" (D-02)
	CORSAllowedOrigins []string // CORS_ALLOWED_ORIGINS — comma-separated list of origins (D7-16)
	MatcherEnabled     bool     // MATCHER_ENABLED — default false; v0.3 W4 queue+matcher (park on no agent → matcher pull) instead of offer-now-or-fallback
	WSMaxConnsPerOrg   int      // WS_MAX_CONNS_PER_ORG — default 500; per-org agent WS connection cap on one gateway instance (0 ⇒ unlimited)
}

// Load reads every environment variable, validates defaults / enums, and
// returns either a populated *Config or a joined error listing every
// missing or invalid variable so the caller sees all gaps in one shot.
func Load() (*Config, error) {
	var errs []error

	databaseURL, err := requireEnv("DATABASE_URL")
	if err != nil {
		errs = append(errs, err)
	}

	redisURL, err := requireEnv("REDIS_URL")
	if err != nil {
		errs = append(errs, err)
	}

	otelExporter := getEnvOrDefault("OTEL_EXPORTER", "stdout")
	if otelExporter != "stdout" && otelExporter != "otlp" {
		errs = append(errs, fmt.Errorf("config: OTEL_EXPORTER must be 'stdout' or 'otlp', got %q", otelExporter))
	}

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelExporter == "otlp" && otlpEndpoint == "" {
		errs = append(errs, errors.New("config: OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_EXPORTER=otlp"))
	}

	otlpProtocol := getEnvOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	listenAddr := getEnvOrDefault("LISTEN_ADDR", ":8080")

	validationMode := getEnvOrDefault("ORGDB_VALIDATION_MODE", "panic")
	if validationMode != "panic" && validationMode != "error" {
		errs = append(errs, fmt.Errorf("config: ORGDB_VALIDATION_MODE must be 'panic' or 'error', got %q", validationMode))
	}

	var corsAllowedOrigins []string
	rawCORS := os.Getenv("CORS_ALLOWED_ORIGINS")
	if rawCORS != "" {
		parts := strings.Split(rawCORS, ",")
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				corsAllowedOrigins = append(corsAllowedOrigins, trimmed)
			}
		}
	}

	wsMaxConns, err := getEnvIntOrDefault("WS_MAX_CONNS_PER_ORG", 500)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &Config{
		DatabaseURL:        databaseURL,
		RedisURL:           redisURL,
		OTelExporter:       otelExporter,
		OTLPEndpoint:       otlpEndpoint,
		OTLPProtocol:       otlpProtocol,
		ListenAddr:         listenAddr,
		ValidationMode:     validationMode,
		CORSAllowedOrigins: corsAllowedOrigins,
		MatcherEnabled:     getEnvOrDefault("MATCHER_ENABLED", "false") == "true",
		WSMaxConnsPerOrg:   wsMaxConns,
	}, nil
}

// MustLoad calls Load and terminates the process with log.Fatal on error.
// Used by cmd/migrate (Plan 03) and cmd/api (Plan 06) main() functions
// where a failed environment lookup is a startup-fatal condition.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("config: load failed: %v", err)
	}
	return cfg
}

// requireEnv reads an env var that must be non-empty.
func requireEnv(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("config: %s is required", key)
	}
	return v, nil
}

// getEnvOrDefault reads an env var, returning def if unset or empty.
func getEnvOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// getEnvIntOrDefault parses a non-negative int env var, returning def if unset.
func getEnvIntOrDefault(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer, got %q", key, v)
	}
	return n, nil
}
