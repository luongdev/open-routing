import { createApiClient } from '../../api/client.js';
import type { ApiClient } from '../../api/client.js';
import {
  MOCK_ORG_ID,
  MOCK_AGENTS,
  MOCK_SKILLS,
  MOCK_QUEUES,
  MOCK_CHANNELS,
  MOCK_ADAPTERS,
  MOCK_BREAK_REASONS,
  MOCK_FLOWS,
  MOCK_AGENT_STATES,
  MOCK_IMPORT_JOB,
} from './playground-mock-data.js';

let _mockIdCounter = 0;
function nextMockId(): string {
  _mockIdCounter++;
  const hex = _mockIdCounter.toString(16).padStart(12, '0');
  return `01919f00-ffff-7000-9000-${hex}`;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function notFound(): Response {
  return jsonResponse({ error: 'not_found', reason: 'mock' }, 404);
}

// Pattern: /v1/orgs/:orgId/<entity>
const ORG_PATH = /^\/v1\/orgs\/[^/]+\//;

function mockFetch(req: Request): Promise<Response> {
  const url = new URL(req.url, 'http://localhost');
  const path = url.pathname;
  const method = req.method.toUpperCase();

  // ---- agents/status (status list must come before /:id)
  if (method === 'GET' && /\/agents\/status$/.test(path)) {
    return Promise.resolve(jsonResponse({ items: MOCK_AGENT_STATES, next_cursor: null, has_more: false }));
  }

  // ---- agent state GET/PATCH
  const agentStateMatch = /\/agents\/([^/]+)\/status$/.exec(path);
  if (agentStateMatch) {
    const aid = agentStateMatch[1];
    const state = MOCK_AGENT_STATES.find(s => s.agent_id === aid);
    if (!state) return Promise.resolve(notFound());
    return Promise.resolve(jsonResponse(state));
  }

  // ---- import job GET
  const importJobGetMatch = /\/imports\/([^/]+)$/.exec(path);
  if (importJobGetMatch && method === 'GET') {
    const jid = importJobGetMatch[1];
    if (jid === MOCK_IMPORT_JOB.id) return Promise.resolve(jsonResponse(MOCK_IMPORT_JOB));
    return Promise.resolve(notFound());
  }

  // ---- bulk import POST
  if (method === 'POST' && /\/imports$/.test(path)) {
    const result = {
      succeeded: MOCK_AGENTS.slice(0, 7).map(a => a.id),
      failed: [
        { row: 2, code: 'emp_9999', reason: 'duplicate_code', message: 'Code already exists' },
        { row: 5, code: 'emp_0003', reason: 'invalid_body',   message: 'Email required' },
        { row: 9, code: '',         reason: 'invalid_body',   message: 'Code required' },
      ],
      idempotent_replay: false,
    };
    return Promise.resolve(jsonResponse(result, 207));
  }

  if (!ORG_PATH.test(path)) return Promise.resolve(notFound());

  // Determine entity from path segment after /orgs/:orgId/
  const afterOrg = path.replace(/^\/v1\/orgs\/[^/]+\//, '');
  const segments = afterOrg.split('/');
  const entity = segments[0];
  const entityId = segments[1];
  if (!entity) return Promise.resolve(notFound());

  const entityMap: Record<string, unknown[]> = {
    agents:        MOCK_AGENTS,
    skills:        MOCK_SKILLS,
    queues:        MOCK_QUEUES,
    channels:      MOCK_CHANNELS,
    adapters:      MOCK_ADAPTERS,
    'break-reasons': MOCK_BREAK_REASONS,
    // Map the FlowSummary fixture onto the api.Flow shape the real or-flow-list
    // reads (status -> enabled; graph/created_at synthesised).
    flows: MOCK_FLOWS.map((f) => ({
      id: f.id,
      org_id: f.org_id,
      code: f.code,
      name: f.name,
      graph: {},
      enabled: f.status !== 'archived',
      version: f.version,
      created_at: f.updated_at,
      updated_at: f.updated_at,
    })),
  };

  const items = entityMap[entity];
  if (!items) return Promise.resolve(notFound());

  // LIST
  if (method === 'GET' && !entityId) {
    const search = (url.searchParams.get('search') ?? '').toLowerCase();
    const includeDisabled = url.searchParams.get('include_disabled') === 'true';
    let filtered = items as Array<{ enabled: boolean; name?: string; code?: string }>;
    if (!includeDisabled) {
      filtered = filtered.filter(i => i.enabled !== false);
    }
    if (search) {
      filtered = filtered.filter(i =>
        (i.name ?? '').toLowerCase().includes(search) ||
        (i.code ?? '').toLowerCase().includes(search)
      );
    }
    return Promise.resolve(jsonResponse({ items: filtered, next_cursor: null, has_more: false }));
  }

  // GET single
  if (method === 'GET' && entityId) {
    const item = (items as Array<{ id: string }>).find(i => i.id === entityId);
    if (!item) return Promise.resolve(notFound());
    return Promise.resolve(jsonResponse(item));
  }

  // POST create — echo back with generated id
  if (method === 'POST') {
    return req.json().then((body: Record<string, unknown>) => {
      const newItem = {
        ...body,
        id: nextMockId(),
        org_id: MOCK_ORG_ID,
        version: 1,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      return jsonResponse(newItem, 201);
    });
  }

  // PATCH update — echo back with version bumped
  if (method === 'PATCH' && entityId) {
    const existing = (items as Array<{ id: string; version: number }>).find(i => i.id === entityId);
    if (!existing) return Promise.resolve(notFound());
    return req.json().then((body: Record<string, unknown>) => {
      const updated = { ...existing, ...body, updated_at: new Date().toISOString(), version: existing.version + 1 };
      return jsonResponse(updated);
    });
  }

  // DELETE
  if (method === 'DELETE' && entityId) {
    const exists = (items as Array<{ id: string }>).some(i => i.id === entityId);
    if (!exists) return Promise.resolve(notFound());
    return Promise.resolve(new Response(null, { status: 204 }));
  }

  return Promise.resolve(notFound());
}

export function createMockApiClient(): ApiClient {
  return createApiClient({
    baseURL: '',
    getOrgId: () => MOCK_ORG_ID,
    fetch: mockFetch,
  });
}
