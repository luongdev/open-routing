import { describe, expect, it } from 'vitest';
import { isApiError, parseApiError, ErrorCodes, type ApiError } from './errors';

describe('isApiError', () => {
  it('accepts a valid {error, reason} shape', () => {
    const v: unknown = { error: 'not_found', reason: 'no_such_agent' };
    expect(isApiError(v)).toBe(true);
    if (isApiError(v)) {
      const _narrowed: ApiError = v; // typechecks: narrowing works
      void _narrowed;
    }
  });
  it('rejects non-object values', () => {
    expect(isApiError(null)).toBe(false);
    expect(isApiError(undefined)).toBe(false);
    expect(isApiError(42)).toBe(false);
    expect(isApiError('string')).toBe(false);
  });
  it('rejects objects missing required fields', () => {
    expect(isApiError({ foo: 'bar' })).toBe(false);
    expect(isApiError({ error: 42 })).toBe(false);
    expect(isApiError({ error: 'not_found' })).toBe(false); // missing reason
  });
});

describe('parseApiError', () => {
  it('parses a JSON 4xx body', async () => {
    const body = { error: 'not_found', reason: 'x', request_id: '01935b00-0000-7000-8000-000000000000' };
    const resp = new Response(JSON.stringify(body), { status: 404, headers: { 'Content-Type': 'application/json' } });
    const parsed = await parseApiError(resp);
    expect(parsed).not.toBeNull();
    expect(parsed!.error).toBe('not_found');
    expect(parsed!.reason).toBe('x');
    expect(parsed!.request_id).toBe('01935b00-0000-7000-8000-000000000000');
  });
  it('returns null when body is not JSON', async () => {
    const resp = new Response('not json', { status: 500, headers: { 'Content-Type': 'text/plain' } });
    const parsed = await parseApiError(resp);
    expect(parsed).toBeNull();
  });
  it('returns null when body does not match the ApiError shape', async () => {
    const resp = new Response(JSON.stringify({ foo: 'bar' }), { status: 500, headers: { 'Content-Type': 'application/json' } });
    const parsed = await parseApiError(resp);
    expect(parsed).toBeNull();
  });
});

describe('ErrorCodes', () => {
  it('has the 10 D-36 codes plus 5 Phase 04.1 codes (15 total, D6-24)', () => {
    const expected = new Set([
      // Phase 2 D-36 original codes
      'invalid_body', 'invalid_id', 'not_found', 'internal', 'version_conflict',
      'cross_org', 'invalid_org_id', 'invalid_transition', 'import_failed', 'rate_limited',
      // Phase 04.1 + Phase 5 additions (D6-24)
      'duplicate_code', 'duplicate_external_id', 'immutable_field', 'invalid_reference', 'invalid_value',
    ]);
    const actual = new Set(Object.values(ErrorCodes));
    expect(actual.size).toBe(15);
    expect(actual).toEqual(expected);
  });
});
