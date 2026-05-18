// Phase 6 Plan 13 Task 1: <or-import-page> — 3-step bulk import wizard.
//
// Step 1: Pick entity (6 entities: agents/skills/queues/channels/adapters/break_reasons)
// Step 2: Upload file (drag-drop zone + browse; CSV/JSON toggle; format help per entity)
// Step 3: Review + submit via createImporter() from ../../api/import.js
//
// ADMIN-04: all HTTP is via createImporter() only. No direct fetch calls permitted.
// D6-08: Per-component Shoelace imports for tree-shaking.
// D6-13: UUIDv7 idempotency key generated via crypto.randomUUID().

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { createImporter, ImportError } from '../../api/import.js';
import type { CatalogEntity, ImportCatalogArgs, BulkImportResult } from '../../api/import.js';
import type { ApiClient } from '../../api/client.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/radio-group/radio-group.js';
import '@shoelace-style/shoelace/dist/components/radio/radio.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/checkbox/checkbox.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/card/card.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/badge/badge.js';

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
  { key: 'pick', label: 'Pick' },
  { key: 'upload', label: 'Upload' },
  { key: 'review', label: 'Review' },
];

/**
 * <or-import-page> — 3-step bulk import wizard.
 *
 * Step 1: Entity picker (6 radio options)
 * Step 2: File upload (drag-drop zone, format toggle, CSV help, idempotency)
 * Step 3: Review + submit
 *
 * All HTTP calls are via createImporter() — ADMIN-04 forbids direct fetch calls.
 *
 * Dispatches 'open-routing:navigate' on successful import (200/207/422 all navigate).
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
      max-width: 720px;
      margin: 0 auto;
    }

    h1 {
      font-size: 20px;
      font-weight: 600;
      margin: 0 0 24px;
      color: var(--or-color-text-strong, #1a1a1a);
    }

    h2 {
      font-size: 16px;
      font-weight: 600;
      margin: 0 0 16px;
      color: var(--or-color-text-strong, #1a1a1a);
    }

    /* Step 1: Entity picker */
    .entity-grid {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: 12px;
      margin: 16px 0;
    }

    .entity-card {
      border: 2px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      padding: 16px;
      cursor: pointer;
      background: transparent;
      text-align: left;
      font-size: 14px;
      color: var(--or-color-text-body, #404040);
      transition: border-color 0.15s ease, background 0.15s ease;
    }

    .entity-card:hover {
      border-color: var(--sl-color-primary-300, #7ec8d0);
      background: var(--sl-color-primary-50, #f0fafb);
    }

    .entity-card.selected {
      border-color: var(--sl-color-primary-500, #2b8a93);
      background: var(--sl-color-primary-50, #f0fafb);
    }

    .entity-card-label {
      font-weight: 600;
      font-size: 15px;
      display: block;
    }

    /* Step 2: Upload */
    .format-toggle {
      margin-bottom: 16px;
    }

    .drop-zone {
      border: 2px dashed var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      padding: 48px 32px;
      text-align: center;
      color: var(--or-color-text-muted, #737373);
      font-size: 14px;
      transition: border-color 0.15s ease, background 0.15s ease;
      cursor: default;
      margin-bottom: 16px;
    }

    .drop-zone.drag-over {
      border-color: var(--sl-color-primary-500, #2b8a93);
      background: var(--sl-color-primary-50, #f0fafb);
    }

    .drop-zone-staged {
      border: 2px solid var(--sl-color-success-500, #198754);
      border-radius: var(--or-radius-md, 8px);
      padding: 16px;
      background: var(--sl-color-success-50, #f0fdf4);
      margin-bottom: 16px;
      font-size: 13px;
    }

    .browse-link {
      color: var(--sl-color-primary-600, #237880);
      text-decoration: underline;
      cursor: pointer;
      background: none;
      border: none;
      font-size: inherit;
      font-family: inherit;
      padding: 0;
    }

    .csv-help {
      background: var(--or-color-code-bg, #f5f5f5);
      padding: 16px;
      border-radius: var(--or-radius-md, 8px);
      font-family: monospace;
      font-size: 13px;
      margin-bottom: 16px;
      white-space: pre-wrap;
      color: var(--or-color-code-fg, #1a575f);
    }

    .csv-help-header {
      font-weight: 600;
      margin-bottom: 8px;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--or-color-text-muted, #737373);
      font-family: sans-serif;
    }

    .schema-chip {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      background: var(--sl-color-primary-100, #d6f1f4);
      color: var(--sl-color-primary-700, #1a6a72);
      border-radius: 12px;
      padding: 2px 10px;
      font-size: 12px;
      font-weight: 600;
      margin-bottom: 12px;
    }

    /* Step 3: Review */
    .review-table {
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      overflow: hidden;
      margin-bottom: 16px;
    }

    .review-row {
      display: flex;
      padding: 10px 16px;
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      font-size: 14px;
    }

    .review-row:last-child {
      border-bottom: none;
    }

    .review-label {
      width: 140px;
      flex-shrink: 0;
      font-weight: 500;
      color: var(--or-color-text-muted, #737373);
    }

    .review-value {
      color: var(--or-color-text-body, #404040);
      font-family: monospace;
      font-size: 13px;
    }

    .review-warning {
      background: var(--sl-color-warning-50, #fff8ec);
      border: 1px solid var(--sl-color-warning-300, #f5c842);
      border-radius: var(--or-radius-md, 8px);
      padding: 12px 16px;
      font-size: 13px;
      margin-bottom: 16px;
      color: var(--sl-color-warning-800, #6b4800);
    }

    /* Navigation */
    .step-nav {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 24px;
      padding-top: 16px;
      border-top: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .idempotency-section {
      margin-top: 16px;
      padding: 12px 16px;
      background: var(--sl-color-neutral-50, #fafafa);
      border-radius: var(--or-radius-md, 8px);
      border: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .idempotency-key-display {
      font-family: monospace;
      font-size: 12px;
      color: var(--or-color-code-fg, #1a575f);
      background: var(--or-color-code-bg, #f5f5f5);
      padding: 4px 8px;
      border-radius: 4px;
      margin-top: 4px;
      word-break: break-all;
    }

    input[type="file"] {
      display: none;
    }

    sl-alert {
      margin-bottom: 16px;
    }
  `;

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
      return 'File too large — maximum is 50 MB / 500 rows. Use the v0.2 async pathway for larger imports.';
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
  }

  private _handleIdempotencyChange(e: Event): void {
    const cb = e.target as HTMLInputElement;
    this._useIdempotency = cb.checked;
    if (this._useIdempotency && !this._idempotencyKey) {
      this._idempotencyKey = crypto.randomUUID();
    }
    if (!this._useIdempotency) {
      this._idempotencyKey = null;
    }
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
          this._error = 'Your file is too large. v0.1 supports up to 50 MB / 500 rows synchronously. See v0.2 async pathway docs for larger imports.';
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
      <h2>What are you importing?</h2>
      <div class="entity-grid">
        ${entities.map((entity) => html`
          <button
            class="entity-card ${this._selectedEntity === entity ? 'selected' : ''}"
            data-entity="${entity}"
            @click=${() => { this._selectedEntity = entity; }}
            aria-pressed="${this._selectedEntity === entity ? 'true' : 'false'}"
          >
            <span class="entity-card-label">${ENTITY_LABELS[entity]}</span>
          </button>
        `)}
      </div>
      <div class="step-nav">
        <sl-button
          variant="primary"
          data-action="step1-next"
          ?disabled=${!this._selectedEntity}
          @click=${this._stepNext}
        >
          Next: Upload →
        </sl-button>
      </div>
    `;
  }

  private _renderStep2() {
    const entity = this._selectedEntity ?? 'agents';
    const help = CSV_FORMAT_HELP[entity];
    const formatMime = this._selectedFormat === 'csv' ? '.csv' : '.json';

    return html`
      <h2>Upload file</h2>

      <div class="schema-chip">
        <sl-icon name="info-circle"></sl-icon>
        schema_version=v0.1
      </div>

      <!-- Format toggle -->
      <div class="format-toggle">
        <sl-radio-group label="Format" value="${this._selectedFormat}" @sl-change=${(e: CustomEvent) => {
          this._selectedFormat = (e.target as HTMLInputElement).value as 'csv' | 'json';
          this._stagedFile = null;
          this._fileError = null;
        }}>
          <sl-radio value="csv">CSV</sl-radio>
          <sl-radio value="json">JSON</sl-radio>
        </sl-radio-group>
      </div>

      <!-- Drop zone or staged file -->
      ${this._stagedFile
        ? html`
          <div class="drop-zone-staged">
            <sl-icon name="file-earmark-check" style="color: var(--sl-color-success-600); margin-right: 8px;"></sl-icon>
            <strong>${this._stagedFile.name}</strong>
            — ${this._formatFileSize(this._stagedFile.size)}
            <sl-icon-button
              name="x"
              label="Remove file"
              @click=${() => { this._stagedFile = null; this._fileError = null; }}
              style="float: right;"
            ></sl-icon-button>
          </div>
        `
        : html`
          <div
            class="drop-zone ${this._dragOver ? 'drag-over' : ''}"
            data-dropzone
            @dragover=${this._handleDragOver}
            @dragleave=${this._handleDragLeave}
            @drop=${this._handleDrop}
          >
            <sl-icon name="cloud-upload" style="font-size: 32px; display: block; margin: 0 auto 12px;"></sl-icon>
            Drop a ${this._selectedFormat.toUpperCase()} file here, or
            <button class="browse-link" @click=${this._handleBrowseClick}>Browse</button>
            <br>
            <small>Max 50 MB / 500 rows</small>
          </div>
        `
      }

      <!-- Hidden file input -->
      <input
        type="file"
        accept="${formatMime}"
        @change=${this._handleFileInputChange}
      >

      <!-- File error alert -->
      ${when(this._fileError, () => html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          ${this._fileError}
        </sl-alert>
      `)}

      <!-- CSV format help -->
      ${when(this._selectedFormat === 'csv', () => html`
        <div class="csv-help" data-csv-help>
          <div class="csv-help-header">${ENTITY_LABELS[entity]} — CSV columns</div>
          <div><strong>Required:</strong> ${help.required}</div>
          <div><strong>Optional:</strong> ${help.optional}</div>
          ${help.extra ? html`<div style="margin-top: 8px; white-space: pre-wrap;">${help.extra}</div>` : null}
        </div>
      `)}

      <!-- Idempotency checkbox -->
      <div class="idempotency-section">
        <sl-checkbox
          data-idempotency-checkbox
          ?checked=${this._useIdempotency}
          @sl-change=${this._handleIdempotencyChange}
        >
          Make this import retry-safe (Idempotency-Key)
        </sl-checkbox>
        <div style="font-size: 12px; color: var(--or-color-text-muted); margin-top: 4px;">
          When checked, retrying with the same file will return the original result.
        </div>
        ${when(this._useIdempotency && this._idempotencyKey, () => html`
          <div class="idempotency-key-display">${this._idempotencyKey}</div>
        `)}
      </div>

      <div class="step-nav">
        <sl-button variant="default" @click=${this._stepBack}>← Back</sl-button>
        <sl-button
          variant="primary"
          data-action="step2-next"
          ?disabled=${!this._stagedFile}
          @click=${this._stepNext}
        >
          Next: Review →
        </sl-button>
      </div>
    `;
  }

  private _renderStep3() {
    const entity = this._selectedEntity ?? 'agents';
    const file = this._stagedFile ?? new File([], 'unknown');

    return html`
      <h2>Review & start import</h2>

      <!-- Submit error alerts -->
      ${when(this._errorType === 'schema', () => html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          ${this._error}
        </sl-alert>
      `)}
      ${when(this._errorType === 'size', () => html`
        <sl-alert variant="warning" open>
          <sl-icon slot="icon" name="exclamation-triangle"></sl-icon>
          ${this._error}
        </sl-alert>
      `)}
      ${when(this._errorType === 'generic', () => html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="x-circle"></sl-icon>
          ${this._error}
        </sl-alert>
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
          <sl-icon name="exclamation-triangle" style="margin-right: 6px;"></sl-icon>
          Importing existing agents MERGES skills. To remove a skill, use the agent detail edit page.
        </div>
      `)}

      <div class="step-nav">
        <sl-button variant="default" @click=${this._stepBack} ?disabled=${this._submitting}>
          ← Back
        </sl-button>
        <sl-button
          variant="primary"
          ?loading=${this._submitting}
          ?disabled=${this._submitting}
          @click=${() => this._doImport()}
        >
          ${this._submitting ? html`<sl-spinner></sl-spinner> Starting…` : 'Start import'}
        </sl-button>
      </div>
    `;
  }

  override render() {
    if (this._inlineResult) {
      // Inline result fallback (Codex HIGH ship-fix): the server's
      // BulkImportResult does not yet include import_id, so we cannot navigate
      // to /imports/{id}. Render a minimal success summary inline instead.
      // Once Phase 5's contract grows a real import_id we swap back to
      // navigation + or-import-result for the full historical view.
      const succeededCount = this._inlineResult.succeeded?.length ?? 0;
      const failedCount = this._inlineResult.failed?.length ?? 0;
      const isPartial = failedCount > 0;
      return html`
        <h1>Bulk Import — Result</h1>
        <div style="display:flex;gap:24px;margin:24px 0">
          <sl-card>
            <strong style="font-size:24px;color:var(--sl-color-success-500)">${succeededCount}</strong>
            <div>Succeeded</div>
          </sl-card>
          <sl-card>
            <strong style="font-size:24px;color:${isPartial ? 'var(--sl-color-danger-500)' : 'var(--or-color-text-muted)'}">${failedCount}</strong>
            <div>Failed</div>
          </sl-card>
        </div>
        ${isPartial ? html`
          <sl-alert variant="warning" open style="margin-bottom:24px">
            ${failedCount} row${failedCount === 1 ? '' : 's'} failed validation. Review the failure list below and re-upload after correcting.
          </sl-alert>
          <table style="width:100%;border-collapse:collapse">
            <thead>
              <tr><th align="left">Row</th><th align="left">Field</th><th align="left">Reason</th></tr>
            </thead>
            <tbody>
              ${this._inlineResult.failed.map(
                (row) => html`
                  <tr style="border-top:1px solid var(--or-color-divider)">
                    <td>${row.row ?? '—'}</td>
                    <td>${row.field ?? '—'}</td>
                    <td>${row.reason ?? '—'}</td>
                  </tr>
                `,
              )}
            </tbody>
          </table>
        ` : html`
          <sl-alert variant="success" open style="margin-bottom:24px">
            All ${succeededCount} row${succeededCount === 1 ? '' : 's'} imported successfully.
          </sl-alert>
        `}
        <div style="margin-top:24px">
          <sl-button @click=${() => { this._inlineResult = null; this._step = 1; this._selectedEntity = null; this._stagedFile = null; }}>
            Start another import
          </sl-button>
        </div>
      `;
    }
    return html`
      <h1>Bulk Import</h1>

      <or-form-wizard
        .steps=${WIZARD_STEPS}
        .currentStep=${this._wizardStep}
        hide-nav
      >
        <div slot="step-pick">
          ${this._renderStep1()}
        </div>
        <div slot="step-upload">
          ${this._renderStep2()}
        </div>
        <div slot="step-review">
          ${this._renderStep3()}
        </div>
      </or-form-wizard>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-import-page': OrImportPage;
  }
}
