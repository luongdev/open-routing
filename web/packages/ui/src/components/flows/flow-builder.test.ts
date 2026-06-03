// Tests for <or-flow-builder> (v0.2 Layer 2). The builder loads a draft graph
// from GET /flows/{id}, persists it via PATCH (POST in create mode), and wires
// Validate/Publish to the real endpoints — which are 501 stubs until Layer 3,
// surfaced as an action toast rather than a faked success.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './flow-builder.js';

const NODES = [
  { id: 'n1', kind: 'trigger', label: 'On Inbound', description: 'voice', x: 80, y: 40 },
  { id: 'n2', kind: 'end', label: 'Hang up', description: 'done', x: 80, y: 208 },
];
const EDGES = [{ id: 'e1', from: 'n1', to: 'n2' }];

const MOCK_FLOW = {
  id: '01935c00-0000-7000-8000-0000000000f1',
  org_id: 'test-org',
  code: 'flow_inbound',
  name: 'Inbound Voice',
  graph: { nodes: NODES, edges: EDGES },
  enabled: true,
  version: 3,
  created_at: '2026-05-01T00:00:00Z',
  updated_at: '2026-05-17T00:00:00Z',
};

describe('OrFlowBuilder', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-flow-builder');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function settle() {
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;
  }

  it('loads the draft graph from GET /flows/{id} and hydrates nodes/edges', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: mockGet };
    await settle();

    expect(mockGet).toHaveBeenCalled();
    const [path, opts] = mockGet.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows/{id}');
    expect(opts?.params?.path).toEqual({ org_id: 'test-org', id: MOCK_FLOW.id });
    expect((el as any)._nodes).toHaveLength(2);
    expect((el as any)._edges).toHaveLength(1);
    // Deep-copied, not aliased to the response.
    expect((el as any)._nodes).not.toBe(MOCK_FLOW.graph.nodes);
    expect(el.shadowRoot!.textContent).toContain('Inbound Voice');
  });

  it('Save draft PATCHes the graph with the optimistic version', async () => {
    const mockPatch = vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, version: 4 }, error: null });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }),
      PATCH: mockPatch,
    };
    await settle();
    await (el as any)._saveDraft();

    expect(mockPatch).toHaveBeenCalled();
    const [path, opts] = mockPatch.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows/{id}');
    expect(opts?.params?.path?.id).toBe(MOCK_FLOW.id);
    expect(opts?.body?.version).toBe(3);
    expect(opts?.body?.graph?.nodes).toHaveLength(2);
    expect(opts?.body?.graph?.edges).toHaveLength(1);
    expect((el as any)._actionToast).toContain('Saved draft');
  });

  it('surfaces a 409 version conflict on save instead of swallowing it', async () => {
    const mockPatch = vi.fn().mockResolvedValue({ data: null, error: { status: 409 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }),
      PATCH: mockPatch,
    };
    await settle();
    await (el as any)._saveDraft();
    expect((el as any)._actionToast).toContain('conflict');
    expect((el as any)._actionTone).toBe('error');
  });

  it('Validate calls POST /validate and reports the 501 stub honestly', async () => {
    const mockPost = vi.fn().mockResolvedValue({ data: null, error: { status: 501 }, response: { status: 501 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }),
      POST: mockPost,
    };
    await settle();
    await (el as any)._runStubAction('validate', 'Validate');

    expect(mockPost).toHaveBeenCalled();
    expect((mockPost.mock.calls[0] as [string, any])[0]).toBe('/v1/orgs/{org_id}/flows/{id}/validate');
    expect((el as any)._actionToast).toContain('Layer 3');
    expect((el as any)._actionTone).toBe('warn');
  });

  it('create mode (no flowId) POSTs a new flow and navigates to its detail', async () => {
    const created = { ...MOCK_FLOW, id: '01935c00-0000-7000-8000-0000000000ff' };
    const mockPost = vi.fn().mockResolvedValue({ data: created, error: null });
    (el as any).orgId = 'test-org';
    (el as any).flowId = '';
    (el as any).client = { POST: mockPost };
    await settle();
    (el as any)._codeDraft = 'flow_new';
    (el as any)._nameDraft = 'Brand New Flow';

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));
    await (el as any)._saveDraft();

    expect(mockPost).toHaveBeenCalled();
    const [path, opts] = mockPost.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows');
    expect(opts?.body?.code).toBe('flow_new');
    expect(opts?.body?.name).toBe('Brand New Flow');
    expect(opts?.body?.graph).toBeDefined();
    expect(events[0]?.detail?.path).toBe(`/orgs/test-org/flows/${created.id}`);
  });

  it('create mode requires code and name before POSTing', async () => {
    const mockPost = vi.fn();
    (el as any).orgId = 'test-org';
    (el as any).flowId = '';
    (el as any).client = { POST: mockPost };
    await settle();
    await (el as any)._saveDraft();
    expect(mockPost).not.toHaveBeenCalled();
    expect((el as any)._actionToast).toContain('required');
  });

  it('renders the load error state with Retry when GET fails', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: null, error: { reason: 'boom' } }) };
    await settle();
    const status = el.shadowRoot!.querySelector('.builder-status--error');
    expect(status).toBeTruthy();
    expect(status?.textContent).toContain('Failed to load flow');
    expect(status?.textContent).toContain('boom');
  });
});
