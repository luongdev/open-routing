// Phase 6 Plan 13 Task 2: Tests for <or-import-result>
// TDD RED — fails until import-result.ts is implemented.
// Uses ImportJob shape from GET /v1/orgs/{orgId}/imports/{importId}.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { components } from '../../api/generated.js';

// Registers <or-import-result> element
import './import-result.js';

type ImportJob = components['schemas']['ImportJob'];

/** Successful import job fixture */
const JOB_SUCCESS: ImportJob = {
  id: '01935b00-0000-7000-8000-000000000001',
  org_id: '01935b00-0000-7000-8000-abcdef000001',
  entity_type: 'agents',
  status: 'completed',
  total_rows: 5,
  succeeded_rows: 5,
  failed_rows: 0,
  errors: [],
  created_at: '2026-05-18T03:00:00Z',
};

/** Partial success (207) import job fixture */
const JOB_PARTIAL: ImportJob = {
  id: '01935b00-0000-7000-8000-000000000002',
  org_id: '01935b00-0000-7000-8000-abcdef000001',
  entity_type: 'agents',
  status: 'completed',
  total_rows: 10,
  succeeded_rows: 7,
  failed_rows: 3,
  errors: [
    { row: 2, field: 'email', error: 'import_failed', reason: 'duplicate email' },
    { row: 5, field: 'code', error: 'import_failed', reason: 'duplicate code' },
    { row: 8, field: null, error: 'import_failed', reason: 'required field missing: name' },
  ],
  created_at: '2026-05-18T03:00:00Z',
};

/** All-failed import job fixture */
const JOB_ALL_FAILED: ImportJob = {
  id: '01935b00-0000-7000-8000-000000000003',
  org_id: '01935b00-0000-7000-8000-abcdef000001',
  entity_type: 'skills',
  status: 'failed',
  total_rows: 3,
  succeeded_rows: 0,
  failed_rows: 3,
  errors: [
    { row: 1, field: 'code', error: 'import_failed', reason: 'duplicate code' },
    { row: 2, field: 'name', error: 'import_failed', reason: 'required' },
    { row: 3, field: null, error: 'import_failed', reason: 'invalid schema' },
  ],
  created_at: '2026-05-18T03:00:00Z',
};

/** Helper: wait for async renders */
function nextTick(): Promise<void> {
  return new Promise((r) => setTimeout(r, 80));
}

/** Helper: create a mock client */
function mockClient(result: { data?: ImportJob | null; error?: unknown }) {
  return {
    GET: vi.fn().mockResolvedValue(result),
  };
}

describe('OrImportResult', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-import-result');
    (el as any).orgId = '01935b00-0000-7000-8000-abcdef000001';
    (el as any).importId = '01935b00-0000-7000-8000-000000000001';
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  // Test 1: success banner (green) when succeeded === total and failed === 0
  it('renders success banner when all rows succeeded', async () => {
    (el as any).client = mockClient({ data: JOB_SUCCESS, error: null });
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const successBanner = shadow.querySelector('[data-banner="success"]');
    expect(successBanner).not.toBeNull();
    // Success banner text should mention total records
    expect(successBanner!.textContent).toContain('5');
  });

  // Test 2: partial banner (amber) for 207; "Imported N of M..."
  it('renders partial banner for partial success with correct counts', async () => {
    (el as any).client = mockClient({ data: JOB_PARTIAL, error: null });
    (el as any).importId = JOB_PARTIAL.id;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const partialBanner = shadow.querySelector('[data-banner="partial"]');
    expect(partialBanner).not.toBeNull();
    expect(partialBanner!.textContent).toContain('7');
    expect(partialBanner!.textContent).toContain('10');
    expect(partialBanner!.textContent).toContain('3');
  });

  // Test 3: failure banner (red) when succeeded === 0
  it('renders failure banner when all rows failed', async () => {
    (el as any).client = mockClient({ data: JOB_ALL_FAILED, error: null });
    (el as any).importId = JOB_ALL_FAILED.id;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const failBanner = shadow.querySelector('[data-banner="failure"]');
    expect(failBanner).not.toBeNull();
  });

  // Test 4: stat cards Succeeded / Failed / Total
  it('shows 3 stat cards with correct numbers', async () => {
    (el as any).client = mockClient({ data: JOB_PARTIAL, error: null });
    (el as any).importId = JOB_PARTIAL.id;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Ember redesign uses stat-card divs; numbers + labels co-located
    const text = shadow.textContent ?? '';
    expect(text).toContain('7');   // succeeded count
    expect(text).toContain('3');   // failed count
    expect(text).toContain('10');  // total count
    expect(text.toLowerCase()).toMatch(/imported|succeeded/);
    expect(text.toLowerCase()).toContain('failed');
  });

  // Test 5: failed rows table renders Row | Field | Reason columns
  it('renders failed rows table with Row/Field/Reason columns', async () => {
    (el as any).client = mockClient({ data: JOB_PARTIAL, error: null });
    (el as any).importId = JOB_PARTIAL.id;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const failTable = shadow.querySelector('[data-failures-table]');
    expect(failTable).not.toBeNull();

    // Should have 3 rows worth of content
    const rows = failTable!.querySelectorAll('[data-failure-row]');
    expect(rows.length).toBe(3);
  });

  // Test 6: download failures button always disabled with v0.2 tooltip
  it('renders download failures button as always disabled with v0.2 tooltip', async () => {
    (el as any).client = mockClient({ data: JOB_PARTIAL, error: null });
    (el as any).importId = JOB_PARTIAL.id;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Download button is a <button> with text containing "Download failures"
    const buttons = Array.from(shadow.querySelectorAll('button'));
    const downloadBtn = buttons.find((b) => b.textContent?.toLowerCase().includes('download failures')) as HTMLButtonElement | undefined;
    expect(downloadBtn).toBeTruthy();
    // Must be disabled (v0.2 deferred per IMP-11)
    expect(downloadBtn!.disabled || downloadBtn!.hasAttribute('disabled')).toBe(true);
    // Title attribute carries the v0.2 hint
    const title = downloadBtn!.getAttribute('title') ?? '';
    expect(title.toLowerCase()).toContain('v0.2');
  });

  // Test 7: idempotent_replay=true shows special message in disclosure
  it('shows special idempotent_replay message in disclosure when true', async () => {
    // BulkImportResult with idempotent_replay=true
    const replayResult = {
      ...JOB_SUCCESS,
      id: '01935b00-0000-7000-8000-000000000004',
    };

    // Mock client returns a result that the component exposes idempotent_replay on
    const mockWithReplay = {
      GET: vi.fn().mockResolvedValue({
        data: replayResult,
        error: null,
        // Inject idempotent_replay via extra field on the result
      }),
    };

    (el as any).client = mockWithReplay;
    (el as any).importId = replayResult.id;
    // Simulate idempotent replay mode
    (el as any)._idempotentReplay = true;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const disclosure = shadow.querySelector('[data-succeeded-disclosure]');
    expect(disclosure).not.toBeNull();
    // When idempotent replay, should show special message
    expect(disclosure!.textContent).toMatch(/idempotent|replay/i);
  });

  // Test 8: 404 shows "Import not found in this org." empty state
  it('shows not-found empty state on 404', async () => {
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: null,
        error: { error: 'not_found', reason: 'Import job not found' },
        response: { status: 404 },
      }),
    };
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const notFound = shadow.querySelector('[data-empty="not-found"]');
    expect(notFound).not.toBeNull();
    expect(notFound!.textContent).toContain('not found');
  });
});
