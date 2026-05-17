/**
 * Typed bulk catalog import wrapper — ADMIN-04 compliance gate.
 *
 * This file is the SOLE gateway for POST /v1/orgs/{orgId}/catalog/import.
 * Raw fetch() calls in component or page code for this endpoint are FORBIDDEN.
 *
 * Per ADMIN-04: "Generated TS API client from packages/ui is the ONLY API
 * access layer; typecheck fails on raw fetch". createImporter() is the typed
 * single entry point for CSV/JSON catalog imports.
 *
 * Usage:
 *   const importCatalog = createImporter({ baseURL, getOrgId, fetchImpl });
 *   const result = await importCatalog({ entity: 'agents', body: csvText, contentType: 'text/csv', schemaVersion: 'v0.1' });
 */

import type { components } from './generated';

// ── Type exports ─────────────────────────────────────────────────────────────

/**
 * BulkImportResult mirrors the on-the-wire shape of the 200/207 response from
 * POST /v1/orgs/{orgId}/catalog/import. Sourced from the generated OpenAPI types.
 */
export type BulkImportResult = components['schemas']['BulkImportResult'];

/**
 * CatalogEntity is the set of entity types that support bulk import.
 */
export type CatalogEntity =
  | 'agents'
  | 'skills'
  | 'queues'
  | 'channels'
  | 'adapters'
  | 'break_reasons';

/**
 * ImportContentType enumerates the supported import body content types.
 */
export type ImportContentType = 'application/json' | 'text/csv';

/**
 * ImportCatalogArgs describes the arguments for a single import call.
 */
export interface ImportCatalogArgs {
  /** Which entity type to import. Sent as the `entity` query parameter. */
  entity: CatalogEntity;
  /** Raw request body — either JSON string or raw CSV text. */
  body: string;
  /** MIME type of the body. Determines Content-Type header and schema_version logic. */
  contentType: ImportContentType;
  /**
   * CSV schema version — appended as `schema_version` query parameter for CSV
   * imports only (per D5-04). Ignored for JSON. Required when contentType is
   * 'text/csv'.
   */
  schemaVersion?: 'v0.1';
  /**
   * Optional idempotency key (UUIDv7 recommended, per Phase 5 D5-13).
   * When present, sets the `Idempotency-Key` request header. The server
   * persists the result; repeated requests with the same key return the
   * original result without reprocessing.
   */
  idempotencyKey?: string;
}

/**
 * ImportCatalogDeps describes the external dependencies injected into the importer.
 * Injection enables testing without network I/O.
 */
export interface ImportCatalogDeps {
  /** API server origin (e.g. "" for same-origin, or "https://api.example.com"). */
  baseURL: string;
  /**
   * Returns the current org ID. Invoked PER REQUEST (not at construction time)
   * so it stays current if the org changes — same contract as createApiClient.
   */
  getOrgId: () => string;
  /**
   * Optional fetch implementation. Defaults to globalThis.fetch.
   * Pass a spy/stub in tests.
   */
  fetchImpl?: typeof fetch;
}

// ── Import error class ────────────────────────────────────────────────────────

/**
 * ImportError is thrown by importCatalog() when the server returns a 4xx
 * response (excluding 207 partial success, which is returned as BulkImportResult).
 *
 * Note: ApiError in errors.ts is a type alias (not a class) for
 * components['schemas']['ErrorResponse']. ImportError wraps the response status
 * plus the parsed error body so callers can inspect both.
 */
export class ImportError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown) {
    super(`Import request failed with status ${status}`);
    this.name = 'ImportError';
    this.status = status;
    this.body = body;
  }
}

// ── Factory ───────────────────────────────────────────────────────────────────

/**
 * createImporter returns a typed importCatalog function bound to the given deps.
 *
 * The returned function encapsulates all HTTP concerns — URL construction,
 * header injection (X-Org-Id, Content-Type, Accept, Idempotency-Key),
 * schema_version query parameter, response parsing, and error throwing.
 *
 * @example
 * ```ts
 * const importCatalog = createImporter({ baseURL: '', getOrgId: () => orgId });
 * try {
 *   const result = await importCatalog({
 *     entity: 'agents',
 *     body: csvText,
 *     contentType: 'text/csv',
 *     schemaVersion: 'v0.1',
 *   });
 *   // result is BulkImportResult (200 success or 207 partial)
 * } catch (err) {
 *   if (err instanceof ImportError && err.status === 413) { /* too large *\/ }
 * }
 * ```
 */
export function createImporter(deps: ImportCatalogDeps) {
  const f = deps.fetchImpl ?? globalThis.fetch;

  return async function importCatalog(args: ImportCatalogArgs): Promise<BulkImportResult> {
    const orgId = deps.getOrgId();

    // Build query string: always includes entity; CSV imports add schema_version
    const params = new URLSearchParams({ entity: args.entity });
    if (args.contentType === 'text/csv' && args.schemaVersion) {
      params.set('schema_version', args.schemaVersion);
    }

    // Build headers
    const headers: Record<string, string> = {
      'Content-Type': args.contentType,
      'X-Org-Id': orgId,
      Accept: 'application/json',
    };
    if (args.idempotencyKey) {
      headers['Idempotency-Key'] = args.idempotencyKey;
    }

    const url = `${deps.baseURL}/v1/orgs/${orgId}/catalog/import?${params.toString()}`;
    const resp = await f(url, { method: 'POST', headers, body: args.body });

    // Parse body regardless of status (used in both success and error paths)
    const data = await resp.json().catch(() => ({}));

    // 207 is a partial success — treat the same as 200 (caller inspects failed[])
    if (resp.status >= 400 && resp.status !== 207) {
      throw new ImportError(resp.status, data);
    }

    return data as BulkImportResult;
  };
}
