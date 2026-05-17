/**
 * Tests for createImporter() — ADMIN-04 compliance gate.
 *
 * createImporter() is the SOLE typed gateway for POST /v1/orgs/{orgId}/catalog/import.
 * No component or page may call fetch() directly for this endpoint.
 *
 * All 7 tests use a spy fetchImpl injected via deps so they assert on the
 * exact Request shape sent to the HTTP layer.
 */

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { createImporter } from './import';

const BULK_IMPORT_RESULT = {
  succeeded: ['01935b00-0000-7000-8000-000000000001'],
  failed: [],
  idempotent_replay: false,
};

function makeFetch(status: number, body: unknown = BULK_IMPORT_RESULT) {
  return vi.fn(async (_url: string | URL | Request, _init?: RequestInit) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    })
  );
}

describe('createImporter', () => {
  const BASE_URL = 'https://api.test';
  const ORG_ID = 'test-org';

  let mockFetch: ReturnType<typeof makeFetch>;

  beforeEach(() => {
    mockFetch = makeFetch(200);
  });

  // Test 1 — X-Org-Id header injected from deps.getOrgId()
  it('sets X-Org-Id header from deps.getOrgId', async () => {
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: mockFetch,
    });
    await importer({
      entity: 'agents',
      body: '{}',
      contentType: 'application/json',
    });
    const [_url, init] = mockFetch.mock.calls[0]!;
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Org-Id')).toBe(ORG_ID);
  });

  // Test 2a — Content-Type: text/csv for CSV args
  it('sets Content-Type: text/csv for CSV imports', async () => {
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: mockFetch,
    });
    await importer({
      entity: 'agents',
      body: 'code,name,email\nalice,Alice,alice@example.com',
      contentType: 'text/csv',
    });
    const [_url, init] = mockFetch.mock.calls[0]!;
    const headers = new Headers(init?.headers);
    expect(headers.get('Content-Type')).toBe('text/csv');
  });

  // Test 2b — Content-Type: application/json for JSON args
  it('sets Content-Type: application/json for JSON imports', async () => {
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: mockFetch,
    });
    await importer({
      entity: 'skills',
      body: '[{"code":"skill_voice","name":"Voice"}]',
      contentType: 'application/json',
    });
    const [_url, init] = mockFetch.mock.calls[0]!;
    const headers = new Headers(init?.headers);
    expect(headers.get('Content-Type')).toBe('application/json');
  });

  // Test 3 — schema_version query param: appended for CSV, omitted for JSON
  it('appends schema_version for CSV; omits for JSON', async () => {
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: mockFetch,
    });

    // CSV + schemaVersion
    await importer({
      entity: 'agents',
      body: 'code,name,email\nalice,Alice,alice@example.com',
      contentType: 'text/csv',
      schemaVersion: 'v0.1',
    });
    const [csvUrl] = mockFetch.mock.calls[0]!;
    expect(String(csvUrl)).toContain('schema_version=v0.1');

    // JSON — no schema_version
    await importer({
      entity: 'agents',
      body: '[{}]',
      contentType: 'application/json',
    });
    const [jsonUrl] = mockFetch.mock.calls[1]!;
    expect(String(jsonUrl)).not.toContain('schema_version');
  });

  // Test 4 — idempotencyKey header: set when present, absent when not
  it('sets Idempotency-Key header when provided; omits when absent', async () => {
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: mockFetch,
    });

    // With idempotency key
    await importer({
      entity: 'agents',
      body: '{}',
      contentType: 'application/json',
      idempotencyKey: 'key-123',
    });
    const [_url1, init1] = mockFetch.mock.calls[0]!;
    expect(new Headers(init1?.headers).get('Idempotency-Key')).toBe('key-123');

    // Without idempotency key
    await importer({
      entity: 'agents',
      body: '{}',
      contentType: 'application/json',
    });
    const [_url2, init2] = mockFetch.mock.calls[1]!;
    expect(new Headers(init2?.headers).get('Idempotency-Key')).toBeNull();
  });

  // Test 5 — response 200 and 207 → returns BulkImportResult
  it('returns BulkImportResult for 200 success', async () => {
    const fetch200 = makeFetch(200, BULK_IMPORT_RESULT);
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: fetch200,
    });
    const result = await importer({
      entity: 'agents',
      body: '{}',
      contentType: 'application/json',
    });
    expect(result.succeeded).toEqual(['01935b00-0000-7000-8000-000000000001']);
    expect(result.failed).toEqual([]);
  });

  it('returns BulkImportResult for 207 partial success', async () => {
    const partialResult = {
      succeeded: ['01935b00-0000-7000-8000-000000000001'],
      failed: [{ row: 2, error: 'import_failed', reason: 'duplicate code' }],
    };
    const fetch207 = makeFetch(207, partialResult);
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: fetch207,
    });
    const result = await importer({
      entity: 'agents',
      body: '{}',
      contentType: 'application/json',
    });
    expect(result.succeeded).toHaveLength(1);
    expect(result.failed).toHaveLength(1);
  });

  // Test 6 — response 413 → throws with status 413
  it('throws on 413 response', async () => {
    const fetch413 = makeFetch(413, { error: 'payload_too_large', reason: 'File exceeds 50MB limit' });
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: fetch413,
    });
    await expect(
      importer({ entity: 'agents', body: '{}', contentType: 'application/json' })
    ).rejects.toMatchObject({ status: 413 });
  });

  // Test 7 — response 400 → throws with status 400
  it('throws on 400 response', async () => {
    const fetch400 = makeFetch(400, { error: 'invalid_body', reason: 'malformed JSON' });
    const importer = createImporter({
      baseURL: BASE_URL,
      getOrgId: () => ORG_ID,
      fetchImpl: fetch400,
    });
    await expect(
      importer({ entity: 'agents', body: 'bad', contentType: 'application/json' })
    ).rejects.toMatchObject({ status: 400 });
  });
});
