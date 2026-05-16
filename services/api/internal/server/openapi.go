// Package server: openapi.go owns the /openapi.yaml and /docs bypass
// routes per D-45 (Phase 2). Both register at chi root in NewMux BEFORE
// the /v1 sub-router so they are reachable without an X-Org-Id header
// (D-21 bypass list extension). In v0.1 with stub auth, both routes are
// public; v1 RBAC will gate /docs behind admin role.

package server

import (
	_ "embed"
	"net/http"
)

// docsHTML is the static HTML viewer page baked into the binary via
// //go:embed. The page loads the Scalar API Reference viewer from CDN
// and points it at /openapi.yaml from the same origin so the served spec
// always matches the running binary (D-45).
//
//go:embed docs.html
var docsHTML []byte

// OpenAPISpecHandler returns 200 + the embedded spec bytes (D-45).
// Content-Type is application/yaml. specBytes is captured at NewMux time;
// serving the embedded bytes guarantees the spec matches the running
// binary even if openapi/openapi.yaml on disk is mutated post-deploy.
//
// Bypasses OrgContext per D-21 because it must be reachable without
// X-Org-Id (the spec endpoint is meant for unauthenticated readers in v0.1;
// v1 RBAC will gate it).
func OpenAPISpecHandler(specBytes []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(specBytes)
	}
}

// DocsHandler returns 200 + the Scalar viewer HTML (D-45). The HTML
// fetches /openapi.yaml from the same origin; no extra static assets
// in the repo. Bypasses OrgContext per D-21.
func DocsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(docsHTML)
	}
}
