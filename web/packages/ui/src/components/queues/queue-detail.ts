// Phase 6 Plan 08 Task 2: <or-queue-detail> — Queue entity detail/edit page.
// D6-03: 409 body comes from error.current — no re-GET (Pitfall 9).
// D04_1-02: code field is ALWAYS read-only after create.
// D6-V-40: delete confirm requires typing exact queue.name (case-sensitive).
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// W0.1-13: redesigned to Ember healthcare-dashboard style (no sl-* in shadow).

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateQueue from '../../validators/UpdateQueueRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/conflict-banner.js';

type Queue = components['schemas']['Queue'];
type ChannelType = 'voice' | 'chat' | 'email';

type QueueForm = {
  name: string;
  external_id: string;
  channel_types: ChannelType[];
  priority: number;
  acw_sec: number;
  enabled: boolean;
};

/**
 * <or-queue-detail> — Queue detail / edit page (Ember healthcare-dashboard style).
 *
 * Page-header + section cards layout.
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the queue UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-queue-detail')
export class OrQueueDetail extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 20px 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 16px;
      gap: 16px;
    }

    .page-header-left {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      min-width: 0;
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
      margin-top: 2px;
      flex-shrink: 0;
      transition: color .12s, background .12s;
    }

    .back-btn:hover {
      color: var(--foreground);
      background: var(--muted);
    }

    .page-title {
      font-size: 22px;
      font-weight: 700;
      margin: 0 0 2px;
      color: var(--foreground);
    }

    .page-subtitle {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0;
    }

    .page-header-right {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-shrink: 0;
    }

    /* ── Status badge ────────────────────────────────────────────── */
    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 4px 12px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 600;
      flex-shrink: 0;
    }

    .status-badge--active {
      background: color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent);
      color: oklch(0.45 0.18 145);
    }

    .status-badge--disabled {
      background: var(--muted);
      color: var(--muted-foreground);
    }

    /* ── Stats row ───────────────────────────────────────────────── */
    .stats-row {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 10px;
      margin-bottom: 14px;
    }

    @media (max-width: 900px) {
      .stats-row {
        grid-template-columns: repeat(2, 1fr);
      }
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px 14px;
      box-shadow: var(--shadow-xs);
    }

    .stat-label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .04em;
      color: var(--muted-foreground);
      margin-bottom: 4px;
    }

    .stat-value {
      font-size: 18px;
      font-weight: 700;
      color: var(--foreground);
    }

    .stat-sub {
      font-size: 11px;
      color: var(--muted-foreground);
      margin-top: 2px;
    }

    /* ── Info cards ──────────────────────────────────────────────── */
    .info-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px 20px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 12px;
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 12px;
      display: flex;
      align-items: center;
      gap: 7px;
    }

    .card-title uk-icon {
      color: var(--muted-foreground);
    }

    /* ── Two-column grid ─────────────────────────────────────────── */
    .two-col-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10px 20px;
    }

    @media (max-width: 640px) {
      .two-col-grid { grid-template-columns: 1fr; }
    }

    .form-group { margin-bottom: 0; }

    .field-label {
      display: block;
      font-size: 12px;
      font-weight: 600;
      color: var(--muted-foreground);
      margin-bottom: 5px;
    }

    .field-error {
      font-size: 12px;
      color: var(--destructive);
      margin-top: 4px;
    }

    .field-help {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 4px;
    }

    /* ── Channel type pill checkboxes ────────────────────────────── */
    .channel-group {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
    }

    .channel-chip {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 5px 12px;
      border: 1px solid var(--border);
      border-radius: 9999px;
      font-size: 13px;
      font-weight: 500;
      color: var(--foreground);
      background: var(--card);
      cursor: pointer;
      transition: border-color .12s, background .12s, color .12s;
      user-select: none;
    }

    .channel-chip:has(input:checked) {
      border-color: var(--primary);
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
    }

    .channel-chip input {
      position: absolute;
      opacity: 0;
      width: 0;
      height: 0;
    }

    /* ── Toggle row ──────────────────────────────────────────────── */
    .toggle-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 0;
    }

    .toggle-row + .toggle-row {
      border-top: 1px solid var(--border);
    }

    .toggle-label {
      font-size: 14px;
      font-weight: 500;
      color: var(--foreground);
    }

    .toggle-sub {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 1px;
    }

    /* Frankenstyle-style checkbox toggle */
    .uk-toggle {
      position: relative;
      display: inline-block;
      width: 40px;
      height: 22px;
      flex-shrink: 0;
    }

    .uk-toggle input { opacity: 0; width: 0; height: 0; }

    .uk-toggle-slider {
      position: absolute;
      inset: 0;
      background: var(--muted);
      border-radius: 9999px;
      cursor: pointer;
      transition: background .15s;
    }

    .uk-toggle-slider::before {
      content: '';
      position: absolute;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: white;
      left: 3px;
      top: 3px;
      transition: transform .15s;
      box-shadow: 0 1px 3px rgba(0,0,0,.2);
    }

    .uk-toggle input:checked + .uk-toggle-slider {
      background: var(--primary);
    }

    .uk-toggle input:checked + .uk-toggle-slider::before {
      transform: translateX(18px);
    }

    /* ── Alerts ──────────────────────────────────────────────────── */
    .alert {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 12px;
    }

    .alert--danger {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent);
      color: var(--destructive);
    }

    /* ── Footer meta ─────────────────────────────────────────────── */
    .footer-meta {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 12px;
      padding-top: 10px;
      border-top: 1px solid var(--border);
    }

    .bottom-bar {
      display: flex;
      gap: 8px;
      margin-top: 14px;
      justify-content: flex-end;
    }

    /* ── Loading spinner ─────────────────────────────────────────── */
    .spinner {
      display: inline-block;
      width: 20px;
      height: 20px;
      border: 2px solid var(--border);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin .6s linear infinite;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    .loading-wrap {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 40px 0;
      color: var(--muted-foreground);
      font-size: 14px;
    }

    /* ── Delete confirm overlay ──────────────────────────────────── */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: var(--shadow-overlay, oklch(0 0 0 / 0.5));
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }

    .confirm-panel {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 24px;
      max-width: 480px;
      width: 90%;
      box-shadow: var(--shadow-lg);
    }

    .confirm-title {
      margin: 0 0 10px;
      font-size: 18px;
      font-weight: 600;
      color: var(--foreground);
    }

    .confirm-body {
      margin: 0 0 16px;
      font-size: 14px;
      color: var(--muted-foreground);
    }

    .action-row {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }

    /* ── Toast ───────────────────────────────────────────────────── */
    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: var(--or-z-toast, 9000);
    }

    .toast {
      display: flex;
      align-items: center;
      gap: 8px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px 16px;
      box-shadow: var(--shadow-md);
      font-size: 14px;
      color: var(--foreground);
    }

    .toast uk-icon { color: oklch(0.45 0.18 145); }

    /* ── Icon button ─────────────────────────────────────────────── */
    .icon-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 4px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: color .12s, background .12s;
    }

    .icon-btn--danger:hover {
      color: var(--destructive);
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _entity: Queue | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() private _dirty = false;
  @state() private _conflictServer: Record<string, unknown> | null = null;
  @state() private _fieldErrors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';

  private _savedToastTimeout?: ReturnType<typeof setTimeout>;
  private _onKeydown?: (e: KeyboardEvent) => void;
  private _formData: QueueForm = {
    name: '',
    external_id: '',
    channel_types: [],
    priority: 0,
    acw_sec: 0,
    enabled: true,
  };

  // --- Computed ---
  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  // --- Lifecycle ---

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadEntity();
    this._onKeydown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && this._deleteConfirmOpen) {
        this._deleteConfirmOpen = false;
        this._deleteConfirmName = '';
      }
    };
    document.addEventListener('keydown', this._onKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._onKeydown) document.removeEventListener('keydown', this._onKeydown);
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('_deleteConfirmOpen') && this._deleteConfirmOpen) {
      void this.updateComplete.then(() => {
        const input = this.shadowRoot?.querySelector('.confirm-panel input') as HTMLElement | null;
        if (input) input.focus();
      });
    }
  }

  // --- Private methods ---

  private async _loadEntity(): Promise<void> {
    if (!this.orgId || !this.entityId) return;
    this._loading = true;
    try {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/queues/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load queue';
        return;
      }
      const queue = data as Queue;
      this._entity = queue;
      this._formData = {
        name: queue.name ?? '',
        external_id: queue.external_id ?? '',
        channel_types: (queue.channel_types ?? []) as ChannelType[],
        priority: queue.priority ?? 0,
        acw_sec: queue.acw_sec ?? 0,
        enabled: queue.enabled ?? true,
      };
      this._dirty = false;
    } finally {
      this._loading = false;
    }
  }

  private _markDirty(): void {
    this._dirty = true;
    this._conflictServer = null;
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

  private _showSaved(): void {
    this._showSavedToast = true;
    clearTimeout(this._savedToastTimeout);
    this._savedToastTimeout = setTimeout(() => {
      this._showSavedToast = false;
    }, 3000);
  }

  private _relativeTime(iso: string): string {
    const time = new Date(iso).getTime();
    if (Number.isNaN(time)) return iso;
    const ms = Date.now() - time;
    const min = Math.floor(ms / 60000);
    if (min < 1) return 'just now';
    if (min < 60) return `${min}m ago`;
    const h = Math.floor(min / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
  }

  private _toggleChannelType(type: ChannelType): void {
    const current = this._formData.channel_types;
    const next = current.includes(type)
      ? current.filter((t) => t !== type)
      : [...current, type];
    this._formData = { ...this._formData, channel_types: next };
    this._markDirty();
    if (this._fieldErrors['channel_types']) {
      this._fieldErrors = { ...this._fieldErrors, channel_types: '' };
    }
  }

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    const body = {
      name: this._formData.name,
      external_id: this._formData.external_id,
      channel_types: this._formData.channel_types,
      priority: this._formData.priority,
      acw_sec: this._formData.acw_sec,
      enabled: this._formData.enabled,
      version: this._entity.version,
    };

    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateUpdateQueue as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(body)) {
      const errors: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errors[field] = err.message ?? 'Invalid value';
      }
      this._fieldErrors = errors;
      return;
    }
    this._fieldErrors = {};

    this._saving = true;
    try {
      const result = await this.client.PATCH('/v1/orgs/{org_id}/queues/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: Queue | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (Pitfall 9: never call response.json())
        // [Rule 1 - Bug] Update _entity from error.current so re-submit uses the correct version.
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Queue }).current;
          this._conflictServer = current as unknown as Record<string, unknown>;
          this._entity = current;
          return;
        }
        // 422 immutable_field
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'immutable_field'
        ) {
          this._apiError = 'Code cannot be changed after create.';
          return;
        }
        // 422 invalid_value
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'invalid_value'
        ) {
          const errObj = error as { field?: string; reason?: string };
          if (errObj.field) {
            this._fieldErrors = { [errObj.field]: errObj.reason ?? 'Invalid value' };
          } else {
            this._apiError = errObj.reason ?? 'Invalid value';
          }
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Save failed';
        return;
      }

      if (data) {
        this._entity = data;
        this._formData = {
          name: data.name ?? '',
          external_id: data.external_id ?? '',
          channel_types: (data.channel_types ?? []) as ChannelType[],
          priority: data.priority ?? 0,
          acw_sec: data.acw_sec ?? 0,
          enabled: data.enabled ?? true,
        };
        this._dirty = false;
        this._conflictServer = null;
        this._showSaved();
      }
    } finally {
      this._saving = false;
    }
  }

  // --- Enable / Disable ---

  async _handleEnable(): Promise<void> {
    if (!this._entity) return;
    const result = await this.client.PATCH('/v1/orgs/{org_id}/queues/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body: { enabled: true, version: this._entity.version },
    } as never);
    const { data, error } = result as { data: Queue | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const result = await this.client.PATCH('/v1/orgs/{org_id}/queues/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body: { enabled: false, version: this._entity.version },
    } as never);
    const { data, error } = result as { data: Queue | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: false };
    }
  }

  // --- Delete ---

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/queues/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/queues`);
    } else {
      this._deleteConfirmOpen = false;
      this._deleteConfirmName = '';
      this._apiError = (error as { reason?: string })?.reason ?? 'Delete failed. The queue may be in use.';
    }
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard') {
      if (this._entity) {
        this._formData = {
          name: this._entity.name ?? '',
          external_id: this._entity.external_id ?? '',
          channel_types: (this._entity.channel_types ?? []) as ChannelType[],
          priority: this._entity.priority ?? 0,
          acw_sec: this._entity.acw_sec ?? 0,
          enabled: this._entity.enabled ?? true,
        };
        this._dirty = false;
      }
    }
    this._conflictServer = null;
  }

  // --- Render helpers ---

  private _renderPageHeader() {
    const entity = this._entity;
    const statusBadge = entity
      ? entity.enabled
        ? html`<span class="status-badge status-badge--active">Active</span>`
        : html`<span class="status-badge status-badge--disabled">Disabled</span>`
      : nothing;

    return html`
      <div class="page-header">
        <div class="page-header-left">
          <button
            class="back-btn"
            title="Back to Queues"
            @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
          >
            <uk-icon icon="arrow-left" width="18" height="18"></uk-icon>
          </button>
          <div>
            <h1 class="page-title">${entity?.name ?? 'Queue Detail'}</h1>
            ${entity ? html`<p class="page-subtitle">${entity.code}</p>` : nothing}
          </div>
        </div>

        <div class="page-header-right">
          ${statusBadge}

          ${entity
            ? html`
                ${entity.enabled
                  ? html`<button class="uk-button uk-button-default uk-button-small" @click=${this._handleDisable}>Disable</button>`
                  : html`<button class="uk-button uk-button-default uk-button-small" @click=${this._handleEnable}>Enable</button>`}

                <button
                  class="uk-button uk-button-danger uk-button-small"
                  @click=${() => {
                    this._deleteConfirmOpen = true;
                    this._deleteConfirmName = '';
                  }}
                >
                  <uk-icon icon="trash-2" width="14" height="14"></uk-icon>
                  Delete
                </button>
              `
            : nothing}
        </div>
      </div>
    `;
  }

  private _renderStatsRow() {
    const entity = this._entity;
    if (!entity) return nothing;

    return html`
      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-label">Priority</div>
          <div class="stat-value">${entity.priority ?? 0}</div>
          <div class="stat-sub">routing order</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">ACW</div>
          <div class="stat-value">${entity.acw_sec ?? 0}s</div>
          <div class="stat-sub">after-call work</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Channels</div>
          <div class="stat-value">${(entity.channel_types ?? []).length}</div>
          <div class="stat-sub">active types</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Last Updated</div>
          <div class="stat-value" style="font-size:14px;padding-top:4px">${this._relativeTime(entity.updated_at ?? '')}</div>
          <div class="stat-sub">${entity.updated_at ? new Date(entity.updated_at).toLocaleDateString() : ''}</div>
        </div>
      </div>
    `;
  }

  private _renderConflictBanner() {
    if (!this._conflictServer) return nothing;
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._formData as unknown as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
    `;
  }

  private _renderIdentityCard() {
    const entity = this._entity;
    if (!entity) return nothing;

    return html`
      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="inbox" width="15" height="15"></uk-icon>
          Identity
        </h2>

        ${this._apiError
          ? html`<div class="alert alert--danger" style="margin-bottom:16px">
              <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
              ${this._apiError}
            </div>`
          : nothing}

        <div class="form-group" style="margin-bottom:16px">
          <or-code-input
            .value=${entity.code}
            .readonly=${true}
          ></or-code-input>
        </div>

        <div class="two-col-grid">
          <div class="form-group">
            <label class="field-label" for="queue-name">Name</label>
            <input
              id="queue-name"
              class="uk-input"
              type="text"
              .value=${this._formData.name}
              required
              @input=${(e: Event) => {
                this._formData = { ...this._formData, name: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['name']
              ? html`<div class="field-error">${this._fieldErrors['name']}</div>`
              : nothing}
          </div>

          <div class="form-group">
            <label class="field-label" for="queue-ext-id">External ID</label>
            <input
              id="queue-ext-id"
              class="uk-input"
              type="text"
              .value=${this._formData.external_id}
              placeholder="Optional integration reference"
              @input=${(e: Event) => {
                this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            <div class="field-help">Pass empty string to clear.</div>
          </div>

          <div class="form-group">
            <label class="field-label" for="queue-priority">Priority</label>
            <input
              id="queue-priority"
              class="uk-input"
              type="number"
              min="0"
              step="1"
              .value=${String(this._formData.priority)}
              required
              @input=${(e: Event) => {
                const v = parseInt((e.target as HTMLInputElement).value, 10);
                this._formData = { ...this._formData, priority: isNaN(v) ? 0 : v };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['priority']
              ? html`<div class="field-error">${this._fieldErrors['priority']}</div>`
              : nothing}
          </div>

          <div class="form-group">
            <label class="field-label" for="queue-acw">After-Call Work (s)</label>
            <input
              id="queue-acw"
              class="uk-input"
              type="number"
              min="0"
              step="1"
              .value=${String(this._formData.acw_sec)}
              required
              @input=${(e: Event) => {
                const v = parseInt((e.target as HTMLInputElement).value, 10);
                this._formData = { ...this._formData, acw_sec: isNaN(v) ? 0 : v };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['acw_sec']
              ? html`<div class="field-error">${this._fieldErrors['acw_sec']}</div>`
              : nothing}
          </div>
        </div>

        <div style="margin-top:16px">
          <label class="field-label">Channel Types</label>
          <div class="channel-group">
            ${(['voice', 'chat', 'email'] as ChannelType[]).map(
              (ct) => html`
                <label class="channel-chip">
                  <input
                    type="checkbox"
                    ?checked=${this._formData.channel_types.includes(ct)}
                    @change=${() => this._toggleChannelType(ct)}
                  />
                  <uk-icon
                    icon=${ct === 'voice' ? 'phone' : ct === 'chat' ? 'message-circle' : 'mail'}
                    width="13"
                    height="13"
                  ></uk-icon>
                  ${ct}
                </label>
              `
            )}
          </div>
          ${this._fieldErrors['channel_types']
            ? html`<div class="field-error">${this._fieldErrors['channel_types']}</div>`
            : nothing}
        </div>

        <div class="footer-meta">
          version ${entity.version}
          · updated ${this._relativeTime(entity.updated_at ?? '')}
          · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
        </div>
      </div>
    `;
  }

  private _renderStatusCard() {
    if (!this._entity) return nothing;

    return html`
      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="plug" width="15" height="15"></uk-icon>
          Status &amp; Routing
        </h2>

        <div class="toggle-row">
          <div>
            <div class="toggle-label">Enabled</div>
            <div class="toggle-sub">Disabled queues do not receive new interactions</div>
          </div>
          <label class="uk-toggle">
            <input
              type="checkbox"
              ?checked=${this._formData.enabled}
              @change=${(e: Event) => {
                this._formData = { ...this._formData, enabled: (e.target as HTMLInputElement).checked };
                this._markDirty();
              }}
            />
            <span class="uk-toggle-slider"></span>
          </label>
        </div>
      </div>
    `;
  }

  private _renderFormActions() {
    if (!this._entity) return nothing;
    const entity = this._entity;

    return html`
      <div class="bottom-bar">
        <button
          class="uk-button uk-button-default"
          @click=${() => {
            if (!this._dirty) {
              this._navigate(`/orgs/${this.orgId}/queues`);
            } else {
              this._formData = {
                name: entity.name ?? '',
                external_id: entity.external_id ?? '',
                channel_types: (entity.channel_types ?? []) as ChannelType[],
                priority: entity.priority ?? 0,
                acw_sec: entity.acw_sec ?? 0,
                enabled: entity.enabled ?? true,
              };
              this._dirty = false;
            }
          }}
        >Cancel</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${!this._dirty || this._saving}
          @click=${this._handleSave}
        >
          ${this._saving
            ? html`<span class="spinner" style="width:14px;height:14px;border-width:2px;margin-right:6px"></span> Saving…`
            : 'Save changes'}
        </button>
      </div>
    `;
  }

  private _renderDeleteDialog() {
    return when(this._deleteConfirmOpen, () => html`
      <div
        class="confirm-overlay"
        role="presentation"
        @click=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}
      >
        <div
          class="confirm-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="confirm-title"
          @click=${(e: Event) => e.stopPropagation()}
        >
          <h3 id="confirm-title" class="confirm-title">Delete queue "${this._entity?.name ?? ''}"?</h3>
          <p class="confirm-body">This is permanent and cannot be undone. Type the queue name to confirm.</p>
          <div style="margin-bottom: 20px;">
            <label class="field-label" for="delete-confirm-input">Queue name</label>
            <input
              id="delete-confirm-input"
              class="uk-input"
              type="text"
              placeholder="Type queue name to confirm"
              .value=${this._deleteConfirmName}
              aria-label="Type queue name to confirm deletion"
              @input=${(e: Event) => {
                this._deleteConfirmName = (e.target as HTMLInputElement).value;
              }}
            />
          </div>
          <div class="action-row">
            <button
              class="uk-button uk-button-default uk-button-small"
              @click=${() => {
                this._deleteConfirmOpen = false;
                this._deleteConfirmName = '';
              }}
            >Cancel</button>
            <button
              class="uk-button uk-button-danger uk-button-small"
              ?disabled=${!this._canDelete}
              @click=${this._handleDelete}
            >Delete</button>
          </div>
        </div>
      </div>
    `);
  }

  override render() {
    if (this._loading) {
      return html`
        <div class="loading-wrap">
          <span class="spinner"></span>
          Loading queue…
        </div>
      `;
    }

    if (!this._entity && this._apiError) {
      return html`
        <div class="alert alert--danger" style="margin:24px 0">
          <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
          ${this._apiError}
        </div>
      `;
    }

    return html`
      ${this._renderPageHeader()}
      ${this._renderStatsRow()}
      ${this._renderConflictBanner()}
      ${this._renderIdentityCard()}
      ${this._renderStatusCard()}
      ${this._renderFormActions()}
      ${this._renderDeleteDialog()}

      ${when(
        this._showSavedToast,
        () => html`
          <div class="toast-container">
            <div class="toast">
              <uk-icon icon="check" width="16" height="16"></uk-icon>
              Saved
            </div>
          </div>
        `
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-detail': OrQueueDetail;
  }
}
