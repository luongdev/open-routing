import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateSkill from '../../validators/UpdateSkillRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

import '../primitives/code-input.js';
import '../primitives/conflict-banner.js';

type Skill = components['schemas']['Skill'];

type Form = {
  name: string;
  external_id: string;
  description: string;
  skill_type: string;
  enabled: boolean;
};

/**
 * <or-skill-detail> — Skill detail / edit page (Ember healthcare-dashboard style).
 *
 * Page-header + section cards layout. Simple entity per D6-17 (no sub-table).
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the skill UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-skill-detail')
export class OrSkillDetail extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 24px;
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
      font-size: 24px;
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
      gap: 12px;
      margin-bottom: 20px;
    }

    @media (max-width: 640px) {
      .stats-row { grid-template-columns: 1fr 1fr; }
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px 16px;
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
      font-size: 20px;
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
      padding: 20px 24px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 16px;
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 16px;
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
      gap: 12px 24px;
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

    textarea.uk-input {
      min-height: 72px;
      resize: vertical;
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

    /* ── Alerts ──────────────────────────────────────────────────── */
    .alert {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px 14px;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 16px;
    }

    .alert--warning {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 15%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.65 0.18 80) 40%, transparent);
      color: oklch(0.45 0.18 75);
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
      margin-top: 16px;
      padding-top: 12px;
      border-top: 1px solid var(--border);
    }

    .bottom-bar {
      display: flex;
      gap: 8px;
      margin-top: 20px;
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

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;

  @state() private _entity: Skill | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() private _dirty = false;
  @state() private _conflictServer: Record<string, unknown> | null = null;
  @state() private _fieldErrors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';
  @state() private _discardDialogOpen = false;

  private _form: Form = { name: '', external_id: '', description: '', skill_type: '', enabled: true };
  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  private _onKeydown?: (e: KeyboardEvent) => void;

  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadEntity();
    this._onKeydown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (this._deleteConfirmOpen) {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }
        if (this._discardDialogOpen) {
          this._discardDialogOpen = false;
        }
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
      this.setAttribute('aria-live', 'polite');
      void this.updateComplete.then(() => {
        const input = this.shadowRoot?.querySelector('.confirm-panel input') as HTMLElement | null;
        if (input) input.focus();
      });
    } else if (changed.has('_deleteConfirmOpen') && !this._deleteConfirmOpen) {
      this.removeAttribute('aria-live');
    }
    if (changed.has('_discardDialogOpen') && this._discardDialogOpen) {
      this.setAttribute('aria-live', 'polite');
      void this.updateComplete.then(() => {
        const btn = this.shadowRoot?.querySelector('.discard-dialog-btn') as HTMLElement | null;
        if (btn) btn.focus();
      });
    } else if (changed.has('_discardDialogOpen') && !this._discardDialogOpen) {
      this.removeAttribute('aria-live');
    }
  }

  private async _loadEntity(): Promise<void> {
    if (!this.orgId || !this.entityId) return;
    this._loading = true;
    try {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/skills/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load skill';
        return;
      }
      const skill = data as Skill;
      this._entity = skill;
      this._form = {
        name: skill.name ?? '',
        external_id: skill.external_id ?? '',
        description: (skill as { description?: string | null }).description ?? '',
        skill_type: (skill as { skill_type?: string }).skill_type ?? '',
        enabled: skill.enabled ?? true,
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

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    const body = {
      name: this._form.name,
      // Per D04_1-07: pass "" to clear binding; null === omission
      external_id: this._form.external_id,
      description: this._form.description,
      skill_type: this._form.skill_type,
      enabled: this._form.enabled,
      version: this._entity.version,
    };

    const validateFn = validateUpdateSkill as unknown as {
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
      const result = await this.client.PATCH('/v1/orgs/{org_id}/skills/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: Skill | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (D6-03, Pitfall 9: never call response.json())
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Record<string, unknown> }).current;
          this._conflictServer = current;
          // Cross-AI fix: also update _entity so re-submit uses server-current version
          this._entity = current as unknown as Skill;
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
          external_id: data.external_id ?? '',
          description: (data as { description?: string | null }).description ?? '',
          skill_type: (data as { skill_type?: string }).skill_type ?? '',
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

  async _handleEnable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: true, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/skills/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Skill | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: false, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/skills/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Skill | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: false };
    }
  }

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/skills/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/skills`);
    }
  }

  _handleCancel(): void {
    if (!this._dirty) {
      this._navigate(`/orgs/${this.orgId}/skills`);
    } else {
      this._discardDialogOpen = true;
    }
  }

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard' && this._conflictServer) {
      const serverData = this._conflictServer;
      this._entity = serverData as unknown as Skill;
      this._form = {
        name: (serverData['name'] as string) ?? '',
        external_id: (serverData['external_id'] as string) ?? '',
        description: (serverData['description'] as string) ?? '',
        skill_type: (serverData['skill_type'] as string) ?? '',
        enabled: (serverData['enabled'] as boolean) ?? true,
      };
      this._dirty = false;
    } else if (e.detail?.action !== 'discard' && this._conflictServer) {
      if (this._entity && 'version' in this._conflictServer) {
        this._entity = { ...this._entity, version: this._conflictServer['version'] as number };
      }
    }
    this._conflictServer = null;
  }

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
            title="Back to Skills"
            @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
          >
            <uk-icon icon="arrow-left" width="18" height="18"></uk-icon>
          </button>
          <div>
            <h1 class="page-title">${entity?.name ?? 'Skill Detail'}</h1>
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
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._form as Record<string, unknown>}
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
          <uk-icon icon="tag" width="15" height="15"></uk-icon>
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
            <label class="field-label" for="skill-name">Name</label>
            <input
              id="skill-name"
              class="uk-input"
              type="text"
              required
              .value=${this._form.name}
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
            <label class="field-label" for="skill-type">Skill Type</label>
            <input
              id="skill-type"
              class="uk-input"
              type="text"
              required
              .value=${this._form.skill_type}
              @input=${(e: Event) => {
                this._form = { ...this._form, skill_type: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['skill_type']
              ? html`<div class="field-error">${this._fieldErrors['skill_type']}</div>`
              : nothing}
          </div>

          <div class="form-group" style="grid-column: 1 / -1">
            <label class="field-label" for="skill-description">Description</label>
            <textarea
              id="skill-description"
              class="uk-input"
              .value=${this._form.description}
              @input=${(e: Event) => {
                this._form = { ...this._form, description: (e.target as HTMLTextAreaElement).value };
                this._markDirty();
              }}
            ></textarea>
          </div>

          <div class="form-group">
            <label class="field-label" for="skill-ext-id">External ID</label>
            <input
              id="skill-ext-id"
              class="uk-input"
              type="text"
              .value=${this._form.external_id}
              @input=${(e: Event) => {
                this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
          </div>
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
            <div class="toggle-sub">Disabled skills cannot be assigned to agents</div>
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
              this._navigate(`/orgs/${this.orgId}/skills`);
            } else {
              this._form = {
                name: entity.name ?? '',
                external_id: entity.external_id ?? '',
                description: (entity as { description?: string | null }).description ?? '',
                skill_type: (entity as { skill_type?: string }).skill_type ?? '',
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
          <h3 id="confirm-title" class="confirm-title">Delete skill "${this._entity?.name ?? ''}"?</h3>
          <p class="confirm-body">This is permanent and cannot be undone. Type the skill name to confirm.</p>
          <div style="margin-bottom: 20px;">
            <label class="field-label" for="delete-confirm-input">Skill name</label>
            <input
              id="delete-confirm-input"
              class="uk-input"
              type="text"
              placeholder="Type skill name to confirm"
              .value=${this._deleteConfirmName}
              aria-label="Type skill name to confirm deletion"
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

  private _renderDiscardDialog() {
    return when(this._discardDialogOpen, () => html`
      <div
        class="confirm-overlay"
        role="presentation"
        @click=${() => { this._discardDialogOpen = false; }}
      >
        <div
          class="confirm-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="discard-title"
          @click=${(e: Event) => e.stopPropagation()}
        >
          <h3 id="discard-title" class="confirm-title">Discard your changes?</h3>
          <p class="confirm-body">Unsaved edits will be lost.</p>
          <div class="action-row">
            <button
              class="uk-button uk-button-default uk-button-small"
              @click=${() => { this._discardDialogOpen = false; }}
            >Keep editing</button>
            <button
              class="uk-button uk-button-danger uk-button-small discard-dialog-btn"
              @click=${() => {
                this._discardDialogOpen = false;
                this._navigate(`/orgs/${this.orgId}/skills`);
              }}
            >Discard</button>
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
          Loading skill…
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
      ${this._renderDiscardDialog()}

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
    'or-skill-detail': OrSkillDetail;
  }
}
