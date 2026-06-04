// bypass.go — real bypass-route handlers on catalog.Handlers (D-21, D-69).
//
// The four endpoints here are reachable WITHOUT X-Org-Id (server.go's
// orgContextMiddleware checks the /v1/ prefix). After Plan 03-10 deletes
// the Wave 0 transitional stubs, the production binary serves these four
// paths through the same StrictServerInterface impl that owns every
// catalog CRUD path — single dispatch surface, single test fixture.
//
// Liveness (/healthz) is intentionally dependency-free: the LB only asks
// "is the process up?". Readiness (/readyz) probes pgxpool + Redis +
// schema_migrations to gate traffic when an upstream dep is degraded.
package catalog

import (
	"bytes"
	"context"
	_ "embed"
	"log/slog"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// GetExprFunctions — org-independent catalog of the condition-DSL functions,
// served from expr.Catalog() so the UI autocomplete has a single source.
func (*Handlers) GetExprFunctions(_ context.Context, _ api.GetExprFunctionsRequestObject) (api.GetExprFunctionsResponseObject, error) {
	cat := expr.Catalog()
	fns := make([]api.ExprFunction, len(cat))
	for i, f := range cat {
		fns[i] = api.ExprFunction{Ns: f.Ns, Name: f.Name, Arity: f.Arity, Signature: f.Signature, Summary: f.Summary}
	}
	return api.GetExprFunctions200JSONResponse(api.ExprFunctionCatalog{Functions: fns}), nil
}

// readyzTimeout is the per-probe budget for pgxpool.Ping + Cache.Ping +
// schema_migrations read. 2s mirrors the Wave 0 stub and matches the
// T-1-DOS-READYZ mitigation from Phase 1: a wedged downstream cannot
// hang the readyz handler past this ceiling.
const readyzTimeout = 2 * time.Second

// docsHTML is the Scalar API Reference viewer page baked into the binary
// via go:embed. The page loads /openapi.yaml from the same origin so the
// served spec always matches the running binary (D-45).
//
//go:embed docs.html
var docsHTML []byte

// specYAMLBytes is the memoised spec→YAML marshal so /openapi.yaml does
// not re-marshal on every hit. api.GetSpec() returns a *kin.openapi.T
// which must be yaml-marshalled to produce the wire form — doing this
// once at first hit is a load-bearing optimisation for the LB
// health-check path and the docs-load path.
var (
	specYAMLOnce  sync.Once
	specYAMLBytes []byte
	specYAMLErr   error
)

// GetHealthz — liveness probe; returns 200 unconditionally. The LB only
// cares whether the process is still serving HTTP; deep checks (DB,
// Redis) live in GetReadyz so a degraded dependency does NOT cause the
// LB to kill the pod (which would compound the outage).
func (h *Handlers) GetHealthz(_ context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	return api.GetHealthz200JSONResponse(api.HealthResponse{Status: api.Alive}), nil
}

// GetReadyz — readiness probe. Pings pgxpool + Cache + reads the latest
// schema_migrations row. Returns 200 only when every check passes;
// 503 + degraded body when any one fails or schema_migrations.dirty is
// true. The bypass routing means there is no X-Org-Id, so the handler
// uses Pool directly (not orgDB which rejects unscoped queries via the
// SQLChecker).
func (h *Handlers) GetReadyz(ctx context.Context, _ api.GetReadyzRequestObject) (api.GetReadyzResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, readyzTimeout)
	defer cancel()

	resp := api.ReadinessResponse{Status: api.ReadinessResponseStatusOk}
	resp.Checks.Db = api.ReadinessCheck("ok")
	resp.Checks.Redis = api.ReadinessCheck("ok")
	resp.Checks.Migrations.Version = 0
	degraded := false

	if h.deps.Pool != nil {
		if err := h.deps.Pool.Ping(ctx); err != nil {
			h.logReadyzFailure(ctx, "pool ping", err)
			resp.Checks.Db = api.ReadinessCheck("fail")
			degraded = true
		}
	} else {
		resp.Checks.Db = api.ReadinessCheck("fail")
		degraded = true
	}

	if h.deps.Cache != nil {
		if err := h.deps.Cache.Ping(ctx); err != nil {
			h.logReadyzFailure(ctx, "cache ping", err)
			resp.Checks.Redis = api.ReadinessCheck("fail")
			degraded = true
		}
	} else {
		resp.Checks.Redis = api.ReadinessCheck("fail")
		degraded = true
	}

	// schema_migrations is operator metadata shared across orgs; reading it
	// directly via Pool (not orgDB) is the one legitimate raw-pool surface
	// in the running API binary — matches the Wave 0 stub semantics.
	if h.deps.Pool != nil {
		var ver int
		var dirty bool
		if err := h.deps.Pool.QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`,
		).Scan(&ver, &dirty); err != nil {
			h.logReadyzFailure(ctx, "migrations query", err)
			degraded = true
		} else {
			resp.Checks.Migrations.Version = ver
			if dirty {
				degraded = true
			}
		}
	}

	if degraded {
		resp.Status = api.ReadinessResponseStatusDegraded
		return api.GetReadyz503JSONResponse(resp), nil
	}
	return api.GetReadyz200JSONResponse(resp), nil
}

// logReadyzFailure centralises the slog warn for any failing check so the
// readyz path emits a single canonical log line shape for operator
// monitoring/alerting.
func (h *Handlers) logReadyzFailure(ctx context.Context, op string, err error) {
	if h.deps.Logger == nil {
		slog.WarnContext(ctx, "readyz check failed", "op", op, "err", err)
		return
	}
	h.deps.Logger.WarnContext(ctx, "readyz check failed", "op", op, "err", err)
}

// GetOpenAPISpec serves the embedded openapi spec marshalled to YAML.
// api.GetSpec() returns the parsed kin-openapi T; yaml.Marshal on first
// hit produces the wire bytes and the result is cached for the process
// lifetime so every subsequent call is a memcpy.
//
// The spec response in the spec only declares a 200 wrapper — a failure
// in api.GetSpec() / yaml.Marshal here would be a build-time defect
// (the embedded bytes are baked into the binary). We surface it as a
// returned error so the strict-server default error handler emits 500
// with the request_id middleware still wrapping the body.
func (h *Handlers) GetOpenAPISpec(_ context.Context, _ api.GetOpenAPISpecRequestObject) (api.GetOpenAPISpecResponseObject, error) {
	specYAMLOnce.Do(func() {
		swagger, err := api.GetSpec()
		if err != nil {
			specYAMLErr = err
			return
		}
		body, mErr := yaml.Marshal(swagger)
		if mErr != nil {
			specYAMLErr = mErr
			return
		}
		specYAMLBytes = body
	})
	if specYAMLErr != nil {
		return nil, specYAMLErr
	}
	return api.GetOpenAPISpec200ApplicationyamlResponse{
		Body:          bytes.NewReader(specYAMLBytes),
		ContentLength: int64(len(specYAMLBytes)),
	}, nil
}

// GetDocs serves the embedded Scalar viewer HTML — the static page that
// fetches /openapi.yaml from the same origin and renders the interactive
// docs. No CDN fetch is required for the viewer itself beyond the script
// the HTML loads (a pinned Scalar release; see docs.html).
func (h *Handlers) GetDocs(_ context.Context, _ api.GetDocsRequestObject) (api.GetDocsResponseObject, error) {
	return api.GetDocs200TexthtmlResponse{
		Body:          bytes.NewReader(docsHTML),
		ContentLength: int64(len(docsHTML)),
	}, nil
}
