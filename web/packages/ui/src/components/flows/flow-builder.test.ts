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
    const mockPatch = vi.fn().mockResolvedValue({ data: null, error: { reason: 'version_conflict' }, response: { status: 409 } });
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

  it('Validate POSTs /validate and surfaces the real issues', async () => {
    const result = { valid: false, issues: [{ code: 'missing_catalog_reference', message: 'queue "x" not found', node_id: 'n2', field: 'queue' }] };
    const mockPost = vi.fn().mockResolvedValue({ data: result, error: null, response: { status: 200 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }),
      POST: mockPost,
    };
    await settle();
    await (el as any)._validateFlow();

    expect((mockPost.mock.calls[0] as [string, any])[0]).toBe('/v1/orgs/{org_id}/flows/{id}/validate');
    expect((el as any)._validation).toEqual(result);
    expect((el as any)._actionToast).toContain('1 issue');
    // The offending node is highlighted on the canvas.
    expect([...((el as any)._issueNodeIds as Set<string>)]).toEqual(['n2']);
  });

  it('Validate reports a clean graph', async () => {
    const mockPost = vi.fn().mockResolvedValue({ data: { valid: true, issues: [] }, error: null, response: { status: 200 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }), POST: mockPost };
    await settle();
    await (el as any)._validateFlow();
    expect((el as any)._validation.valid).toBe(true);
    expect((el as any)._actionToast).toContain('Valid');
  });

  it('Publish POSTs /publish with the binding + draft version and reports success', async () => {
    const mockPost = vi.fn().mockResolvedValue({
      data: { version: { version_number: 4 }, binding: { active: true } },
      error: null,
      response: { status: 201 },
    });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }), POST: mockPost };
    await settle();
    await (el as any)._publishFlow();

    const [path, opts] = mockPost.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows/{id}/publish');
    expect(opts?.body?.version).toBe(3);
    expect(opts?.body?.channel).toBe('voice');
    expect(opts?.body?.entry_code).toBe('main');
    expect((el as any)._actionToast).toContain('Published v4');
  });

  it('Publish 422 surfaces validation issues instead of faking success', async () => {
    const vr = { valid: false, issues: [{ code: 'no_trigger', message: 'graph has no trigger' }] };
    const mockPost = vi.fn().mockResolvedValue({ data: null, error: vr, response: { status: 422 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }), POST: mockPost };
    await settle();
    await (el as any)._publishFlow();
    expect((el as any)._validation).toEqual(vr);
    expect((el as any)._actionToast).toContain('blocked');
  });

  it('Publish 409 surfaces an optimistic conflict', async () => {
    const mockPost = vi.fn().mockResolvedValue({ data: null, error: { reason: 'draft_version_mismatch' }, response: { status: 409 } });
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }), POST: mockPost };
    await settle();
    await (el as any)._publishFlow();
    expect((el as any)._actionToast).toContain('conflict');
    expect((el as any)._actionTone).toBe('error');
  });

  it('tolerates a malformed graph (non-array nodes/edges) without crashing', async () => {
    const bad = { ...MOCK_FLOW, graph: { nodes: {}, edges: 'oops' } };
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: bad, error: null }) };
    await settle();
    expect((el as any)._nodes).toEqual([]);
    expect((el as any)._edges).toEqual([]);
    // Loaded fine (not the error state) and the builder still renders its header.
    expect(el.shadowRoot!.querySelector('.builder-status--error')).toBeNull();
    expect(el.shadowRoot!.textContent).toContain('Inbound Voice');
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

  it('palette search filters items and hides non-matching groups', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();

    const labelsBefore = [...el.shadowRoot!.querySelectorAll('.palette-item-label')].length;
    expect(labelsBefore).toBeGreaterThan(5);

    (el as any)._paletteQuery = 'dtmf';
    await (el as any).updateComplete;

    const labels = [...el.shadowRoot!.querySelectorAll('.palette-item-label')].map((n) => n.textContent);
    expect(labels).toEqual(['Get DTMF']);
    // Only the matching group header survives.
    expect(el.shadowRoot!.querySelectorAll('.palette-group-label')).toHaveLength(1);
  });

  it('palette search shows an empty state when nothing matches', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();

    (el as any)._paletteQuery = 'zzz-nope';
    await (el as any).updateComplete;
    expect(el.shadowRoot!.querySelector('.palette-empty')).toBeTruthy();
    expect(el.shadowRoot!.querySelectorAll('.palette-item')).toHaveLength(0);
  });

  it('collapsing a group hides its items but keeps the header', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();

    const before = [...el.shadowRoot!.querySelectorAll('.palette-item-label')].map((n) => n.textContent);
    expect(before).toContain('If / Else');

    (el as any)._toggleGroup('Control flow');
    await (el as any).updateComplete;
    const after = [...el.shadowRoot!.querySelectorAll('.palette-item-label')].map((n) => n.textContent);
    expect(after).not.toContain('If / Else');
    // Trigger (a different group) is untouched.
    expect(after).toContain('Trigger');
    // The collapsed header is still present and shows the collapsed chevron.
    const headers = [...el.shadowRoot!.querySelectorAll('.palette-group-label')];
    const collapsed = headers.find((h) => h.textContent?.includes('Control flow'));
    expect(collapsed?.getAttribute('aria-expanded')).toBe('false');
  });

  it('an active search force-expands collapsed groups so matches are visible', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();

    (el as any)._toggleGroup('Control flow');
    (el as any)._paletteQuery = 'branch';
    await (el as any).updateComplete;

    const labels = [...el.shadowRoot!.querySelectorAll('.palette-item-label')].map((n) => n.textContent);
    expect(labels).toContain('If / Else');
  });

  it('footer reflects the real graph: node/edge counts, not a hardcoded warning', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();

    const strip = el.shadowRoot!.querySelector('.canvas-strip')!;
    expect(strip.textContent).toContain('2 nodes');
    expect(strip.textContent).toContain('1 edge');
    // The leftover mock warning must be gone.
    expect(strip.textContent).not.toContain('Notify CRM');
  });

  it('footer shows an empty hint when the graph has no nodes', async () => {
    const empty = { ...MOCK_FLOW, graph: { nodes: [], edges: [] } };
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: empty, error: null }) };
    await settle();

    const strip = el.shadowRoot!.querySelector('.canvas-strip')!;
    expect(strip.textContent).toContain('Empty');
    expect(strip.textContent).not.toContain('errors');
  });

  it('dropping a palette node creates it on the canvas', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes: [], edges: [] } }, error: null }) };
    await settle();
    expect((el as any)._nodes).toHaveLength(0);

    const drop = {
      preventDefault() {},
      clientX: 300,
      clientY: 200,
      dataTransfer: { types: ['application/x-or-node'], getData: () => 'log' },
    };
    (el as any)._onCanvasDrop(drop);
    await (el as any).updateComplete;

    expect((el as any)._nodes).toHaveLength(1);
    expect((el as any)._nodes[0].kind).toBe('log');
    expect((el as any)._selectedNodeId).toBe((el as any)._nodes[0].id);
    expect((el as any)._actionToast).toContain('Added');
  });

  it('_serializeGraph maps kind->type and params->config for the backend', () => {
    (el as any)._nodes = [{ id: 'n1', kind: 'route_queue', label: 'Q', description: '', x: 10, y: 20, params: { queue: 'queue_vip' } }];
    (el as any)._edges = [{ id: 'e1', from: 'n1', to: 'n2', from_port: 'true' }];
    const g = (el as any)._serializeGraph();
    expect(g.nodes[0].type).toBe('route_queue');
    expect(g.nodes[0].config).toEqual({ queue: 'queue_vip' });
    expect(g.edges[0].label).toBe('true');
  });

  it('dragging from a port to a node creates an edge with from_port=label', async () => {
    const nodes = [
      { id: 'n1', kind: 'if_else', label: 'If', description: '', x: 0, y: 0, params: { expr: 'x' }, outputs: [{ id: 'true', label: 'true', kind: 'branch' }, { id: 'false', label: 'false', kind: 'default' }] },
      { id: 'n2', kind: 'end', label: 'End', description: '', x: 0, y: 200 },
    ];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges: [] } }, error: null }) };
    await settle();

    (el as any)._connectEdge('n1', 'true', 'branch', 'n2');
    await (el as any).updateComplete;
    const edges = (el as any)._edges;
    expect(edges).toHaveLength(1);
    expect(edges[0]).toMatchObject({ from: 'n1', to: 'n2', from_port: 'true', label: 'true' });
    expect((el as any)._validation).toBeNull();
  });

  it('connecting a linear node replaces its single outgoing edge', async () => {
    const nodes = [
      { id: 'a', kind: 'log', label: 'A', description: '', x: 0, y: 0 },
      { id: 'b', kind: 'end', label: 'B', description: '', x: 0, y: 100 },
      { id: 'c', kind: 'end', label: 'C', description: '', x: 0, y: 200 },
    ];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges: [] } }, error: null }) };
    await settle();
    (el as any)._connectEdge('a', 'done', 'success', 'b');
    (el as any)._connectEdge('a', 'done', 'success', 'c');
    await (el as any).updateComplete;
    expect((el as any)._edges).toHaveLength(1);
    expect((el as any)._edges[0].to).toBe('c');
  });

  it('editing config mutates node.params and clears validation', async () => {
    const nodes = [{ id: 'q', kind: 'route_queue', label: 'Q', description: '', x: 0, y: 0, params: {} }];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges: [] } }, error: null }) };
    await settle();
    (el as any)._validation = { valid: false, issues: [] };
    (el as any)._updateNodeParam('q', 'queue', 'queue_vip');
    expect((el as any)._nodes[0].params.queue).toBe('queue_vip');
    expect((el as any)._validation).toBeNull();
  });

  it('editing switch_case cases regenerates outputs and prunes stale edges', async () => {
    const nodes = [
      { id: 's', kind: 'switch_case', label: 'S', description: '', x: 0, y: 0, params: { cases: ['a', 'b'] }, outputs: [{ id: 'a', label: 'a', kind: 'branch' }, { id: 'b', label: 'b', kind: 'branch' }, { id: 'default', label: 'default', kind: 'default' }] },
      { id: 'x', kind: 'end', label: 'X', description: '', x: 0, y: 200 },
    ];
    const edges = [{ id: 'e1', from: 's', to: 'x', from_port: 'b', label: 'b' }];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges } }, error: null }) };
    await settle();
    // Remove case 'b' → its edge must be pruned and outputs regenerated.
    (el as any)._updateNodeParam('s', 'cases', ['a']);
    await (el as any).updateComplete;
    const sNode = (el as any)._nodes.find((n: any) => n.id === 's');
    expect(sNode.outputs.map((o: any) => o.id)).toEqual(['a', 'default']);
    expect((el as any)._edges).toHaveLength(0); // edge on removed port 'b' pruned
  });

  it('zoom-around-cursor keeps the world point under the cursor fixed', async () => {
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: MOCK_FLOW, error: null }) };
    await settle();
    (el as any)._zoom = 1; (el as any)._panX = 0; (el as any)._panY = 0;
    // world point under cursor (100,100) at zoom 1 = (100,100). After zoom it must still map there.
    (el as any)._zoomAround(100, 100, 2);
    expect((el as any)._zoom).toBe(2);
    // screen 100 = panX + world*zoom → world = (100 - panX)/zoom
    const world = (100 - (el as any)._panX) / (el as any)._zoom;
    expect(world).toBeCloseTo(100, 5);
  });

  it('select an edge then Delete removes it; typing in an input does not', async () => {
    const nodes = [
      { id: 'a', kind: 'log', label: 'A', description: '', x: 0, y: 0 },
      { id: 'b', kind: 'end', label: 'B', description: '', x: 0, y: 100 },
    ];
    const edges = [{ id: 'e1', from: 'a', to: 'b', from_port: 'done', label: 'done' }];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges } }, error: null }) };
    await settle();

    (el as any)._selectEdge('e1');
    // Backspace while focused in a text input must NOT delete.
    (el as any)._onKeyDown({ key: 'Backspace', preventDefault() {}, composedPath: () => [{ tagName: 'INPUT' }] });
    expect((el as any)._edges).toHaveLength(1);
    // Delete on the canvas removes the selected edge.
    (el as any)._onKeyDown({ key: 'Delete', preventDefault() {}, composedPath: () => [{ tagName: 'DIV' }] });
    expect((el as any)._edges).toHaveLength(0);
    expect((el as any)._selectedEdgeId).toBeNull();
  });

  it('Delete removes the selected node and its incident edges', async () => {
    const nodes = [
      { id: 'a', kind: 'log', label: 'A', description: '', x: 0, y: 0 },
      { id: 'b', kind: 'end', label: 'B', description: '', x: 0, y: 100 },
    ];
    const edges = [{ id: 'e1', from: 'a', to: 'b', from_port: 'done', label: 'done' }];
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph: { nodes, edges } }, error: null }) };
    await settle();
    (el as any)._selectedNodeId = 'a';
    (el as any)._onKeyDown({ key: 'Delete', preventDefault() {}, composedPath: () => [{ tagName: 'DIV' }] });
    expect((el as any)._nodes.map((n: any) => n.id)).toEqual(['b']);
    expect((el as any)._edges).toHaveLength(0);
  });

  it('condition builder parses and composes lhs/op/rhs round-trip', () => {
    const p = (el as any)._parseCondition('customer.tier == gold');
    expect(p).toEqual({ lhs: 'customer.tier', op: '==', rhs: 'gold' });
    // multi-operator → not basic-representable
    expect((el as any)._parseCondition('a == b == c')).toBeNull();
    // bare variable → truthy test (empty rhs)
    expect((el as any)._parseCondition('vip')).toEqual({ lhs: 'vip', op: '==', rhs: '' });
    // compose drops the operator when rhs is empty
    expect((el as any)._composeCondition({ lhs: 'vip', op: '==', rhs: '' })).toBe('vip');
    expect((el as any)._composeCondition({ lhs: 'age', op: '>', rhs: '18' })).toBe('age > 18');
  });

  it('loads a backend-shaped graph (type/config) into the UI shape (kind/params)', async () => {
    // Graph authored via the API only carries type/config — the UI must map it
    // to kind/params on load or the render crashes on node.kind.
    const graph = {
      nodes: [
        { id: 't', type: 'trigger' },
        { id: 'q', type: 'route_queue', config: { queue: 'queue_vip' } },
      ],
      edges: [{ from: 't', to: 'q', label: 'done' }],
    };
    (el as any).orgId = 'test-org';
    (el as any).flowId = MOCK_FLOW.id;
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, graph }, error: null }) };
    await settle();
    const q = (el as any)._nodes.find((n: any) => n.id === 'q');
    expect(q.kind).toBe('route_queue');
    expect(q.params).toEqual({ queue: 'queue_vip' });
    expect((el as any)._edges[0].from_port).toBe('done'); // label → from_port
    expect(el.shadowRoot!.querySelector('.builder-status--error')).toBeNull();
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
