// Phase 6 Plan 13 Task 1: <or-import-page> — 3-step bulk import wizard.
//
// Step 1: Pick entity (6 entities: agents/skills/queues/channels/adapters/break_reasons)
// Step 2: Upload file (drag-drop zone + browse; CSV/JSON toggle; format help per entity)
// Step 3: Review + submit via createImporter() from ../../api/import.js
//
// ADMIN-04: all HTTP is via createImporter() only. No direct fetch calls permitted.
// D6-08: Per-component Shoelace imports for tree-shaking.
// D6-13: UUIDv7 idempotency key generated via crypto.randomUUID().
// W0.1-22: redesigned to Ember bulk-import-wizard style (no sl-* in shadow).

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { createImporter, ImportError } from '../../api/import.js';
import type { CatalogEntity, ImportCatalogArgs, BulkImportResult } from '../../api/import.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import '../primitives/form-wizard.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';

/** 50 MB size limit per D5-22 / IMP-07 */
const MAX_FILE_SIZE_BYTES = 50 * 1024 * 1024;

/** Entity label mapping */
const ENTITY_LABELS: Record<CatalogEntity, string> = {
  agents: 'Agents',
  skills: 'Skills',
  queues: 'Queues',
  channels: 'Channels',
  adapters: 'Adapters',
  break_reasons: 'Break Reasons',
};

const ENTITY_ICONS: Record<CatalogEntity, string> = {
  agents: 'users',
  skills: 'star',
  queues: 'list',
  channels: 'radio',
  adapters: 'plug',
  break_reasons: 'coffee',
};

/** CSV format help content per entity (D5-04, D5-15..D5-17, UI-SPEC §5.8) */
const CSV_FORMAT_HELP: Record<CatalogEntity, { required: string; optional: string; extra?: string }> = {
  agents: {
    required: 'code, name, email',
    optional: 'external_id, enabled, skills',
    extra: 'Skills syntax: skill_code:proficiency|skill_code:proficiency\nExample: skill_voice:7|skill_chat:9',
  },
  skills: {
    required: 'code, name',
    optional: 'external_id, description, skill_type, enabled',
  },
  queues: {
    required: 'code, name',
    optional: 'external_id, channel_types, priority, acw_sec, enabled',
    extra: 'Multi-value columns: | > ; > ,',
  },
  channels: {
    required: 'code, name, channel_type',
    optional: 'external_id, default_queue_code, enabled',
    extra: 'default_queue_code references a queue\'s code (not UUID)',
  },
  adapters: {
    required: 'code, name, adapter_type',
    optional: 'external_id, enabled',
    extra: 'config excluded from CSV; use JSON for adapter config',
  },
  break_reasons: {
    required: 'code, name',
    optional: 'external_id, routable, display_order, enabled',
  },
};

/** BulkImportResult extended with import_id from the import job (per plan interface) */
interface ExtendedBulkImportResult extends BulkImportResult {
  import_id?: string;
}

const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'pick', label: 'Choose entity' },
  { key: 'upload', label: 'Upload file' },
  { key: 'review', label: 'Review' },
];

/**
 * <or-import-page> — Ember bulk-import-wizard style, 3-step.
 *
 * Step 1: Entity picker (6 radio cards in a grid)
 * Step 2: File upload (drag-drop zone, format toggle, CSV help, idempotency)
 * Step 3: Review + submit
 *
 * All HTTP calls are via createImporter() — ADMIN-04 forbids direct fetch calls.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — org UUID from URL
 *   - baseURL: (attribute 'base-url') — API origin, '' for same-origin
 *   - client: (no attribute) — passed from shell; not used directly (createImporter handles HTTP)
 */
@customElement('or-import-page')
export class OrImportPage extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      margin-bottom: 28px;
    }

    .page-header-top {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 4px;
    }

    .back-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 6px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      transition: color .12s, background .12s;
    }

    .back-btn:hover {
      color: var(--foreground);
      background: var(--muted);
    }

    .page-title {
      font-size: 24px;
      font-weight: 700;
      margin: 0;
      color: var(--foreground);
    }

    .page-subtitle {
      font-size: 14px;
      color: var(--muted-foreground);
      margin: 0 0 0 44px;
    }

    /* ── Wizard card ─────────────────────────────────────────────── */
    .wizard-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      max-width: 720px;
      overflow: hidden;
    }

    .wizard-body {
      padding: 28px 32px;
    }

    /* ── Step heading ────────────────────────────────────────────── */
    .step-heading {
      font-size: 17px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 6px;
    }

    .step-subheading {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0 0 20px;
    }

    /* ── Radio cards grid (entity picker) ───────────────────────── */
    .radio-card-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 12px;
      margin-bottom: 24px;
    }

    @media (max-width: 560px) {
      .radio-card-grid { grid-template-columns: repeat(2, 1fr); }
    }

    .radio-card {
      position: relative;
      border: 2px solid var(--border);
      border-radius: 10px;
      padding: 16px 14px;
      cursor: pointer;
      background: var(--card);
      text-align: left;
      transition: border-color .15s, background .15s, box-shadow .15s;
    }

    .radio-card:hover {
      border-color: var(--primary);
      background: color-mix(in oklch, var(--primary) 6%, transparent);
    }

    .radio-card.selected {
      border-color: var(--primary);
      background: color-mix(in oklch, var(--primary) 8%, transparent);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 20%, transparent);
    }

    .radio-card input[type="radio"] {
      position: absolute;
      opacity: 0;
      width: 0;
      height: 0;
    }

    .radio-card-icon {
      color: var(--muted-foreground);
      margin-bottom: 8px;
      display: block;
    }

    .radio-card.selected .radio-card-icon {
      color: var(--primary);
    }

    .radio-card-label {
      font-size: 14px;
      font-weight: 600;
      color: var(--foreground);
      display: block;
    }

    .radio-card-check {
      position: absolute;
      top: 10px;
      right: 10px;
      width: 18px;
      height: 18px;
      border-radius: 50%;
      background: var(--primary);
      display: flex;
      align-items: center;
      justify-content: center;
      opacity: 0;
      transition: opacity .12s;
    }

    .radio-card.selected .radio-card-check {
      opacity: 1;
    }

    /* ── File drop zone ──────────────────────────────────────────── */
    .file-drop {
      border: 2px dashed var(--border);
      border-radius: 10px;
      padding: 48px 32px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 14px;
      cursor: pointer;
      transition: border-color .15s, background .15s;
      margin-bottom: 16px;
      background: var(--muted, #fafafa);
    }

    .file-drop:hover,
    .file-drop.drag-over {
      border-color: var(--primary);
      background: color-mix(in oklch, var(--primary) 6%, transparent);
      color: var(--foreground);
    }

    .file-drop-icon {
      display: block;
      margin: 0 auto 12px;
      color: var(--muted-foreground);
    }

    .file-drop:hover .file-drop-icon,
    .file-drop.drag-over .file-drop-icon {
      color: var(--primary);
    }

    .file-drop-hint {
      font-size: 13px;
      margin-top: 6px;
      color: var(--muted-foreground);
    }

    .file-drop-link {
      color: var(--primary);
      text-decoration: underline;
      cursor: pointer;
      background: none;
      border: none;
      font-size: inherit;
      font-family: inherit;
      padding: 0;
    }

    /* Staged file */
    .file-staged {
      display: flex;
      align-items: center;
      gap: 10px;
      border: 2px solid color-mix(in oklch, oklch(0.65 0.18 145) 40%, transparent);
      border-radius: 10px;
      padding: 14px 16px;
      background: color-mix(in oklch, oklch(0.65 0.18 145) 8%, transparent);
      margin-bottom: 16px;
      font-size: 13px;
    }

    .file-staged-name {
      flex: 1;
      font-weight: 600;
      color: var(--foreground);
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .file-staged-size {
      color: var(--muted-foreground);
      font-size: 12px;
      flex-shrink: 0;
    }

    .file-staged-remove {
      background: none;
      border: none;
      cursor: pointer;
      padding: 4px;
      border-radius: 4px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      transition: color .12s;
      flex-shrink: 0;
    }

    .file-staged-remove:hover { color: var(--destructive); }

    /* Format toggle — pill tabs */
    .format-tabs {
      display: inline-flex;
      border: 1px solid var(--border);
      border-radius: 8px;
      overflow: hidden;
      margin-bottom: 16px;
      background: var(--muted);
    }

    .format-tab {
      padding: 6px 20px;
      font-size: 13px;
      font-weight: 500;
      border: none;
      background: transparent;
      cursor: pointer;
      color: var(--muted-foreground);
      transition: background .12s, color .12s;
    }

    .format-tab.active {
      background: var(--card);
      color: var(--foreground);
      box-shadow: 0 1px 4px rgba(0,0,0,.08);
    }

    /* CSV help block */
    .csv-help {
      background: var(--muted, #f5f5f5);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px 16px;
      font-size: 12px;
      margin-bottom: 16px;
    }

    .csv-help-header {
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: .06em;
      color: var(--muted-foreground);
      margin-bottom: 8px;
    }

    .csv-help pre {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--foreground);
      margin: 0;
      white-space: pre-wrap;
    }

    /* Schema chip */
    .schema-chip {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
      border-radius: 12px;
      padding: 3px 10px;
      font-size: 12px;
      font-weight: 600;
      margin-bottom: 14px;
    }

    /* Idempotency section */
    .idempotency-section {
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px 16px;
      margin-top: 16px;
      background: var(--card);
    }

    .idempotency-label {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      font-weight: 500;
      color: var(--foreground);
      cursor: pointer;
      user-select: none;
    }

    .idempotency-sub {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 4px;
      padding-left: 26px;
    }

    .idempotency-key {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--foreground);
      background: var(--muted);
      padding: 4px 10px;
      border-radius: 4px;
      margin-top: 8px;
      word-break: break-all;
    }

    /* Toggle / checkbox */
    .switch-wrap {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;
      user-select: none;
    }

    .switch-wrap input[type="checkbox"] { display: none; }

    .switch-track {
      position: relative;
      width: 34px;
      height: 18px;
      border-radius: 9999px;
      background: var(--border);
      transition: background .15s;
      flex-shrink: 0;
    }

    .switch-wrap:has(input:checked) .switch-track { background: var(--primary); }

    .switch-thumb {
      position: absolute;
      top: 2px;
      left: 2px;
      width: 14px;
      height: 14px;
      border-radius: 50%;
      background: white;
      box-shadow: 0 1px 3px rgba(0,0,0,.2);
      transition: transform .15s;
    }

    .switch-wrap:has(input:checked) .switch-thumb { transform: translateX(16px); }

    /* Review table */
    .review-table {
      border: 1px solid var(--border);
      border-radius: 10px;
      overflow: hidden;
      margin-bottom: 16px;
    }

    .review-row {
      display: flex;
      padding: 11px 16px;
      border-bottom: 1px solid var(--border);
      font-size: 14px;
      align-items: flex-start;
      gap: 12px;
    }

    .review-row:last-child { border-bottom: none; }

    .review-label {
      width: 140px;
      flex-shrink: 0;
      font-weight: 500;
      font-size: 13px;
      color: var(--muted-foreground);
      padding-top: 1px;
    }

    .review-value {
      color: var(--foreground);
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
    }

    .review-warning {
      display: flex;
      gap: 8px;
      align-items: flex-start;
      background: color-mix(in oklch, oklch(0.75 0.18 80) 12%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.75 0.18 80) 40%, transparent);
      border-radius: 8px;
      padding: 12px 14px;
      font-size: 13px;
      color: oklch(0.5 0.18 80);
      margin-bottom: 16px;
    }

    /* Error / alert banners */
    .alert {
      display: flex;
      gap: 8px;
      align-items: flex-start;
      border-radius: 8px;
      padding: 12px 14px;
      font-size: 13px;
      margin-bottom: 16px;
    }

    .alert-danger {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
    }

    .alert-warning {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 12%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.75 0.18 80) 40%, transparent);
      color: oklch(0.5 0.18 80);
    }

    /* Step nav */
    .step-nav {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 24px;
      padding-top: 20px;
      border-top: 1px solid var(--border);
    }

    /* Loading spinner */
    .spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid rgba(255,255,255,.3);
      border-top-color: white;
      border-radius: 50%;
      animation: spin .6s linear infinite;
      margin-right: 6px;
      vertical-align: middle;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    /* Inline result stat cards */
    .stat-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 12px;
      margin-bottom: 24px;
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
      text-align: center;
      box-shadow: var(--shadow-xs);
    }

    .stat-number {
      font-size: 28px;
      font-weight: 700;
      display: block;
      color: var(--foreground);
    }

    .stat-number--success { color: oklch(0.55 0.18 145); }
    .stat-number--danger  { color: var(--destructive); }
    .stat-number--neutral { color: var(--muted-foreground); }

    .stat-label {
      font-size: 13px;
      color: var(--muted-foreground);
      margin-top: 4px;
      display: block;
    }

    /* Errors table */
    .errors-section h3 {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 10px;
    }

    .errors-table {
      border: 1px solid var(--border);
      border-radius: 10px;
      overflow: hidden;
    }

    .errors-header {
      display: grid;
      grid-template-columns: 56px 130px 1fr;
      background: var(--muted);
      padding: 8px 14px;
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: .06em;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }

    .error-row {
      display: grid;
      grid-template-columns: 56px 130px 1fr;
      padding: 9px 14px;
      font-size: 13px;
      border-bottom: 1px solid var(--border);
    }

    .error-row:last-child { border-bottom: none; }

    .error-row-num {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--muted-foreground);
    }

    .error-field {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--foreground);
    }

    .error-reason { color: var(--destructive); }

    input[type="file"] { display: none; }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'base-url' }) accessor baseURL = '';
  /** Optional typed client — not used directly; createImporter() handles HTTP (ADMIN-04) */
  @property({ attribute: false }) accessor client: ApiClient | null = null;

  @state() private accessor _step: 1 | 2 | 3 = 1;
  @state() private accessor _selectedEntity: CatalogEntity | null = null;
  @state() private accessor _selectedFormat: 'csv' | 'json' = 'csv';
  @state() private accessor _stagedFile: File | null = null;
  @state() private accessor _dragOver = false;
  @state() private accessor _fileError: string | null = null;
  @state() private accessor _useIdempotency = false;
  @state() private accessor _idempotencyKey: string | null = null;
  @state() private accessor _submitting = false;
  @state() private accessor _error: string | null = null;
  @state() private accessor _errorType: 'schema' | 'size' | 'generic' | null = null;
  /**
   * Cross-AI fix: When BulkImportResult lacks an import_id (current Phase 5 contract),
   * render the result inline rather than navigating to /imports/{wrong-id}. Set after
   * a successful POST when no import_id is present in the response.
   */
  @state() private accessor _inlineResult: BulkImportResult | null = null;

  /**
   * Factory for the importer function. Overridable in tests to inject a spy.
   * Returns a function with the same signature as the result of createImporter().
   */
  _importerFactory: (deps: { baseURL: string; getOrgId: () => string }) => (args: ImportCatalogArgs) => Promise<ExtendedBulkImportResult> = (deps) =>
    createImporter(deps) as unknown as (args: ImportCatalogArgs) => Promise<ExtendedBulkImportResult>;

  private get _wizardStep(): number {
    return this._step - 1;
  }

  private _formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  private _validateFile(file: File): string | null {
    if (file.size > MAX_FILE_SIZE_BYTES) {
      return 'File too large — maximum is 50 MB. Use the v0.2 async pathway for larger imports.';
    }
    const ext = file.name.split('.').pop()?.toLowerCase();
    if (this._selectedFormat === 'csv' && ext !== 'csv') {
      return `Format mismatch — expected a .csv file but got .${ext}. Switch to JSON or re-export as CSV.`;
    }
    if (this._selectedFormat === 'json' && ext !== 'json') {
      return `Format mismatch — expected a .json file but got .${ext}. Switch to CSV or re-export as JSON.`;
    }
    return null;
  }

  private _handleFileChosen(file: File): void {
    const err = this._validateFile(file);
    if (err) {
      this._fileError = err;
      this._stagedFile = null;
    } else {
      this._fileError = null;
      this._stagedFile = file;
    }
  }

  private _handleDragOver(e: DragEvent): void {
    e.preventDefault();
    this._dragOver = true;
  }

  private _handleDragLeave(): void {
    this._dragOver = false;
  }

  private _handleDrop(e: DragEvent): void {
    e.preventDefault();
    this._dragOver = false;
    const file = e.dataTransfer?.files?.[0];
    if (file) {
      this._handleFileChosen(file);
    }
  }

  private _handleBrowseClick(): void {
    const input = this.shadowRoot?.querySelector('input[type="file"]') as HTMLInputElement | null;
    if (input) input.click();
  }

  private _handleFileInputChange(e: Event): void {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (file) {
      this._handleFileChosen(file);
    }
    // Reset so the same file can be re-selected after removing it.
    input.value = '';
  }

  private _handleIdempotencyToggle(): void {
    this._useIdempotency = !this._useIdempotency;
    if (this._useIdempotency && !this._idempotencyKey) {
      this._idempotencyKey = crypto.randomUUID();
    }
    if (!this._useIdempotency) {
      this._idempotencyKey = null;
    }
  }

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _stepNext(): void {
    if (this._step === 1 && this._selectedEntity) {
      this._step = 2;
    } else if (this._step === 2 && this._stagedFile) {
      this._step = 3;
    }
  }

  private _stepBack(): void {
    if (this._step === 2) this._step = 1;
    else if (this._step === 3) this._step = 2;
  }

  async _doImport(): Promise<void> {
    if (!this._stagedFile || !this._selectedEntity) return;

    this._submitting = true;
    this._error = null;
    this._errorType = null;

    try {
      const fileText = await this._stagedFile.text();

      const importer = this._importerFactory({
        baseURL: this.baseURL,
        getOrgId: () => this.orgId,
      });

      const result = await importer({
        entity: this._selectedEntity,
        body: fileText,
        contentType: this._selectedFormat === 'csv' ? 'text/csv' : 'application/json',
        schemaVersion: this._selectedFormat === 'csv' ? 'v0.1' : undefined,
        idempotencyKey: this._useIdempotency && this._idempotencyKey ? this._idempotencyKey : undefined,
      });

      // Cross-AI fix (Codex HIGH): The OpenAPI v0.1 BulkImportResult does NOT include
      // an import_id field — the plan called for it but Phase 5 didn't add it to the
      // response shape. succeeded[] holds ENTITY IDs, not the import job ID, so the
      // previous fallback would navigate to /imports/{entity_id} → 404.
      // Until the server contract gains a real import_id, render the result inline
      // on this page (success counts + failed[] table) rather than mis-navigating.
      const ext = result as ExtendedBulkImportResult;
      if (ext.import_id) {
        this.dispatchEvent(
          new CustomEvent('open-routing:navigate', {
            detail: { path: `/orgs/${this.orgId}/imports/${ext.import_id}` },
            bubbles: true,
            composed: true,
          })
        );
      } else {
        this._inlineResult = result;
      }
    } catch (err) {
      if (err instanceof ImportError) {
        if (err.status === 400) {
          this._error = 'This file was generated against a different schema version; current is v0.1. Regenerate using the v0.1 export template.';
          this._errorType = 'schema';
        } else if (err.status === 413) {
          this._error = 'Your file is too large. This endpoint supports up to 50 MB synchronously. Use the v0.2 async pathway for larger imports.';
          this._errorType = 'size';
        } else {
          this._error = `Import couldn't start (HTTP ${err.status}). Check your file format and try again.`;
          this._errorType = 'generic';
        }
      } else {
        this._error = "Import couldn't start. Please check your connection and try again.";
        this._errorType = 'generic';
      }
    } finally {
      this._submitting = false;
    }
  }

  // ── Step render helpers ───────────────────────────────────────────────────

  private _renderStep1() {
    const entities: CatalogEntity[] = ['agents', 'skills', 'queues', 'channels', 'adapters', 'break_reasons'];

    return html`
      <p class="step-heading">Choose entity type</p>
      <p class="step-subheading">Select what you want to import from your CSV file.</p>

      <div class="radio-card-grid">
        ${entities.map((entity) => html`
          <label
            class="radio-card ${this._selectedEntity === entity ? 'selected' : ''}"
            data-entity="${entity}"
            @click=${() => { this._selectedEntity = entity; }}
          >
            <input
              type="radio"
              name="entity-type"
              value="${entity}"
              ?checked=${this._selectedEntity === entity}
              @change=${() => { this._selectedEntity = entity; }}
            />
            <span class="radio-card-icon">
              <uk-icon icon="${ENTITY_ICONS[entity]}" width="22" height="22"></uk-icon>
            </span>
            <span class="radio-card-label">${ENTITY_LABELS[entity]}</span>
            <span class="radio-card-check" aria-hidden="true">
              <uk-icon icon="check" width="11" height="11" style="color:white"></uk-icon>
            </span>
          </label>
        `)}
      </div>

      <div class="step-nav">
        <button
          type="button"
          class="uk-button uk-button-primary"
          data-action="step1-next"
          ?disabled=${!this._selectedEntity}
          @click=${this._stepNext}
        >
          Next: Upload file
          <uk-icon icon="chevron-right" width="14" height="14"></uk-icon>
        </button>
      </div>
    `;
  }

  private _renderStep2() {
    const entity = this._selectedEntity ?? 'agents';
    const help = CSV_FORMAT_HELP[entity];
    const formatMime = this._selectedFormat === 'csv' ? '.csv' : '.json';

    return html`
      <p class="step-heading">Upload file</p>
      <p class="step-subheading">Drag and drop or browse for your ${ENTITY_LABELS[entity]} CSV file.</p>

      <div class="schema-chip">
        <uk-icon icon="info" width="12" height="12"></uk-icon>
        schema_version=v0.1
      </div>

      <!-- Format toggle -->
      <div class="format-tabs">
        <button
          type="button"
          class="format-tab ${this._selectedFormat === 'csv' ? 'active' : ''}"
          @click=${() => {
            if (this._selectedFormat !== 'csv') {
              this._selectedFormat = 'csv';
              this._stagedFile = null;
              this._fileError = null;
            }
          }}
        >CSV</button>
        <button
          type="button"
          class="format-tab ${this._selectedFormat === 'json' ? 'active' : ''}"
          @click=${() => {
            if (this._selectedFormat !== 'json') {
              this._selectedFormat = 'json';
              this._stagedFile = null;
              this._fileError = null;
            }
          }}
        >JSON</button>
      </div>

      <!-- Drop zone or staged file -->
      ${this._stagedFile
        ? html`
          <div class="file-staged">
            <uk-icon icon="file-text" width="18" height="18" style="color:oklch(0.55 0.18 145);flex-shrink:0"></uk-icon>
            <span class="file-staged-name">${this._stagedFile.name}</span>
            <span class="file-staged-size">${this._formatFileSize(this._stagedFile.size)}</span>
            <button
              type="button"
              class="file-staged-remove"
              aria-label="Remove file"
              @click=${() => { this._stagedFile = null; this._fileError = null; }}
            >
              <uk-icon icon="x" width="16" height="16"></uk-icon>
            </button>
          </div>
        `
        : html`
          <div
            class="file-drop ${this._dragOver ? 'drag-over' : ''}"
            data-dropzone
            @dragover=${this._handleDragOver}
            @dragleave=${this._handleDragLeave}
            @drop=${this._handleDrop}
            @click=${this._handleBrowseClick}
          >
            <uk-icon icon="upload-cloud" width="40" height="40" class="file-drop-icon"></uk-icon>
            <div>
              <strong>Drag ${this._selectedFormat.toUpperCase()} here</strong> or
              <button type="button" class="file-drop-link" @click=${(e: Event) => { e.stopPropagation(); this._handleBrowseClick(); }}>browse</button>
            </div>
            <div class="file-drop-hint">CSV or JSON · Max 50 MB · Max 500 rows · BOM/CRLF handled server-side</div>
          </div>
        `
      }

      <!-- Hidden file input -->
      <input
        type="file"
        accept="${formatMime}"
        @change=${this._handleFileInputChange}
      >

      <!-- File error -->
      ${when(this._fileError, () => html`
        <div class="alert alert-danger">
          <uk-icon icon="alert-circle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>${this._fileError}</span>
        </div>
      `)}

      <!-- CSV format help -->
      ${when(this._selectedFormat === 'csv', () => html`
        <div class="csv-help" data-csv-help>
          <div class="csv-help-header">${ENTITY_LABELS[entity]} — CSV columns</div>
          <pre><strong>Required:</strong> ${help.required}
<strong>Optional:</strong> ${help.optional}${help.extra ? `\n\n${help.extra}` : ''}</pre>
        </div>
      `)}

      <!-- Idempotency -->
      <div class="idempotency-section">
        <label class="idempotency-label" @click=${this._handleIdempotencyToggle}>
          <span class="switch-wrap">
            <input type="checkbox" ?checked=${this._useIdempotency} />
            <span class="switch-track"><span class="switch-thumb"></span></span>
          </span>
          Make this import retry-safe (Idempotency-Key)
        </label>
        <div class="idempotency-sub">When enabled, retrying with the same file will return the original result.</div>
        ${when(this._useIdempotency && this._idempotencyKey, () => html`
          <div class="idempotency-key" data-idempotency-key>${this._idempotencyKey}</div>
        `)}
      </div>

      <div class="step-nav">
        <button type="button" class="uk-button uk-button-default" @click=${this._stepBack}>
          <uk-icon icon="chevron-left" width="14" height="14"></uk-icon>
          Back
        </button>
        <button
          type="button"
          class="uk-button uk-button-primary"
          data-action="step2-next"
          ?disabled=${!this._stagedFile}
          @click=${this._stepNext}
        >
          Next: Review
          <uk-icon icon="chevron-right" width="14" height="14"></uk-icon>
        </button>
      </div>
    `;
  }

  private _renderStep3() {
    const entity = this._selectedEntity ?? 'agents';
    const file = this._stagedFile ?? new File([], 'unknown');

    return html`
      <p class="step-heading">Review & start import</p>
      <p class="step-subheading">Confirm details before submitting the import job.</p>

      ${when(this._errorType === 'schema', () => html`
        <div class="alert alert-danger">
          <uk-icon icon="alert-circle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>${this._error}</span>
        </div>
      `)}
      ${when(this._errorType === 'size', () => html`
        <div class="alert alert-warning">
          <uk-icon icon="alert-triangle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>${this._error}</span>
        </div>
      `)}
      ${when(this._errorType === 'generic', () => html`
        <div class="alert alert-danger">
          <uk-icon icon="alert-circle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>${this._error}</span>
        </div>
      `)}

      <div class="review-table" data-review>
        <div class="review-row">
          <span class="review-label">Entity</span>
          <span class="review-value">${ENTITY_LABELS[entity]}</span>
        </div>
        <div class="review-row">
          <span class="review-label">File</span>
          <span class="review-value">${file.name} (${this._formatFileSize(file.size)})</span>
        </div>
        <div class="review-row">
          <span class="review-label">Format</span>
          <span class="review-value">${this._selectedFormat.toUpperCase()}</span>
        </div>
        <div class="review-row">
          <span class="review-label">Schema version</span>
          <span class="review-value">v0.1</span>
        </div>
        <div class="review-row">
          <span class="review-label">Retry-safe</span>
          <span class="review-value">
            ${this._useIdempotency && this._idempotencyKey
              ? html`Yes — key: ${this._idempotencyKey}`
              : 'No'
            }
          </span>
        </div>
      </div>

      ${when(entity === 'agents', () => html`
        <div class="review-warning">
          <uk-icon icon="alert-triangle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>Importing existing agents MERGES skills. To remove a skill, use the agent detail edit page.</span>
        </div>
      `)}

      <div class="step-nav">
        <button
          type="button"
          class="uk-button uk-button-default"
          @click=${this._stepBack}
          ?disabled=${this._submitting}
        >
          <uk-icon icon="chevron-left" width="14" height="14"></uk-icon>
          Back
        </button>
        <button
          type="button"
          class="uk-button uk-button-primary"
          ?disabled=${this._submitting}
          @click=${() => this._doImport()}
        >
          ${when(this._submitting, () => html`<span class="spinner"></span>`)}
          ${this._submitting ? 'Importing…' : 'Import'}
        </button>
      </div>
    `;
  }

  private _renderInlineResult() {
    const result = this._inlineResult!;
    const succeededCount = result.succeeded?.length ?? 0;
    const failedCount = result.failed?.length ?? 0;
    const isPartial = failedCount > 0;
    const totalCount = succeededCount + failedCount;
    const entityPath = `/orgs/${this.orgId}/${this._selectedEntity ?? 'agents'}`;

    return html`
      <div class="page-header">
        <div class="page-header-top">
          <button type="button" class="back-btn" @click=${() => this._navigate(entityPath)}>
            <uk-icon icon="chevron-left" width="18" height="18"></uk-icon>
          </button>
          <h1 class="page-title">Import Result</h1>
        </div>
        <p class="page-subtitle">Bulk import completed</p>
      </div>

      <div class="wizard-card">
        <div class="wizard-body">
          <div class="stat-grid">
            <div class="stat-card" data-stat="imported">
              <span class="stat-number stat-number--success">${succeededCount}</span>
              <span class="stat-label">Imported</span>
            </div>
            <div class="stat-card" data-stat="skipped">
              <span class="stat-number stat-number--neutral">0</span>
              <span class="stat-label">Skipped</span>
            </div>
            <div class="stat-card" data-stat="failed">
              <span class="stat-number ${isPartial ? 'stat-number--danger' : 'stat-number--neutral'}">${failedCount}</span>
              <span class="stat-label">Failed</span>
            </div>
          </div>

          ${isPartial ? html`
            <div class="alert alert-warning" style="margin-bottom:20px">
              <uk-icon icon="alert-triangle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
              <span>${failedCount} row${failedCount === 1 ? '' : 's'} failed validation. Review the failure list and re-upload after correcting.</span>
            </div>
            <div class="errors-section">
              <h3>Failed rows</h3>
              <div class="errors-table" data-errors-table>
                <div class="errors-header">
                  <span>Row</span>
                  <span>Field</span>
                  <span>Reason</span>
                </div>
                ${result.failed.map((row) => html`
                  <div class="error-row" data-error-row>
                    <span class="error-row-num">${row.row ?? '—'}</span>
                    <span class="error-field">${row.field ?? '—'}</span>
                    <span class="error-reason">${row.reason ?? '—'}</span>
                  </div>
                `)}
              </div>
            </div>
          ` : html`
            <div class="alert" style="background:color-mix(in oklch,oklch(0.65 0.18 145) 10%,transparent);border:1px solid color-mix(in oklch,oklch(0.65 0.18 145) 35%,transparent);color:oklch(0.45 0.18 145);margin-bottom:20px">
              <uk-icon icon="check-circle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
              <span>All ${totalCount} row${totalCount === 1 ? '' : 's'} imported successfully.</span>
            </div>
          `}

          <div style="margin-top:24px;display:flex;gap:8px">
            <button
              type="button"
              class="uk-button uk-button-default"
              @click=${() => {
                this._inlineResult = null;
                this._step = 1;
                this._selectedEntity = null;
                this._stagedFile = null;
                this._error = null;
                this._errorType = null;
              }}
            >
              <uk-icon icon="upload" width="14" height="14"></uk-icon>
              Start another import
            </button>
            <button
              type="button"
              class="uk-button uk-button-default"
              @click=${() => this._navigate(entityPath)}
            >
              Back to Catalog
            </button>
          </div>
        </div>
      </div>
    `;
  }

  override render() {
    if (this._inlineResult) {
      return this._renderInlineResult();
    }

    const backPath = `/orgs/${this.orgId}/${this._selectedEntity ?? 'agents'}`;

    return html`
      <div class="page-header">
        <div class="page-header-top">
          <button type="button" class="back-btn" @click=${() => this._navigate(backPath)}>
            <uk-icon icon="chevron-left" width="18" height="18"></uk-icon>
          </button>
          <h1 class="page-title">Bulk Import</h1>
        </div>
        <p class="page-subtitle">Import agents, skills, queues, and more from CSV</p>
      </div>

      <div class="wizard-card">
        <or-form-wizard
          .steps=${WIZARD_STEPS}
          .currentStep=${this._wizardStep}
          hide-nav
        >
          <div slot="step-pick">
            <div class="wizard-body">
              ${this._renderStep1()}
            </div>
          </div>
          <div slot="step-upload">
            <div class="wizard-body">
              ${this._renderStep2()}
            </div>
          </div>
          <div slot="step-review">
            <div class="wizard-body">
              ${this._renderStep3()}
            </div>
          </div>
        </or-form-wizard>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-import-page': OrImportPage;
  }
}
