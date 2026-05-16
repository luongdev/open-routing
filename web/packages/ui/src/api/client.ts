// Phase 2 D-38: factory client for the typed REST API.
//
// Why factory (not singleton): admin SPA constructs one client at boot
// from a known org_id; Web Component embed (apps/embed) may construct
// many clients with different getOrgId resolvers (one per Custom Element
// instance). `getOrgId` is invoked per-request so embed instances react
// to attribute changes without client recreation.

import createClient from 'openapi-fetch';
import type { Middleware } from 'openapi-fetch';
import type { paths } from './generated';

/**
 * Configuration for {@link createApiClient}.
 *
 * @property baseURL - Origin of the API server (e.g. "https://api.open-routing.io")
 *   or "" for same-origin requests.
 * @property getOrgId - Invoked PER REQUEST (not at construction) — embed
 *   instances can return different values across the life of the client
 *   as their `org-id` attribute changes.
 * @property fetch - Optional fetch override for testing. Defaults to
 *   globalThis.fetch.
 */
export interface CreateApiClientConfig {
  baseURL: string;
  getOrgId: () => string;
  fetch?: (input: Request) => Promise<Response>;
}

/**
 * ApiClient is the openapi-fetch client typed against the v0.1 spec.
 * Every method (`GET`, `POST`, `PATCH`, etc.) is narrowed to operations
 * present in the spec; nonexistent paths fail compilation.
 */
export type ApiClient = ReturnType<typeof createClient<paths>>;

/**
 * Factory for the typed REST API client (D-38).
 *
 * Adds `X-Org-Id` to every request via openapi-fetch Middleware. Routes
 * that bypass org middleware on the Go side (`/healthz`, `/readyz`,
 * `/metrics`, `/openapi.yaml`, `/docs`) are NOT exposed via the typed
 * `paths` map, so this header is harmless on them.
 */
export function createApiClient(config: CreateApiClientConfig): ApiClient {
  const orgIdMiddleware: Middleware = {
    onRequest({ request }) {
      // D-38: getOrgId invoked per request, not at construction time.
      request.headers.set('X-Org-Id', config.getOrgId());
      return request;
    },
  };

  const clientOptions: Parameters<typeof createClient>[0] = {
    baseUrl: config.baseURL,
  };

  if (config.fetch !== undefined) {
    clientOptions.fetch = config.fetch;
  }

  const client = createClient<paths>(clientOptions);
  client.use(orgIdMiddleware);
  return client;
}
