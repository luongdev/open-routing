import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';

// Path constant: W-4 — agents is a stable v0.1 path (CAT-01..CAT-11).
// Phase 3 will delete /v1/orgs/{org_id}/_scaffold per D-46, so the
// _scaffold path is unsafe as a long-lived test fixture.
const STABLE_TEST_PATH = '/v1/orgs/{org_id}/agents' as const;
const TEST_ORG_ID = '01935b00-0000-7000-8000-000000000099';

// Fake AgentList envelope — matches the Pagination + Agent[] shape from
// openapi.yaml. Body content does not matter for client-layer tests;
// only the Request the client constructs is under test.
const fakeAgentList = JSON.stringify({ items: [], has_more: false, next_cursor: null });

describe('createApiClient', () => {
  it('invokes getOrgId per request, not at construction', async () => {
    let calls = 0;
    const getOrgId = vi.fn(() => {
      calls += 1;
      return `01935b00-0000-7000-8000-00000000000${calls}`;
    });
    const captured: Request[] = [];
    const stubFetch = vi.fn(async (input: Request) => {
      captured.push(input);
      return new Response(fakeAgentList, {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    const client = createApiClient({
      baseURL: 'https://test.local',
      getOrgId,
      fetch: stubFetch,
    });
    await client.GET(STABLE_TEST_PATH, { params: { path: { org_id: TEST_ORG_ID } } });
    await client.GET(STABLE_TEST_PATH, { params: { path: { org_id: TEST_ORG_ID } } });
    expect(getOrgId).toHaveBeenCalledTimes(2);
    expect(captured).toHaveLength(2);
    expect(captured[0]!.headers.get('X-Org-Id')).toBe('01935b00-0000-7000-8000-000000000001');
    expect(captured[1]!.headers.get('X-Org-Id')).toBe('01935b00-0000-7000-8000-000000000002');
  });

  it('uses baseURL on every request', async () => {
    const captured: Request[] = [];
    const stubFetch = vi.fn(async (input: Request) => {
      captured.push(input);
      return new Response(fakeAgentList, { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    const client = createApiClient({
      baseURL: 'https://api.test',
      getOrgId: () => '01935b00-0000-7000-8000-000000000001',
      fetch: stubFetch,
    });
    await client.GET(STABLE_TEST_PATH, { params: { path: { org_id: TEST_ORG_ID } } });
    expect(captured[0]!.url).toMatch(/^https:\/\/api\.test\/v1\//);
  });

  it('invokes the custom fetch override, not global fetch', async () => {
    const stubFetch = vi.fn(async () => new Response(fakeAgentList, { status: 200, headers: { 'Content-Type': 'application/json' } }));
    const originalGlobalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn(() => { throw new Error('global fetch should NOT be called'); }) as typeof fetch;
    try {
      const client = createApiClient({
        baseURL: 'http://localhost',
        getOrgId: () => '01935b00-0000-7000-8000-000000000001',
        fetch: stubFetch,
      });
      await client.GET(STABLE_TEST_PATH, { params: { path: { org_id: TEST_ORG_ID } } });
      expect(stubFetch).toHaveBeenCalledTimes(1);
    } finally {
      globalThis.fetch = originalGlobalFetch;
    }
  });
});
