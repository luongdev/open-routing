// bypass_placeholders.go — minimal 200 stubs for the four bypass routes
// (GetHealthz, GetReadyz, GetOpenAPISpec, GetDocs) that catalog.Handlers
// must implement to satisfy api.StrictServerInterface.
//
// Why placeholders: Wave 0's `server.Wave0TempStubs` already serves the
// canonical bypass bodies in production (real /healthz + /readyz that
// hit pg / redis, real openapi.yaml bytes, real docs.html). Plan 03-10
// REPLACES these placeholders with the production bodies (likely by
// moving Wave0TempStubs's bypass methods into a small infrastructure
// package the catalog.Handlers consumes via Deps, or by delegating
// directly).
//
// For Plan 03-05 the bypass methods return minimal-but-valid 200
// responses so:
//   1. var _ api.StrictServerInterface = (*Handlers)(nil) compiles, AND
//   2. The strict-server pipeline doesn't crash if a test accidentally
//      routes a request through catalog.Handlers's bypass methods.
//
// The Wave0TempStubs in services/api/internal/server/ stays the real
// strict-server impl wired into main.go and test/isolation until
// Plan 03-10 swaps it for catalog.New(deps).
package catalog

import (
	"bytes"
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// placeholderDocsHTML is a one-line stub served by GetDocs in Plan 03-05
// only. Plan 03-10 swaps in the real Scalar viewer HTML from
// services/api/internal/server/docs.html.
var placeholderDocsHTML = []byte("<!doctype html><title>Docs placeholder</title>")

// placeholderOpenAPIYAML is a one-line stub served by GetOpenAPISpec in
// Plan 03-05 only. Plan 03-10 swaps in the embedded openapi.yaml bytes.
var placeholderOpenAPIYAML = []byte("# openapi placeholder — Plan 03-10 replaces with real spec\n")

// GetDocs — PLACEHOLDER. Plan 03-10 replaces with the Scalar API
// Reference viewer HTML (currently served by server.Wave0TempStubs).
func (h *Handlers) GetDocs(_ context.Context, _ api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	return api.GetDocs200TexthtmlResponse{
		Body:          bytes.NewReader(placeholderDocsHTML),
		ContentLength: int64(len(placeholderDocsHTML)),
	}, nil
}

// GetHealthz — PLACEHOLDER. Plan 03-10 swaps in the real liveness body
// (still {"status":"alive"} — production behaviour is the same).
func (h *Handlers) GetHealthz(_ context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse(api.HealthResponse{Status: api.Alive}), nil
}

// GetOpenAPISpec — PLACEHOLDER. Plan 03-10 swaps in the embedded
// openapi.yaml bytes (currently served by server.Wave0TempStubs).
func (h *Handlers) GetOpenAPISpec(_ context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	return api.GetOpenAPISpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(placeholderOpenAPIYAML),
		ContentLength: int64(len(placeholderOpenAPIYAML)),
	}, nil
}

// GetReadyz — PLACEHOLDER. Plan 03-10 swaps in the real pg + redis
// ping + schema_migrations check (currently in server.Wave0TempStubs).
// Until then, return an "ok" body — tests that actually probe readiness
// continue to route through Wave0TempStubs via the main binary.
func (h *Handlers) GetReadyz(_ context.Context, _ api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	return api.GetReadyz200JSONResponse(api.ReadinessResponse{
		Status: api.ReadinessResponseStatusOk,
		Checks: struct {
			Db         api.ReadinessCheck `json:"db"`
			Migrations struct {
				Version int `json:"version"`
			} `json:"migrations"`
			Redis api.ReadinessCheck `json:"redis"`
		}{
			Db:    api.ReadinessCheckOk,
			Redis: api.ReadinessCheckOk,
			Migrations: struct {
				Version int `json:"version"`
			}{Version: 0},
		},
	}), nil
}
