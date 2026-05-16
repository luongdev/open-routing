// Phase 2 D-39: typed error helpers for the closed ErrorCode enum.
//
// ApiError aliases the spec-generated shape so it stays in sync with
// openapi.yaml automatically (drift gate enforces).

import type { components } from './generated';

/**
 * ApiError mirrors the on-the-wire shape of every 4xx/5xx response body
 * across the v0.1 spec. The shape matches the Go-side `middleware.errorBody`
 * extended for `request_id` per D-35.
 */
export type ApiError = components['schemas']['ErrorResponse'];

/**
 * ErrorCodes is the closed v0.1 enum from D-36. Encoded as `as const`
 * so consumers get autocomplete AND compile-time exhaustiveness when
 * switching over error codes.
 *
 * Mirror of openapi.yaml `components.schemas.ErrorCode`. Adding a new
 * code requires changing both this record and the spec — drift gate
 * (Plan 06) keeps them aligned.
 */
export const ErrorCodes = {
  INVALID_BODY: 'invalid_body',
  INVALID_ID: 'invalid_id',
  NOT_FOUND: 'not_found',
  INTERNAL: 'internal',
  VERSION_CONFLICT: 'version_conflict',
  CROSS_ORG: 'cross_org',
  INVALID_ORG_ID: 'invalid_org_id',
  INVALID_TRANSITION: 'invalid_transition',
  IMPORT_FAILED: 'import_failed',
  RATE_LIMITED: 'rate_limited',
} as const;

/**
 * ErrorCode is the union of valid error code strings. Use it in switch
 * statements over `ApiError['error']` for exhaustiveness checking.
 */
export type ErrorCode = (typeof ErrorCodes)[keyof typeof ErrorCodes];

/**
 * Type guard for ApiError. Use after `catch (e)` blocks where openapi-fetch
 * surfaces the response body as an unknown.
 *
 * Accepts the minimal shape — `error` is a string. Does NOT validate that
 * `error` is one of the ten ErrorCode values; that narrows further with a
 * switch over ErrorCodes if needed.
 */
export function isApiError(value: unknown): value is ApiError {
  return (
    typeof value === 'object' &&
    value !== null &&
    'error' in value &&
    typeof (value as { error: unknown }).error === 'string' &&
    'reason' in value &&
    typeof (value as { reason: unknown }).reason === 'string'
  );
}

/**
 * Parse an HTTP Response as the canonical error body.
 *
 * Strict parser per Claude's Discretion bullet 4 in CONTEXT.md ("strict
 * parse; network errors are caller's concern"). Returns null when:
 *   - The body is not parseable as JSON.
 *   - The parsed value does not match the {error, reason} shape.
 *
 * Caller is responsible for catching network / abort errors before
 * passing the Response. parseApiError NEVER throws.
 */
export async function parseApiError(response: Response): Promise<ApiError | null> {
  try {
    const body = (await response.json()) as unknown;
    return isApiError(body) ? body : null;
  } catch {
    return null;
  }
}
