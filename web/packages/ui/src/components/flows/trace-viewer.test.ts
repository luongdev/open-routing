// Tests for <or-trace-viewer> (v0.2 Layer 2). Read-only viewer bound to
// GET /traces/{id}, which is a not-implemented stub until Layer 3 — it probes
// the real endpoint, and while there is no live trace it shows the sample
// preview behind a banner. A real trace hides the banner.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './trace-viewer.js';

const NOT_IMPL = { data: null, error: { error: 'internal', reason: 'not_implemented' }, response: { status: 500 } };
const LIVE_TRACE = {
  data: { id: 't1', org_id: 'o1', kind: 'runtime', steps: [], created_at: '2026-01-01T00:00:00Z' },
  error: null,
  response: { status: 200 },
};

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

  it('shows the sample banner + preview when the trace endpoint is a stub', async () => {
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    (el as any).client = { GET: vi.fn().mockResolvedValue(NOT_IMPL) };
    await settle();
    const sr = el.shadowRoot!;
    expect(sr.querySelector('.trace-banner')?.textContent).toContain('Layer 3');
    expect(sr.textContent).toContain('Trace viewer');
  });

  it('probes GET /traces/{id} with the right path params', async () => {
    const mockGet = vi.fn().mockResolvedValue(NOT_IMPL);
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    (el as any).client = { GET: mockGet };
    await settle();
    expect(mockGet).toHaveBeenCalled();
    const [path, opts] = mockGet.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/traces/{id}');
    expect(opts?.params?.path).toEqual({ org_id: 'test-org', id: 'trace-1' });
  });

  it('hides the sample banner when a live trace is returned', async () => {
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    (el as any).client = { GET: vi.fn().mockResolvedValue(LIVE_TRACE) };
    await settle();
    expect(el.shadowRoot!.querySelector('.trace-banner')).toBeNull();
  });

  it('reruns the probe when the client arrives after orgId/traceId', async () => {
    const mockGet = vi.fn().mockResolvedValue(NOT_IMPL);
    (el as any).orgId = 'test-org';
    (el as any).traceId = 'trace-1';
    await settle();
    expect(mockGet).not.toHaveBeenCalled();
    (el as any).client = { GET: mockGet };
    await settle();
    expect(mockGet).toHaveBeenCalled();
  });

  it('does not probe when no traceId is set (still renders preview + banner)', async () => {
    const mockGet = vi.fn().mockResolvedValue(NOT_IMPL);
    (el as any).orgId = 'test-org';
    (el as any).traceId = '';
    (el as any).client = { GET: mockGet };
    await settle();
    expect(mockGet).not.toHaveBeenCalled();
    expect(el.shadowRoot!.textContent).toContain('Trace viewer');
    expect(el.shadowRoot!.querySelector('.trace-banner')).not.toBeNull();
  });
});
