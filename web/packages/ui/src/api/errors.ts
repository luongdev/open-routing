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
  // Phase 04.1 + Phase 5 error codes (D6-24)
  DUPLICATE_CODE: 'duplicate_code',
  DUPLICATE_EXTERNAL_ID: 'duplicate_external_id',
  IMMUTABLE_FIELD: 'immutable_field',
  INVALID_REFERENCE: 'invalid_reference',
  INVALID_VALUE: 'invalid_value',
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
 * ERROR_I18N_KEYS maps each ErrorCode to its i18n message key for use with
 * @lit/localize msg() calls in components (D6-24).
 *
 * Components look up the key by: ERROR_I18N_KEYS[err.error] ?? 'errors.internal'
 * XLIFF files in xliff/ contain the translations for each key.
 */
export const ERROR_I18N_KEYS: Record<ErrorCode, string> = {
  invalid_body: 'errors.invalid_body',
  invalid_id: 'errors.invalid_id',
  not_found: 'errors.not_found',
  internal: 'errors.internal',
  version_conflict: 'errors.version_conflict',
  cross_org: 'errors.cross_org',
  invalid_org_id: 'errors.invalid_org_id',
  invalid_transition: 'errors.invalid_transition',
  import_failed: 'errors.import_failed',
  rate_limited: 'errors.rate_limited',
  duplicate_code: 'errors.duplicate_code',
  duplicate_external_id: 'errors.duplicate_external_id',
  immutable_field: 'errors.immutable_field',
  invalid_reference: 'errors.invalid_reference',
  invalid_value: 'errors.invalid_value',
};

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
