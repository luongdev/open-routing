// Phase 6 Plan 10 Task 2: <or-channel-detail> — Channel entity detail/edit page.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET (Pitfall 9 avoided).
// T-06-10-01: ajv validateUpdateChannel enforces channel_type enum.
// default_queue_id: or-queue-picker with null clear support (nullable field).
// UI-SPEC §5.4: fields: code(RO), name, external_id, channel_type(select), default_queue_id(or-queue-picker), enabled.
// W0.1-15: Ember dashboard style — adoptShadowSheets, uk-button/uk-input, uk-icon, zero sl-*.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateChannel from '../../validators/UpdateChannelRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/conflict-banner.js';
import '../primitives/queue-picker.js';

type Channel = components['schemas']['Channel'];
type ChannelType = 'voice' | 'chat' | 'email' | 'sms' | 'social';

const CHANNEL_TYPE_OPTIONS: Array<{ value: ChannelType; label: string }> = [
  { value: 'voice', label: 'Voice' },
  { value: 'chat', label: 'Chat' },
  { value: 'email', label: 'Email' },
  { value: 'sms', label: 'SMS' },
  { value: 'social', label: 'Social' },
];

type ChannelForm = {
  name: string;
  external_id: string;
  channel_type: ChannelType | '';
  default_queue_id: string | null;
  enabled: boolean;
};

/**
 * <or-channel-detail> — Channel detail / edit page (Ember dashboard style).
 *
 * Page-header + identity card layout.
 * Fields: code (readonly), name, external_id, channel_type (select), default_queue_id (or-queue-picker, nullable), enabled.
 * 409 conflict: consumed from error.current (D6-03) — no second GET.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the channel UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-channel-detail')
export class OrChannelDetail extends LitElement {
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

    /* ── Two-column grid for identity fields ─────────────────────── */
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

    .field-label-required::after {
      content: ' *';
      color: var(--destructive);
    }

    .field-error {
      font-size: 12px;
      color: var(--destructive);
      margin-top: 4px;
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

    /* Frankenstyle toggle switch */
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

    /* ── Alert / error banner ────────────────────────────────────── */
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

    /* ── Delete confirm inline panel (D7-04 pattern) ─────────────── */
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

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _entity: Channel | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() private _dirty = false;
  @state() _conflictServer: Record<string, unknown> | null = null;
  @state() _fieldErrors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';

  private _savedToastTimeout?: ReturnType<typeof setTimeout>;
  private _onKeydown?: (e: KeyboardEvent) => void;

  _formData: ChannelForm = {
    name: '',
    external_id: '',
    channel_type: '',
    default_queue_id: null,
    enabled: true,
  };

  // --- Computed ---
  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  // --- Lifecycle ---
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
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/channels/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load channel';
        return;
      }
      const channel = data as Channel;
      this._entity = channel;
      this._formData = {
        name: channel.name ?? '',
        external_id: channel.external_id ?? '',
        channel_type: (channel.channel_type ?? '') as ChannelType | '',
        default_queue_id: channel.default_queue_id ?? null,
        enabled: channel.enabled ?? true,
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

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    // Build validation body — omit default_queue_id if null to work around
    // generated ajv validator's allOf+nullable UUID handling (erroneously fails null).
    const validationBody: Record<string, unknown> = {
      name: this._formData.name,
      external_id: this._formData.external_id,
      channel_type: this._formData.channel_type as ChannelType,
      enabled: this._formData.enabled,
      version: this._entity.version,
    };
    if (this._formData.default_queue_id !== null) {
      validationBody['default_queue_id'] = this._formData.default_queue_id;
    }

    // Client-side validation — ajv cast for .errors access (T-06-10-01)
    const validateFn = validateUpdateChannel as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(validationBody)) {
      const errors: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errors[field] = err.message ?? 'Invalid value';
      }
      this._fieldErrors = errors;
      return;
    }
    this._fieldErrors = {};

    // Full PATCH body includes explicit null for default_queue_id (nullable field)
    // Normalize external_id — pass undefined when empty to avoid 422 minLength
    const body = {
      name: this._formData.name,
      external_id: this._formData.external_id || undefined,
      channel_type: this._formData.channel_type as ChannelType,
      default_queue_id: this._formData.default_queue_id,
      enabled: this._formData.enabled,
      version: this._entity.version,
    };

    this._saving = true;
    try {
      const result = await this.client.PATCH('/v1/orgs/{org_id}/channels/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: Channel | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (D6-03: no re-GET)
        // Also update _entity so re-submit uses the correct version.
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Channel }).current;
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
        this._apiError = (error as { reason?: string })?.reason ?? 'Save failed';
        return;
      }

      if (data) {
        this._entity = data;
        this._formData = {
          name: data.name ?? '',
          external_id: data.external_id ?? '',
          channel_type: (data.channel_type ?? '') as ChannelType | '',
          default_queue_id: data.default_queue_id ?? null,
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
    const result = await this.client.PATCH('/v1/orgs/{org_id}/channels/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body: { enabled: true, version: this._entity.version },
    } as never);
    const { data, error } = result as { data: Channel | null; error: unknown };
    if (error) {
      this._apiError = (error as { reason?: string })?.reason ?? 'Failed to enable channel';
      return;
    }
    if (data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const result = await this.client.PATCH('/v1/orgs/{org_id}/channels/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body: { enabled: false, version: this._entity.version },
    } as never);
    const { data, error } = result as { data: Channel | null; error: unknown };
    if (error) {
      this._apiError = (error as { reason?: string })?.reason ?? 'Failed to disable channel';
      return;
    }
    if (data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: false };
    }
  }

  // --- Delete ---

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/channels/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/channels`);
    } else {
      this._deleteConfirmOpen = false;
      this._deleteConfirmName = '';
      this._apiError = (error as { reason?: string })?.reason ?? 'Delete failed. The channel may be in use.';
    }
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard' && this._entity) {
      this._formData = {
        name: this._entity.name ?? '',
        external_id: this._entity.external_id ?? '',
        channel_type: (this._entity.channel_type ?? '') as ChannelType | '',
        default_queue_id: this._entity.default_queue_id ?? null,
        enabled: this._entity.enabled ?? true,
      };
      this._dirty = false;
    }
    this._conflictServer = null;
  }

  // --- Render helpers ---

  private _renderConflictBanner() {
    if (!this._conflictServer) return null;
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._formData as unknown as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
    `;
  }

  private _renderTopBar() {
    const entity = this._entity;
    return html`
      <div class="page-header">
        <div class="page-header-left">
          <button
            class="back-btn"
            type="button"
            @click=${() => this._navigate(`/orgs/${this.orgId}/channels`)}
            aria-label="Back to Channels"
          >
            <uk-icon icon="chevron-left" height="18" width="18"></uk-icon>
          </button>
          <div>
            <h1 class="page-title">${entity?.name ?? 'Channel'}</h1>
            ${entity ? html`<p class="page-subtitle">${entity.code}</p>` : nothing}
          </div>
        </div>

        <div class="page-header-right">
          ${entity ? html`
            <span class="status-badge ${entity.enabled ? 'status-badge--active' : 'status-badge--disabled'}">
              <uk-icon icon=${entity.enabled ? 'circle-check' : 'circle-x'} height="12" width="12"></uk-icon>
              ${entity.enabled ? 'Active' : 'Disabled'}
            </span>
            ${entity.enabled
              ? html`<button class="uk-button uk-button-default" type="button" style="font-size:13px" @click=${this._handleDisable}>Disable</button>`
              : html`<button class="uk-button uk-button-default" type="button" style="font-size:13px" @click=${this._handleEnable}>Enable</button>`
            }
            <button
              class="uk-button uk-button-default"
              type="button"
              style="font-size:13px;color:var(--destructive)"
              @click=${() => {
                this._deleteConfirmOpen = true;
                this._deleteConfirmName = '';
              }}
            >Delete</button>
          ` : nothing}
        </div>
      </div>
    `;
  }

  private _renderForm() {
    if (!this._entity) return nothing;
    const entity = this._entity;

    return html`
      ${this._renderConflictBanner()}

      ${when(
        this._apiError,
        () => html`
          <div class="alert alert--danger">
            <uk-icon icon="triangle-alert" height="16" width="16"></uk-icon>
            ${this._apiError}
          </div>
        `
      )}

      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="radio-tower" height="16" width="16"></uk-icon>
          Identity
        </h2>

        <div class="two-col-grid">
          <div class="form-group">
            <or-code-input
              .value=${entity.code}
              .readonly=${true}
            ></or-code-input>
          </div>

          <div class="form-group">
            <label class="field-label field-label-required" for="ch-name">Name</label>
            <input
              id="ch-name"
              class="uk-input"
              type="text"
              required
              .value=${this._formData.name}
              @input=${(e: Event) => {
                this._formData = { ...this._formData, name: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${when(
              this._fieldErrors['name'],
              () => html`<div class="field-error">${this._fieldErrors['name']}</div>`
            )}
          </div>

          <div class="form-group">
            <label class="field-label field-label-required" for="ch-type">Channel Type</label>
            <select
              id="ch-type"
              class="uk-select"
              @change=${(e: Event) => {
                const val = (e.target as HTMLSelectElement).value as ChannelType;
                this._formData = { ...this._formData, channel_type: val };
                this._markDirty();
              }}
              aria-label="Channel type"
            >
              ${CHANNEL_TYPE_OPTIONS.map(
                (opt) => html`<option value=${opt.value} ?selected=${this._formData.channel_type === opt.value}>${opt.label}</option>`
              )}
            </select>
            ${when(
              this._fieldErrors['channel_type'],
              () => html`<div class="field-error">${this._fieldErrors['channel_type']}</div>`
            )}
          </div>

          <div class="form-group">
            <label class="field-label" for="ch-ext-id">External ID</label>
            <input
              id="ch-ext-id"
              class="uk-input"
              type="text"
              .value=${this._formData.external_id}
              @input=${(e: Event) => {
                this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
          </div>

          <div class="form-group" style="grid-column: 1 / -1">
            <label class="field-label">Default Queue <span style="font-weight:400">(optional)</span></label>
            <or-queue-picker
              .orgId=${this.orgId}
              .client=${this.client}
              .value=${this._formData.default_queue_id}
              @or-queue-picker-change=${(e: CustomEvent) => {
                this._formData = { ...this._formData, default_queue_id: e.detail.queueId };
                this._markDirty();
              }}
            ></or-queue-picker>
          </div>
        </div>

        <div class="footer-meta">
          version ${entity.version}
          · updated ${this._relativeTime(entity.updated_at ?? '')}
          · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
        </div>

        <div class="bottom-bar">
          <button
            class="uk-button uk-button-default"
            type="button"
            @click=${() => {
              if (!this._dirty) {
                this._navigate(`/orgs/${this.orgId}/channels`);
              } else {
                this._formData = {
                  name: entity.name ?? '',
                  external_id: entity.external_id ?? '',
                  channel_type: (entity.channel_type ?? '') as ChannelType | '',
                  default_queue_id: entity.default_queue_id ?? null,
                  enabled: entity.enabled ?? true,
                };
                this._dirty = false;
              }
            }}
          >Cancel</button>
          <button
            class="uk-button uk-button-primary"
            type="button"
            ?disabled=${!this._dirty || this._saving}
            @click=${this._handleSave}
          >
            ${this._saving
              ? html`<span class="spinner" style="width:14px;height:14px;margin-right:6px;border-width:2px"></span> Saving…`
              : 'Save changes'}
          </button>
        </div>
      </div>

      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="settings" height="16" width="16"></uk-icon>
          Status &amp; Routing
        </h2>

        <div class="toggle-row">
          <div>
            <div class="toggle-label">Enabled</div>
            <div class="toggle-sub">Allow incoming interactions on this channel</div>
          </div>
          <label class="uk-toggle">
            <input
              type="checkbox"
              .checked=${this._formData.enabled}
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

  private _renderDeleteDialog() {
    if (!this._deleteConfirmOpen) return nothing;
    return html`
      <div class="confirm-overlay" role="dialog" aria-modal="true" aria-labelledby="del-title">
        <div class="confirm-panel">
          <h2 class="confirm-title" id="del-title">Delete channel ${this._entity?.name ?? ''}?</h2>
          <p class="confirm-body">This is permanent and cannot be undone. Type the channel name to confirm.</p>
          <input
            class="uk-input"
            style="margin-bottom:16px"
            placeholder="Type channel name to confirm"
            .value=${this._deleteConfirmName}
            @input=${(e: Event) => {
              this._deleteConfirmName = (e.target as HTMLInputElement).value;
            }}
            aria-label="Type channel name to confirm deletion"
          />
          <div class="action-row">
            <button
              class="uk-button uk-button-default"
              type="button"
              @click=${() => {
                this._deleteConfirmOpen = false;
                this._deleteConfirmName = '';
              }}
            >Cancel</button>
            <button
              class="uk-button uk-button-danger"
              type="button"
              ?disabled=${!this._canDelete}
              @click=${this._handleDelete}
            >Delete</button>
          </div>
        </div>
      </div>
    `;
  }

  override render() {
    if (this._loading) {
      return html`
        <div class="loading-wrap">
          <span class="spinner"></span>
          Loading channel…
        </div>
      `;
    }

    if (!this._entity && this._apiError) {
      return html`
        <div class="alert alert--danger">
          <uk-icon icon="triangle-alert" height="16" width="16"></uk-icon>
          ${this._apiError}
        </div>
      `;
    }

    return html`
      ${this._renderTopBar()}
      ${this._renderForm()}
      ${this._renderDeleteDialog()}

      ${when(
        this._showSavedToast,
        () => html`
          <div class="toast-container">
            <div class="toast">
              <uk-icon icon="circle-check" height="16" width="16"></uk-icon>
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
    'or-channel-detail': OrChannelDetail;
  }
}
