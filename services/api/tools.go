//go:build tools

// Package tools tracks dev-time and runtime module dependencies so that
// `go mod tidy` does not prune them before per-package source files are
// added in later plans (Plans 02-07). Plan 01-01 documents the locked
// stack surface here; subsequent plans import these packages from real
// source files, at which point this file becomes redundant and can be
// deleted. Activate with `go vet -tags tools ./...`. The `tools` build
// tag isolates this file from production binary builds.
//
// The sqlc CLI and migrate CLI are installed as separate binaries (Wave 0 /
// Task 1 of Plan 01-01) — they are NOT imported here. This file pins
// runtime stack dependencies (chi, pgx, golang-migrate library, uuid,
// redis, pg_query_go, OTel SDK + exporters + contrib, testcontainers,
// testify) so subsequent plans can author code immediately without
// dependency churn.
//
// CGO note: importing pganalyze/pg_query_go/v6 triggers a ~3 minute CGO
// build on first compile per machine; subsequent builds use the build
// cache. Do NOT set CGO_ENABLED=0.
package tools

import (
	// HTTP + DB stack
	_ "github.com/go-chi/chi/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/google/uuid"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/pganalyze/pg_query_go/v6"
	_ "github.com/redis/go-redis/v9"

	// OTel SDK + exporters + contrib
	_ "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	_ "go.opentelemetry.io/otel"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	_ "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	_ "go.opentelemetry.io/otel/propagation"
	_ "go.opentelemetry.io/otel/sdk"
	_ "go.opentelemetry.io/otel/sdk/resource"
	_ "go.opentelemetry.io/otel/sdk/trace"
	_ "go.opentelemetry.io/otel/semconv/v1.26.0"

	// Test stack (build-tag isolates from production binary; importing here keeps go.mod consistent)
	_ "github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/require"
	_ "github.com/testcontainers/testcontainers-go"
	_ "github.com/testcontainers/testcontainers-go/modules/postgres"
)
