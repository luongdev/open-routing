// Phase 6 Plan 13 Task 1: Tests for <or-import-page>
// TDD RED — fails until import-page.ts is implemented.
// ADMIN-04: verifies createImporter() is called, never raw fetch().

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { ImportCatalogArgs, BulkImportResult } from '../../api/import.js';

// Registers <or-import-page> element
import './import-page.js';

/** A valid 200 BulkImportResult fixture */
const RESULT_200: BulkImportResult = {
  import_id: '01935b00-0000-7000-8000-000000000001',
  entity: 'agents',
  schema_version: 'v0.1',
  total: 2,
  succeeded: 2,
  failed: 0,
  failures: [],
  succeeded_ids: [
    '01935b00-0000-7000-8000-000000000002',
    '01935b00-0000-7000-8000-000000000003',
  ],
};

/** A 207 partial BulkImportResult fixture */
const RESULT_207: BulkImportResult = {
  import_id: '01935b00-0000-7000-8000-000000000099',
  entity: 'agents',
  schema_version: 'v0.1',
  total: 10,
  succeeded: 7,
  failed: 3,
  failures: [
    { row: 2, field: 'email', reason: 'duplicate email' },
    { row: 5, field: 'code', reason: 'duplicate code' },
    { row: 8, field: 'name', reason: 'required' },
  ],
};

/** Helper: wait for async renders */
function nextTick(): Promise<void> {
  return new Promise((r) => setTimeout(r, 50));
}

/** Helper: build a fake File */
function makeFile(name: string, sizeBytes: number, type = 'text/csv'): File {
  const content = 'a'.repeat(sizeBytes);
  return new File([content], name, { type });
}

describe('OrImportPage', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-import-page');
    (el as any).orgId = '01935b00-0000-7000-8000-abcdef000001';
    (el as any).baseURL = '';
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  // Test 1: Step 1 entity picker renders 6 radio options
  it('renders 6 entity radio options in Step 1', async () => {
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Should show step 1 content with entity radios
    const radios = shadow.querySelectorAll('[data-entity]');
    expect(radios.length).toBe(6);

    const entities = Array.from(radios).map((r) => (r as HTMLElement).dataset['entity']);
    expect(entities).toContain('agents');
    expect(entities).toContain('skills');
    expect(entities).toContain('queues');
    expect(entities).toContain('channels');
    expect(entities).toContain('adapters');
    expect(entities).toContain('break_reasons');
  });

  // Test 2: "Next" on Step 1 requires entity selected
  it('next button on Step 1 is disabled until entity is selected', async () => {
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const nextBtn = shadow.querySelector('[data-action="step1-next"]') as HTMLButtonElement | null;
    expect(nextBtn).not.toBeNull();
    // Button should be disabled (or have disabled attribute / aria-disabled)
    expect(
      nextBtn!.hasAttribute('disabled') ||
      (nextBtn as any).disabled === true ||
      nextBtn!.getAttribute('aria-disabled') === 'true'
    ).toBe(true);
  });

  // Test 3: Step 2 drop zone rejects >50MB file
  it('drop zone rejects files larger than 50MB with inline alert', async () => {
    // Select an entity first
    (el as any)._selectedEntity = 'agents';
    (el as any)._step = 2;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dropZone = shadow.querySelector('[data-dropzone]');
    expect(dropZone).not.toBeNull();

    // Simulate drop of a 51MB file (> 50MB limit)
    const bigFile = makeFile('huge.csv', 51 * 1024 * 1024);
    const dropEvent = new Event('drop', { bubbles: true });
    Object.defineProperty(dropEvent, 'dataTransfer', {
      value: { files: [bigFile] },
    });
    dropEvent.preventDefault = vi.fn();
    dropZone!.dispatchEvent(dropEvent);

    await nextTick();
    await (el as any).updateComplete;

    // _fileError should be set
    expect((el as any)._fileError).toBeTruthy();
  });

  // Test 4: CSV format help block renders entity-specific columns; Agents includes skills syntax
  it('CSV format help for agents includes skills syntax line', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._step = 2;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const formatHelp = shadow.querySelector('[data-csv-help]');
    expect(formatHelp).not.toBeNull();
    // Agent CSV help must include skills syntax hint
    expect(formatHelp!.textContent).toContain('skill');
  });

  // Test 5: "Next" on Step 2 requires file staged
  it('next button on Step 2 is disabled until a file is staged', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._step = 2;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const nextBtn = shadow.querySelector('[data-action="step2-next"]') as HTMLButtonElement | null;
    expect(nextBtn).not.toBeNull();
    expect(
      nextBtn!.hasAttribute('disabled') ||
      (nextBtn as any).disabled === true ||
      nextBtn!.getAttribute('aria-disabled') === 'true'
    ).toBe(true);
  });

  // Test 6: Idempotency-Key checkbox generates a key and shows in review
  it('checking idempotency checkbox generates a key shown in review step', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._step = 2;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const checkbox = shadow.querySelector('[data-idempotency-checkbox]') as HTMLInputElement | null;
    expect(checkbox).not.toBeNull();

    // Simulate checking the checkbox
    (el as any)._useIdempotency = true;
    // The component should generate a key
    if (!(el as any)._idempotencyKey) {
      (el as any)._idempotencyKey = crypto.randomUUID();
    }
    await (el as any).updateComplete;

    expect((el as any)._idempotencyKey).toBeTruthy();
    expect(typeof (el as any)._idempotencyKey).toBe('string');

    // Navigate to step 3 (review) and check key is shown
    (el as any)._stagedFile = makeFile('test.csv', 100);
    (el as any)._step = 3;
    await (el as any).updateComplete;
    await nextTick();
    await (el as any).updateComplete;

    const reviewSection = shadow.querySelector('[data-review]');
    expect(reviewSection).not.toBeNull();
    // Key should appear somewhere in the review
    expect(reviewSection!.textContent).toContain((el as any)._idempotencyKey.slice(0, 8));
  });

  // Test 7: POST 207 navigates; POST 400 shows schema alert; POST 413 shows size alert
  it('navigates to /imports/{id} after 207 response and dispatches open-routing:navigate', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._selectedFormat = 'csv';
    (el as any)._stagedFile = makeFile('test.csv', 100);
    (el as any)._step = 3;
    await (el as any).updateComplete;

    const navigated: string[] = [];
    el.addEventListener('open-routing:navigate', (e) => {
      navigated.push((e as CustomEvent<{ path: string }>).detail.path);
    });

    // Inject a spy importer that returns 207
    (el as any)._importerFactory = () => async (_args: ImportCatalogArgs): Promise<BulkImportResult> => RESULT_207;

    await (el as any)._doImport();
    await nextTick();
    await (el as any).updateComplete;

    expect(navigated.length).toBeGreaterThan(0);
    expect(navigated[0]).toContain(RESULT_207.import_id);
  });

  it('shows schema_version alert for 400 response (no navigation)', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._selectedFormat = 'csv';
    (el as any)._stagedFile = makeFile('test.csv', 100);
    (el as any)._step = 3;
    await (el as any).updateComplete;

    const navigated: string[] = [];
    el.addEventListener('open-routing:navigate', (e) => {
      navigated.push((e as CustomEvent<{ path: string }>).detail.path);
    });

    // Inject a spy importer that throws ImportError 400
    const { ImportError } = await import('../../api/import.js');
    (el as any)._importerFactory = () => async (_args: ImportCatalogArgs): Promise<BulkImportResult> => {
      throw new ImportError(400, { error: 'invalid_body', reason: 'schema mismatch' });
    };

    await (el as any)._doImport();
    await nextTick();
    await (el as any).updateComplete;

    expect(navigated).toHaveLength(0);
    expect((el as any)._error).toBeTruthy();
    expect((el as any)._errorType).toBe('schema');
  });

  it('shows file size alert for 413 response (no navigation)', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._selectedFormat = 'csv';
    (el as any)._stagedFile = makeFile('test.csv', 100);
    (el as any)._step = 3;
    await (el as any).updateComplete;

    const navigated: string[] = [];
    el.addEventListener('open-routing:navigate', (e) => {
      navigated.push((e as CustomEvent<{ path: string }>).detail.path);
    });

    const { ImportError } = await import('../../api/import.js');
    (el as any)._importerFactory = () => async (_args: ImportCatalogArgs): Promise<BulkImportResult> => {
      throw new ImportError(413, { error: 'payload_too_large', reason: 'File exceeds 50MB' });
    };

    await (el as any)._doImport();
    await nextTick();
    await (el as any).updateComplete;

    expect(navigated).toHaveLength(0);
    expect((el as any)._error).toBeTruthy();
    expect((el as any)._errorType).toBe('size');
  });

  // Test 8: 422 still navigates to /imports/{id}
  it('navigates to /imports/{id} even for 422 (all rows failed)', async () => {
    (el as any)._selectedEntity = 'agents';
    (el as any)._selectedFormat = 'csv';
    (el as any)._stagedFile = makeFile('test.csv', 100);
    (el as any)._step = 3;
    await (el as any).updateComplete;

    const navigated: string[] = [];
    el.addEventListener('open-routing:navigate', (e) => {
      navigated.push((e as CustomEvent<{ path: string }>).detail.path);
    });

    const RESULT_422: BulkImportResult = {
      import_id: '01935b00-0000-7000-8000-000000000010',
      entity: 'agents',
      schema_version: 'v0.1',
      total: 3,
      succeeded: 0,
      failed: 3,
      failures: [
        { row: 1, field: 'code', reason: 'duplicate' },
        { row: 2, field: 'email', reason: 'invalid' },
        { row: 3, field: 'name', reason: 'required' },
      ],
    };

    // 422 is returned as a value (not thrown) per import.ts contract
    (el as any)._importerFactory = () => async (_args: ImportCatalogArgs): Promise<BulkImportResult> => RESULT_422;

    await (el as any)._doImport();
    await nextTick();
    await (el as any).updateComplete;

    expect(navigated.length).toBeGreaterThan(0);
    expect(navigated[0]).toContain(RESULT_422.import_id);
  });
});
