// Tests for <or-trace-viewer> (v0.2 Layer 2). Read-only viewer bound to
// GET /traces/{id}, which is a not-implemented stub until Layer 3 — it probes
// the real endpoint, reports the stub, and renders the sample trace preview.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './trace-viewer.js';

const NOT_IMPL = { data: null, error: { error: 'internal', reason: 'not_implemented' }, response: { status: 500 } };

describe('OrTraceViewer', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-trace-viewer');
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

  it('renders the sample-trace banner and the viewer preview', async () => {
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    (el as any).client = { GET: vi.fn().mockResolvedValue(NOT_IMPL) };
    await settle();
    const sr = el.shadowRoot!;
    expect(sr.querySelector('.trace-banner')?.textContent).toContain('Layer 3');
    expect(sr.textContent).toContain('Trace viewer');
  });

  it('probes GET /traces/{id} and flips _stub on not_implemented', async () => {
    const mockGet = vi.fn().mockResolvedValue(NOT_IMPL);
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    (el as any).client = { GET: mockGet };
    await settle();
    expect(mockGet).toHaveBeenCalled();
    const [path, opts] = mockGet.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/traces/{id}');
    expect(opts?.params?.path).toEqual({ org_id: 'test-org', id: 'trace-1' });
    expect((el as any)._stub).toBe(true);
  });

  it('does not probe when no client/traceId is set (still renders preview)', async () => {
    (el as any).orgId = 'test-org';
    (el as any).traceId = '';
    await settle();
    expect(el.shadowRoot!.textContent).toContain('Trace viewer');
    expect((el as any)._stub).toBe(false);
  });
});
