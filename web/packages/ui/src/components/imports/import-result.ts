// Phase 6 Plan 13 Task 2: <or-import-result> — import job result display.
//
// Renders the result of a bulk import job retrieved from GET /v1/orgs/{orgId}/imports/{importId}.
// No polling (D6-27 — import is synchronous in v0.1).
//
// Status banner: success (green) / partial (amber) / failure (red) based on outcome.
// Stat cards: Succeeded / Failed / Total (D6-V-21).
// Failed rows table: Row | Field | Reason columns.
// Download failures button: ALWAYS disabled, v0.2 deferred (D6-V-22 / IMP-11).
//
// ADMIN-04: uses this.client.GET — no direct fetch calls.
// D6-08: Per-component Shoelace imports for tree-shaking.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/badge/badge.js';

type ImportJob = components['schemas']['ImportJob'];
type BulkImportFailedRow = components['schemas']['BulkImportFailedRow'];

/** Entity type label mapping */
const ENTITY_LABELS: Record<string, string> = {
  agents: 'Agents',
  skills: 'Skills',
  queues: 'Queues',
  channels: 'Channels',
  adapters: 'Adapters',
  break_reasons: 'Break Reasons',
};

/**
 * <or-import-result> — displays the result of a completed bulk import job.
 *
 * Fetches GET /v1/orgs/{orgId}/imports/{importId} on mount via @lit/task.
 * No polling — bulk import is synchronous in v0.1 (D6-27).
 *
 * Renders:
 *   - Status banner: success / partial / failure based on succeeded_rows vs total_rows
 *   - Summary stat cards: Succeeded / Failed / Total (D6-V-21)
 *   - Job metadata: Entity, Started, Schema, Job ID
 *   - Failed rows table: Row | Field | Reason (omitted if no failures)
 *   - Download failures button: always disabled (D6-V-22 / IMP-11 deferred to v0.2)
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — org UUID from URL
 *   - importId: (attribute 'import-id') — import job UUID from URL
 *   - client: (no attribute) — typed API client from shell
 */
@customElement('or-import-result')
export class OrImportResult extends LitElement {
  static override styles = css`
    :host {
      display: block;
      max-width: 900px;
      margin: 0 auto;
    }

    h1 {
      font-size: 20px;
      font-weight: 600;
      margin: 0 0 24px;
      color: var(--or-color-text-strong, #1a1a1a);
    }

    /* Top bar */
    .top-bar {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 24px;
    }

    /* Status banners */
    .banner {
      border-radius: var(--or-radius-md, 8px);
      padding: 16px 20px;
      margin-bottom: 20px;
      font-size: 14px;
    }

    .banner-success {
      background: rgba(25, 135, 84, 0.05);
      border: 1px solid var(--sl-color-success-300, #6fcf97);
      color: var(--sl-color-success-700, #0f5132);
    }

    .banner-partial {
      background: rgba(255, 193, 7, 0.05);
      border: 1px solid var(--sl-color-warning-300, #ffd965);
      color: var(--sl-color-warning-800, #6b4800);
    }

    .banner-failure {
      background: rgba(220, 53, 69, 0.05);
      border: 1px solid var(--sl-color-danger-300, #f5c6cb);
      color: var(--sl-color-danger-700, #842029);
    }

    .banner-title {
      font-weight: 600;
      font-size: 16px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .banner-sub {
      margin-top: 4px;
      font-size: 13px;
    }

    /* Stat cards */
    .stat-cards {
      display: flex;
      gap: 16px;
      margin-bottom: 24px;
    }

    .stat-card {
      flex: 1;
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      padding: 16px;
      text-align: center;
    }

    .stat-number {
      font-size: 24px;
      font-weight: 600;
      display: block;
    }

    .stat-number.success {
      color: var(--sl-color-success-600, #198754);
    }

    .stat-number.danger {
      color: var(--sl-color-danger-600, #dc3545);
    }

    .stat-number.neutral {
      color: var(--or-color-text-strong, #1a1a1a);
    }

    .stat-label {
      font-size: 14px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 4px;
      display: block;
    }

    /* Job metadata */
    .metadata-table {
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      overflow: hidden;
      margin-bottom: 20px;
    }

    .metadata-row {
      display: flex;
      padding: 10px 16px;
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      font-size: 14px;
    }

    .metadata-row:last-child {
      border-bottom: none;
    }

    .metadata-label {
      width: 140px;
      flex-shrink: 0;
      font-weight: 500;
      color: var(--or-color-text-muted, #737373);
    }

    .metadata-value {
      color: var(--or-color-text-body, #404040);
      font-family: monospace;
      font-size: 13px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    /* Failed rows table */
    .failures-section {
      margin-bottom: 20px;
    }

    .failures-section h2 {
      font-size: 15px;
      font-weight: 600;
      margin: 0 0 12px;
      color: var(--or-color-text-strong, #1a1a1a);
    }

    .failures-table {
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      overflow: hidden;
    }

    .failures-header {
      display: grid;
      grid-template-columns: 64px 140px 1fr;
      background: var(--sl-color-neutral-50, #fafafa);
      padding: 8px 16px;
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--or-color-text-muted, #737373);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .failures-body {
      max-height: 400px;
      overflow-y: auto;
    }

    .failure-row {
      display: grid;
      grid-template-columns: 64px 140px 1fr;
      padding: 8px 16px;
      font-size: 13px;
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .failure-row:last-child {
      border-bottom: none;
    }

    .failure-row-num {
      font-family: monospace;
      text-align: right;
      color: var(--or-color-text-muted, #737373);
    }

    .failure-field {
      font-family: monospace;
      color: var(--or-color-code-fg, #1a575f);
      padding-left: 8px;
    }

    .failure-reason {
      color: var(--sl-color-danger-700, #842029);
      padding-left: 8px;
    }

    /* Download button + toolbar */
    .result-toolbar {
      display: flex;
      gap: 8px;
      margin-bottom: 20px;
    }

    /* Successful IDs disclosure */
    .succeeded-disclosure {
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      overflow: hidden;
      margin-bottom: 20px;
    }

    .succeeded-disclosure summary {
      padding: 12px 16px;
      cursor: pointer;
      font-size: 14px;
      font-weight: 500;
      list-style: none;
      background: var(--sl-color-neutral-50, #fafafa);
    }

    .succeeded-disclosure summary::marker,
    .succeeded-disclosure summary::-webkit-details-marker {
      display: none;
    }

    .succeeded-ids-list {
      padding: 12px 16px;
      font-family: monospace;
      font-size: 12px;
      line-height: 1.6;
      max-height: 200px;
      overflow-y: auto;
      color: var(--or-color-code-fg, #1a575f);
      background: var(--or-color-code-bg, #f5f5f5);
    }

    .replay-notice {
      padding: 12px 16px;
      font-size: 13px;
      color: var(--sl-color-warning-700, #6b4800);
      background: var(--sl-color-warning-50, #fff8ec);
    }

    /* Empty state */
    .empty-state {
      text-align: center;
      padding: 64px 32px;
      color: var(--or-color-text-muted, #737373);
    }

    .empty-state-title {
      font-size: 16px;
      font-weight: 500;
      color: var(--or-color-text-strong, #1a1a1a);
      margin-bottom: 8px;
    }

    /* Loading */
    .loading {
      text-align: center;
      padding: 64px 32px;
      color: var(--or-color-text-muted, #737373);
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'import-id' }) importId = '';
  /** Typed API client from shell */
  @property({ attribute: false }) client: ApiClient | null = null;

  /** Override for idempotent replay display — injected via test or by parent */
  @state() _idempotentReplay = false;

  private _loadTask = new Task<[ApiClient | null, string, string], ImportJob | null>(
    this,
    async ([client, orgId, importId]) => {
      if (!client || !orgId || !importId) return null;

      const resp = await (client as ApiClient).GET(
        '/v1/orgs/{org_id}/imports/{import_id}' as never,
        { params: { path: { org_id: orgId, import_id: importId } } } as never
      );

      if (resp.error) {
        const status = (resp as { response?: { status?: number } }).response?.status ?? 0;
        if (status === 404) {
          return null; // Will be rendered as not-found
        }
        throw new Error('Failed to load import result');
      }

      return (resp.data ?? null) as ImportJob | null;
    },
    () => [this.client, this.orgId, this.importId]
  );

  private _formatDate(iso: string): string {
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
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

  private _renderBanner(job: ImportJob) {
    const { succeeded_rows, failed_rows, total_rows, entity_type } = job;
    const entityLabel = ENTITY_LABELS[entity_type] ?? entity_type;

    if (failed_rows === 0 && succeeded_rows >= total_rows) {
      return html`
        <div class="banner banner-success" data-banner="success">
          <div class="banner-title">
            <sl-icon name="check-circle-fill"></sl-icon>
            Import complete — ${total_rows} records imported
          </div>
        </div>
      `;
    } else if (succeeded_rows === 0) {
      return html`
        <div class="banner banner-failure" data-banner="failure">
          <div class="banner-title">
            <sl-icon name="x-circle-fill"></sl-icon>
            Import failed — ${total_rows} rows rejected
          </div>
          <div class="banner-sub">All rows failed validation. See details below.</div>
        </div>
      `;
    } else {
      return html`
        <div class="banner banner-partial" data-banner="partial">
          <div class="banner-title">
            <sl-icon name="exclamation-triangle-fill"></sl-icon>
            Import partially complete
          </div>
          <div class="banner-sub">
            Imported ${succeeded_rows} of ${total_rows} ${entityLabel}. ${failed_rows} rows failed.
          </div>
        </div>
      `;
    }
  }

  private _renderStatCards(job: ImportJob) {
    return html`
      <div class="stat-cards">
        <div class="stat-card" data-stat="succeeded">
          <span class="stat-number success">${job.succeeded_rows}</span>
          <span class="stat-label">Succeeded</span>
        </div>
        <div class="stat-card" data-stat="failed">
          <span class="stat-number ${job.failed_rows > 0 ? 'danger' : 'neutral'}">${job.failed_rows}</span>
          <span class="stat-label">Failed</span>
        </div>
        <div class="stat-card" data-stat="total">
          <span class="stat-number neutral">${job.total_rows}</span>
          <span class="stat-label">Total</span>
        </div>
      </div>
    `;
  }

  private _renderMetadata(job: ImportJob) {
    const entityLabel = ENTITY_LABELS[job.entity_type] ?? job.entity_type;

    return html`
      <div class="metadata-table">
        <div class="metadata-row">
          <span class="metadata-label">Entity</span>
          <span class="metadata-value">${entityLabel}</span>
        </div>
        <div class="metadata-row">
          <span class="metadata-label">Job ID</span>
          <span class="metadata-value">
            ${job.id}
            <sl-icon-button
              name="clipboard"
              label="Copy job ID"
              @click=${() => navigator.clipboard?.writeText(job.id)}
            ></sl-icon-button>
          </span>
        </div>
        <div class="metadata-row">
          <span class="metadata-label">Started</span>
          <span class="metadata-value">${this._formatDate(job.created_at)}</span>
        </div>
        <div class="metadata-row">
          <span class="metadata-label">Status</span>
          <span class="metadata-value">${job.status}</span>
        </div>
      </div>
    `;
  }

  private _renderFailuresTable(errors: BulkImportFailedRow[]) {
    if (!errors || errors.length === 0) return null;

    return html`
      <div class="failures-section">
        <h2>Failed rows</h2>
        <div class="result-toolbar">
          <sl-tooltip
            content="Coming in v0.2 — track failure download in IMP-11"
            data-download-tooltip
          >
            <sl-button
              variant="default"
              size="small"
              disabled data-action="download-failures"
              style="opacity: 0.5; cursor: not-allowed;"
            >
              <sl-icon slot="prefix" name="download"></sl-icon>
              Download failures CSV
            </sl-button>
          </sl-tooltip>
        </div>
        <div class="failures-table" data-failures-table>
          <div class="failures-header">
            <span>Row</span>
            <span>Field</span>
            <span>Reason</span>
          </div>
          <div class="failures-body">
            ${errors.map((row) => html`
              <div class="failure-row" data-failure-row>
                <span class="failure-row-num">${row.row ?? '—'}</span>
                <span class="failure-field">${row.field ?? '—'}</span>
                <span class="failure-reason">${row.reason}</span>
              </div>
            `)}
          </div>
        </div>
      </div>
    `;
  }

  private _renderSucceededDisclosure(job: ImportJob) {
    if (job.succeeded_rows === 0) return null;

    return html`
      <details class="succeeded-disclosure" data-succeeded-disclosure>
        <summary>
          <sl-icon name="chevron-right"></sl-icon>
          ${job.succeeded_rows} succeeded — expand to view IDs
        </summary>
        ${when(
          this._idempotentReplay,
          () => html`
            <div class="replay-notice">
              Successful IDs not stored on idempotent replay. See server logs with
              Idempotency-Key for forensics.
            </div>
          `,
          () => html`
            <div class="succeeded-ids-list">
              ${job.id}
              <br>
              <em style="color: var(--or-color-text-muted);">(succeeded IDs available via server logs in v0.1)</em>
            </div>
          `
        )}
      </details>
    `;
  }

  override render() {
    return this._loadTask.render({
      pending: () => html`
        <div class="loading">
          <sl-spinner style="font-size: 24px;"></sl-spinner>
          <p>Loading import result…</p>
        </div>
      `,

      complete: (job) => {
        if (!job) {
          return html`
            <div class="empty-state" data-empty="not-found">
              <sl-icon name="search" style="font-size: 32px; margin-bottom: 12px; color: var(--or-color-text-muted);"></sl-icon>
              <div class="empty-state-title">Import not found in this org.</div>
              <p>The import job may have been deleted or may belong to another org.</p>
              <sl-button
                variant="primary"
                @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
              >
                Back to Catalog
              </sl-button>
            </div>
          `;
        }

        const errors = (job.errors ?? []) as BulkImportFailedRow[];

        return html`
          <div class="top-bar">
            <sl-button
              variant="default"
              size="small"
              @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
            >
              <sl-icon slot="prefix" name="arrow-left"></sl-icon>
              Back to Catalog
            </sl-button>
          </div>

          <h1>Import Result</h1>

          ${this._renderBanner(job)}
          ${this._renderStatCards(job)}
          ${this._renderMetadata(job)}
          ${when(errors.length > 0, () => this._renderFailuresTable(errors)!)}
          ${when(
            errors.length === 0,
            () => html`
              <div class="result-toolbar">
                <sl-tooltip
                  content="Coming in v0.2 — track failure download in IMP-11"
                  data-download-tooltip
                >
                  <sl-button
                    variant="default"
                    size="small"
                    disabled
                    data-action="download-failures"
                    style="opacity: 0.5; cursor: not-allowed;"
                  >
                    <sl-icon slot="prefix" name="download"></sl-icon>
                    Download failures CSV
                  </sl-button>
                </sl-tooltip>
              </div>
            `
          )}
          ${this._renderSucceededDisclosure(job)}
        `;
      },

      error: (e) => html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          Failed to load import result. ${String(e)}
        </sl-alert>
      `,
    });
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-import-result': OrImportResult;
  }
}
