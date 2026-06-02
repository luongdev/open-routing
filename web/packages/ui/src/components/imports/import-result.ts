// Phase 6 Plan 13 Task 2: <or-import-result> — import job result display.
//
// Renders the result of a bulk import job retrieved from GET /v1/orgs/{orgId}/imports/{importId}.
// No polling (D6-27 — import is synchronous in v0.1).
//
// Status banner: success (green) / partial (amber) / failure (red) based on outcome.
// Stat cards: Imported / Skipped / Failed (D6-V-21).
// Failed rows table: Row | Field | Reason columns.
// Download failures button: ALWAYS disabled, v0.2 deferred (D6-V-22 / IMP-11).
//
// ADMIN-04: uses this.client.GET — no direct fetch calls.
// W0.1-23: redesigned to Ember bulk-import-wizard style (no sl-* in shadow).

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

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
 * <or-import-result> — Ember-style display of a completed bulk import job.
 *
 * Fetches GET /v1/orgs/{orgId}/imports/{importId} on mount via @lit/task.
 * No polling — bulk import is synchronous in v0.1 (D6-27).
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
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      margin-bottom: 24px;
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

    /* ── Status banner ───────────────────────────────────────────── */
    .banner {
      border-radius: 10px;
      padding: 16px 20px;
      margin-bottom: 20px;
      display: flex;
      align-items: flex-start;
      gap: 10px;
    }

    .banner-success {
      background: color-mix(in oklch, oklch(0.65 0.18 145) 10%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.65 0.18 145) 35%, transparent);
      color: oklch(0.45 0.18 145);
    }

    .banner-partial {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 12%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.75 0.18 80) 40%, transparent);
      color: oklch(0.5 0.18 80);
    }

    .banner-failure {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
    }

    .banner-icon { flex-shrink: 0; margin-top: 1px; }

    .banner-title {
      font-weight: 700;
      font-size: 15px;
    }

    .banner-sub {
      font-size: 13px;
      margin-top: 3px;
      opacity: .85;
    }

    /* ── Stat grid ───────────────────────────────────────────────── */
    .stat-grid {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 12px;
      margin-bottom: 24px;
    }

    @media (max-width: 480px) {
      .stat-grid { grid-template-columns: repeat(2, 1fr); }
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

    /* ── Metadata card ───────────────────────────────────────────── */
    .info-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 20px;
      overflow: hidden;
    }

    .info-card-title {
      font-size: 13px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: .06em;
      color: var(--muted-foreground);
      padding: 12px 16px;
      border-bottom: 1px solid var(--border);
      background: var(--muted);
    }

    .metadata-row {
      display: flex;
      padding: 11px 16px;
      border-bottom: 1px solid var(--border);
      font-size: 14px;
      align-items: center;
      gap: 12px;
    }

    .metadata-row:last-child { border-bottom: none; }

    .metadata-label {
      width: 130px;
      flex-shrink: 0;
      font-weight: 500;
      font-size: 13px;
      color: var(--muted-foreground);
    }

    .metadata-value {
      color: var(--foreground);
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .copy-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 3px;
      border-radius: 4px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      transition: color .12s;
    }

    .copy-btn:hover { color: var(--foreground); }

    /* ── Failures section ────────────────────────────────────────── */
    .failures-section {
      margin-bottom: 20px;
    }

    .section-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
    }

    .section-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0;
    }

    .errors-table {
      border: 1px solid var(--border);
      border-radius: 10px;
      overflow: hidden;
    }

    .errors-header {
      display: grid;
      grid-template-columns: 56px 140px 1fr;
      background: var(--muted);
      padding: 8px 14px;
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: .06em;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }

    .errors-body {
      max-height: 420px;
      overflow-y: auto;
    }

    .error-row {
      display: grid;
      grid-template-columns: 56px 140px 1fr;
      padding: 9px 14px;
      font-size: 13px;
      border-bottom: 1px solid var(--border);
    }

    .error-row:last-child { border-bottom: none; }

    .error-row-num {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--muted-foreground);
      text-align: right;
    }

    .error-field {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--foreground);
      padding-left: 8px;
    }

    .error-reason {
      color: var(--destructive);
      padding-left: 8px;
    }

    /* ── Succeeded disclosure ────────────────────────────────────── */
    .succeeded-disclosure {
      border: 1px solid var(--border);
      border-radius: 10px;
      overflow: hidden;
      margin-bottom: 20px;
    }

    .succeeded-disclosure summary {
      padding: 12px 16px;
      cursor: pointer;
      font-size: 14px;
      font-weight: 500;
      list-style: none;
      background: var(--card);
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--foreground);
    }

    .succeeded-disclosure summary::-webkit-details-marker,
    .succeeded-disclosure summary::marker { display: none; }

    .succeeded-ids-list {
      padding: 12px 16px;
      font-family: var(--uk-font-monospace, monospace);
      font-size: 12px;
      line-height: 1.6;
      max-height: 200px;
      overflow-y: auto;
      color: var(--foreground);
      background: var(--muted);
      border-top: 1px solid var(--border);
    }

    .replay-notice {
      padding: 12px 16px;
      font-size: 13px;
      color: oklch(0.5 0.18 80);
      background: color-mix(in oklch, oklch(0.75 0.18 80) 10%, transparent);
      border-top: 1px solid var(--border);
    }

    /* ── Loading / empty states ─────────────────────────────────── */
    .loading {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 80px 32px;
      gap: 12px;
      color: var(--muted-foreground);
    }

    .loading-spinner {
      width: 32px;
      height: 32px;
      border: 3px solid var(--border);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin .7s linear infinite;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    .empty-state {
      text-align: center;
      padding: 80px 32px;
      color: var(--muted-foreground);
    }

    .empty-state-icon {
      color: var(--muted-foreground);
      margin-bottom: 16px;
    }

    .empty-state-title {
      font-size: 17px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 8px;
    }

    .empty-state p { font-size: 14px; margin: 0 0 20px; }

    /* ── Error alert ─────────────────────────────────────────────── */
    .alert-danger {
      display: flex;
      gap: 8px;
      align-items: flex-start;
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
      border-radius: 8px;
      padding: 12px 14px;
      font-size: 13px;
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'import-id' }) accessor importId = '';
  /** Typed API client from shell */
  @property({ attribute: false }) accessor client: ApiClient | null = null;

  /** Override for idempotent replay display — injected via test or by parent */
  @state() accessor _idempotentReplay = false;

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
      const d = new Date(iso);
      return isNaN(d.getTime()) ? iso : d.toLocaleString();
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
          <uk-icon icon="check-circle" width="18" height="18" class="banner-icon"></uk-icon>
          <div>
            <div class="banner-title">Import complete</div>
            <div class="banner-sub">${total_rows} ${entityLabel} records imported successfully.</div>
          </div>
        </div>
      `;
    } else if (succeeded_rows === 0) {
      return html`
        <div class="banner banner-failure" data-banner="failure">
          <uk-icon icon="x-circle" width="18" height="18" class="banner-icon"></uk-icon>
          <div>
            <div class="banner-title">Import failed</div>
            <div class="banner-sub">All ${total_rows} rows rejected. See details below.</div>
          </div>
        </div>
      `;
    } else {
      return html`
        <div class="banner banner-partial" data-banner="partial">
          <uk-icon icon="alert-triangle" width="18" height="18" class="banner-icon"></uk-icon>
          <div>
            <div class="banner-title">Import partially complete</div>
            <div class="banner-sub">Imported ${succeeded_rows} of ${total_rows} ${entityLabel}. ${failed_rows} rows failed.</div>
          </div>
        </div>
      `;
    }
  }

  private _renderStatCards(job: ImportJob) {
    return html`
      <div class="stat-grid">
        <div class="stat-card" data-stat="imported">
          <span class="stat-number stat-number--success">${job.succeeded_rows}</span>
          <span class="stat-label">Imported</span>
        </div>
        <div class="stat-card" data-stat="skipped">
          <span class="stat-number stat-number--neutral">0</span>
          <span class="stat-label">Skipped</span>
        </div>
        <div class="stat-card" data-stat="failed">
          <span class="stat-number ${job.failed_rows > 0 ? 'stat-number--danger' : 'stat-number--neutral'}">${job.failed_rows}</span>
          <span class="stat-label">Failed</span>
        </div>
      </div>
    `;
  }

  private _renderMetadata(job: ImportJob) {
    const entityLabel = ENTITY_LABELS[job.entity_type] ?? job.entity_type;

    return html`
      <div class="info-card">
        <div class="info-card-title">Job Details</div>
        <div class="metadata-row">
          <span class="metadata-label">Entity</span>
          <span class="metadata-value">${entityLabel}</span>
        </div>
        <div class="metadata-row">
          <span class="metadata-label">Job ID</span>
          <span class="metadata-value">
            ${job.id}
            <button
              type="button"
              class="copy-btn"
              aria-label="Copy job ID"
              @click=${() => navigator.clipboard?.writeText(job.id)}
            >
              <uk-icon icon="copy" width="13" height="13"></uk-icon>
            </button>
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
        <div class="section-header">
          <h2 class="section-title">Failed rows</h2>
          <button
            type="button"
            class="uk-button uk-button-default uk-button-small"
            disabled
            data-action="download-failures"
            title="Coming in v0.2 — track failure download in IMP-11"
            style="opacity:.5;cursor:not-allowed"
          >
            <uk-icon icon="download" width="13" height="13"></uk-icon>
            Download failures CSV
          </button>
        </div>
        <div class="errors-table" data-failures-table>
          <div class="errors-header">
            <span>Row</span>
            <span>Field</span>
            <span>Reason</span>
          </div>
          <div class="errors-body">
            ${errors.map((row) => html`
              <div class="error-row" data-failure-row>
                <span class="error-row-num">${row.row ?? '—'}</span>
                <span class="error-field">${row.field ?? '—'}</span>
                <span class="error-reason">${row.reason}</span>
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
          <uk-icon icon="chevron-right" width="14" height="14"></uk-icon>
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
              <em style="color:var(--muted-foreground)">(succeeded IDs available via server logs in v0.1)</em>
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
          <div class="loading-spinner"></div>
          <p>Loading import result…</p>
        </div>
      `,

      complete: (job) => {
        if (!job) {
          return html`
            <div class="empty-state" data-empty="not-found">
              <div class="empty-state-icon">
                <uk-icon icon="search" width="40" height="40"></uk-icon>
              </div>
              <div class="empty-state-title">Import not found in this org.</div>
              <p>The import job may have been deleted or may belong to another org.</p>
              <button
                type="button"
                class="uk-button uk-button-primary"
                @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
              >
                Back to Agents
              </button>
            </div>
          `;
        }

        const errors = (job.errors ?? []) as BulkImportFailedRow[];
        const entityListPath = `/orgs/${this.orgId}/${job.entity_type ?? 'agents'}`;

        return html`
          <div class="page-header">
            <div class="page-header-top">
              <button type="button" class="back-btn" @click=${() => this._navigate(entityListPath)}>
                <uk-icon icon="chevron-left" width="18" height="18"></uk-icon>
              </button>
              <h1 class="page-title">Import Result</h1>
            </div>
            <p class="page-subtitle">
              ${ENTITY_LABELS[job.entity_type] ?? job.entity_type} · ${this._formatDate(job.created_at)}
            </p>
          </div>

          ${this._renderBanner(job)}
          ${this._renderStatCards(job)}
          ${this._renderMetadata(job)}
          ${when(errors.length > 0, () => this._renderFailuresTable(errors)!)}
          ${when(
            errors.length === 0,
            () => html`
              <div style="margin-bottom:20px">
                <button
                  type="button"
                  class="uk-button uk-button-default uk-button-small"
                  disabled
                  data-action="download-failures"
                  title="Coming in v0.2 — track failure download in IMP-11"
                  style="opacity:.5;cursor:not-allowed"
                >
                  <uk-icon icon="download" width="13" height="13"></uk-icon>
                  Download failures CSV
                </button>
              </div>
            `
          )}
          ${this._renderSucceededDisclosure(job)}
        `;
      },

      error: (e) => html`
        <div class="alert-danger">
          <uk-icon icon="alert-circle" width="16" height="16" style="flex-shrink:0;margin-top:1px"></uk-icon>
          <span>Failed to load import result. ${String(e)}</span>
        </div>
      `,
    });
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-import-result': OrImportResult;
  }
}
