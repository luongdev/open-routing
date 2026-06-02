// Phase 6 Plan 11 Task 2: <or-adapter-detail> — Adapter entity detail/edit page.
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// Pitfall 9: never call response.json() — openapi-fetch parses error body for us.
// T-06-11-01: config display uses Lit html template literals (auto-escape); no innerHTML with config values.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateAdapter from '../../validators/UpdateAdapterRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/conflict-banner.js';

type Adapter = components['schemas']['Adapter'];

type Form = {
  name: string;
  external_id: string;
  adapter_type: string;
  configText: string;
  enabled: boolean;
};

@customElement('or-adapter-detail')
export class OrAdapterDetail extends LitElement {
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
      grid-template-columns: repeat(3, 1fr);
      gap: 10px;
      margin-bottom: 14px;
    }

    @media (max-width: 640px) {
      .stats-row { grid-template-columns: 1fr 1fr; }
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

    /* ── Toggle row ──────────────────────────────────────────────── */
    .toggle-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 0;
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

    /* ── Config textarea ─────────────────────────────────────────── */
    .config-textarea {
      display: block;
      width: 100%;
      box-sizing: border-box;
      min-height: 160px;
      padding: 8px 12px;
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
      line-height: 1.5;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--foreground);
      resize: vertical;
      transition: border-color .12s, box-shadow .12s;
    }

    .config-textarea:focus,
    .config-textarea:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 25%, transparent);
    }

    .config-textarea::placeholder {
      color: var(--muted-foreground);
    }

    .config-textarea--error {
      border-color: var(--destructive);
    }

    /* ── Alert banners ───────────────────────────────────────────── */
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

    /* ── Loading / spinner ───────────────────────────────────────── */
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

    /* ── Delete confirm panel ────────────────────────────────────── */
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
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _entity: Adapter | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() private _dirty = false;
  @state() private _conflictServer: Record<string, unknown> | null = null;
  @state() private _fieldErrors: Record<string, string> = {};
  @state() private _configError = '';
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';

  private _form: Form = {
    name: '',
    external_id: '',
    adapter_type: '',
    configText: '',
    enabled: true,
  };
  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

  // Alias for tests: _formData -> _form
  get _formData() { return this._form; }
  set _formData(v: Form) { this._form = v; }

  // --- Computed ---
  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  // --- Lifecycle ---
  private _onKeydown?: (e: KeyboardEvent) => void;

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
    if (this._onKeydown) {
      document.removeEventListener('keydown', this._onKeydown);
    }
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('_deleteConfirmOpen')) {
      if (this._deleteConfirmOpen) {
        this.setAttribute('aria-live', 'polite');
        void this.updateComplete.then(() => {
          const input = this.shadowRoot?.querySelector('.confirm-panel input') as HTMLElement | null;
          if (input) input.focus();
        });
      } else {
        this.removeAttribute('aria-live');
      }
    }
  }

  // --- Private methods ---

  private async _loadEntity(): Promise<void> {
    if (!this.orgId || !this.entityId) return;
    this._loading = true;
    try {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/adapters/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load adapter';
        return;
      }
      const adapter = data as Adapter;
      this._entity = adapter;
      this._form = {
        name: adapter.name ?? '',
        external_id: (adapter as Record<string, unknown>)['external_id'] as string ?? '',
        adapter_type: (adapter as Record<string, unknown>)['adapter_type'] as string ?? '',
        configText: adapter.config != null
          ? JSON.stringify(adapter.config, null, 2)
          : '',
        enabled: adapter.enabled ?? true,
      };
      this._dirty = false;
      this._configError = '';
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
    try {
      const ms = Date.now() - new Date(iso).getTime();
      const min = Math.floor(ms / 60000);
      if (min < 1) return 'just now';
      if (min < 60) return `${min}m ago`;
      const h = Math.floor(min / 60);
      if (h < 24) return `${h}h ago`;
      return `${Math.floor(h / 24)}d ago`;
    } catch { return iso; }
  }

  // --- Config JSON validation ---

  private _handleConfigInput(e: Event): void {
    const value = (e.target as HTMLTextAreaElement).value;
    this._form = { ...this._form, configText: value };
    if (value.trim() !== '') {
      try {
        JSON.parse(value);
        this._configError = '';
      } catch {
        this._configError = 'Config must be valid JSON';
      }
    } else {
      this._configError = '';
    }
    this._markDirty();
  }

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    if (this._configError) return;

    let configPayload: Record<string, unknown> | null = null;
    if (this._form.configText.trim() !== '') {
      try {
        configPayload = JSON.parse(this._form.configText) as Record<string, unknown>;
      } catch {
        this._configError = 'Config must be valid JSON';
        return;
      }
    } else {
      configPayload = null;
    }

    const body = {
      name: this._form.name,
      external_id: this._form.external_id,
      adapter_type: this._form.adapter_type,
      config: configPayload,
      enabled: this._form.enabled,
      version: this._entity.version,
    };

    const validateFn = validateUpdateAdapter as unknown as {
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
      const result = await this.client.PATCH('/v1/orgs/{org_id}/adapters/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: Adapter | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (D6-03 + Codex review HIGH)
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Adapter }).current;
          this._conflictServer = current as unknown as Record<string, unknown>;
          this._entity = current;
          return;
        }
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'immutable_field'
        ) {
          this._apiError = 'Code cannot be changed after create.';
          return;
        }
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
        this._form = {
          name: data.name ?? '',
          external_id: (data as Record<string, unknown>)['external_id'] as string ?? '',
          adapter_type: (data as Record<string, unknown>)['adapter_type'] as string ?? '',
          configText: data.config != null ? JSON.stringify(data.config, null, 2) : '',
          enabled: data.enabled ?? true,
        };
        this._dirty = false;
        this._conflictServer = null;
        this._configError = '';
        this._showSaved();
      }
    } finally {
      this._saving = false;
    }
  }

  // --- Enable / Disable ---

  async _handleEnable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: true, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/adapters/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Adapter | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: false, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/adapters/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Adapter | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: false };
    }
  }

  // --- Delete ---

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/adapters/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/adapters`);
    }
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard') {
      if (this._entity) {
        this._form = {
          name: this._entity.name ?? '',
          external_id: (this._entity as Record<string, unknown>)['external_id'] as string ?? '',
          adapter_type: (this._entity as Record<string, unknown>)['adapter_type'] as string ?? '',
          configText: this._entity.config != null ? JSON.stringify(this._entity.config, null, 2) : '',
          enabled: this._entity.enabled ?? true,
        };
        this._dirty = false;
        this._configError = '';
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
            title="Back to Adapters"
            @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
          >
            <uk-icon icon="arrow-left" width="18" height="18"></uk-icon>
          </button>
          <div>
            <h1 class="page-title">${entity?.name ?? 'Adapter Detail'}</h1>
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
          <div class="stat-label">Status</div>
          <div class="stat-value" style="font-size:14px;padding-top:4px">${entity.enabled ? 'Active' : 'Disabled'}</div>
          <div class="stat-sub">current</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Version</div>
          <div class="stat-value">${entity.version}</div>
          <div class="stat-sub">lock rev</div>
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

    // For config JSONB: show pretty-printed JSON in conflict banner (UI-SPEC §5.7)
    const serverForBanner = { ...this._conflictServer };
    if (serverForBanner['config'] !== null && serverForBanner['config'] !== undefined) {
      serverForBanner['config'] = JSON.stringify(serverForBanner['config'], null, 2);
    }

    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${serverForBanner}
        .userValue=${this._form as unknown as Record<string, unknown>}
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
          <uk-icon icon="plug" width="15" height="15"></uk-icon>
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
            <label class="field-label" for="adapter-name">Name</label>
            <input
              id="adapter-name"
              class="uk-input"
              type="text"
              .value=${this._form.name}
              required
              @input=${(e: Event) => {
                this._form = { ...this._form, name: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['name']
              ? html`<div class="field-error">${this._fieldErrors['name']}</div>`
              : nothing}
          </div>

          <div class="form-group">
            <label class="field-label" for="adapter-ext-id">External ID</label>
            <input
              id="adapter-ext-id"
              class="uk-input"
              type="text"
              .value=${this._form.external_id}
              @input=${(e: Event) => {
                this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
          </div>

          <div class="form-group">
            <label class="field-label" for="adapter-type">Adapter Type</label>
            <input
              id="adapter-type"
              class="uk-input"
              type="text"
              .value=${this._form.adapter_type}
              required
              placeholder="e.g. freeswitch, livekit, twilio"
              @input=${(e: Event) => {
                this._form = { ...this._form, adapter_type: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['adapter_type']
              ? html`<div class="field-error">${this._fieldErrors['adapter_type']}</div>`
              : nothing}
          </div>
        </div>

        <div class="form-group" style="margin-top:12px">
          <label class="field-label" for="adapter-config">Config (JSON)</label>
          <textarea
            id="adapter-config"
            class="config-textarea ${this._configError ? 'config-textarea--error' : ''}"
            rows="8"
            placeholder='{"key": "value"}'
            .value=${this._form.configText}
            @input=${this._handleConfigInput}
          ></textarea>
          ${this._configError
            ? html`<div class="field-error">${this._configError}</div>`
            : nothing}
          <div class="field-help">Optional JSON configuration blob. Clear to set null.</div>
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
          <uk-icon icon="settings" width="15" height="15"></uk-icon>
          Status &amp; Routing
        </h2>

        <div class="toggle-row">
          <div>
            <div class="toggle-label">Enabled</div>
            <div class="toggle-sub">Adapter accepts routing traffic when enabled</div>
          </div>
          <label class="uk-toggle">
            <input
              type="checkbox"
              ?checked=${this._form.enabled}
              @change=${(e: Event) => {
                this._form = { ...this._form, enabled: (e.target as HTMLInputElement).checked };
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
              this._navigate(`/orgs/${this.orgId}/adapters`);
            } else {
              this._form = {
                name: entity.name ?? '',
                external_id: (entity as Record<string, unknown>)['external_id'] as string ?? '',
                adapter_type: (entity as Record<string, unknown>)['adapter_type'] as string ?? '',
                configText: entity.config != null ? JSON.stringify(entity.config, null, 2) : '',
                enabled: entity.enabled ?? true,
              };
              this._dirty = false;
              this._configError = '';
            }
          }}
        >Cancel</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${!this._dirty || this._saving || !!this._configError}
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
          <h3 id="confirm-title" class="confirm-title">Delete adapter "${this._entity?.name ?? ''}"?</h3>
          <p class="confirm-body">This is permanent and cannot be undone. Type the adapter name to confirm.</p>
          <div style="margin-bottom: 20px;">
            <label class="field-label" for="delete-confirm-input">Adapter name</label>
            <input
              id="delete-confirm-input"
              class="uk-input"
              type="text"
              placeholder="Type adapter name to confirm"
              .value=${this._deleteConfirmName}
              aria-label="Type adapter name to confirm deletion"
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
          Loading adapter…
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
    'or-adapter-detail': OrAdapterDetail;
  }
}
