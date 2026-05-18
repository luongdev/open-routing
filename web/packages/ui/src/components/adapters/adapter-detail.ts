// Phase 6 Plan 11 Task 2: <or-adapter-detail> — Adapter entity detail/edit page.
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// Pitfall 9: never call response.json() — openapi-fetch parses error body for us.
// T-06-11-01: config display uses Lit html template literals (auto-escape); no innerHTML with config values.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateAdapter from '../../validators/UpdateAdapterRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/textarea/textarea.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/dialog/dialog.js';

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

/**
 * <or-adapter-detail> — Adapter detail / edit page.
 *
 * Simpler than agent-detail (no skills sub-panel, no wrapup countdown).
 * Key feature: JSONB config field displayed as monospace sl-textarea with JSON validation.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the adapter UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-adapter-detail')
export class OrAdapterDetail extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      max-width: 720px;
    }

    .top-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .top-bar-spacer { flex: 1; }

    .page-title {
      font-size: var(--or-text-display, 24px);
      font-weight: 700;
      color: var(--or-color-text-strong, #171717);
      margin: 0 0 4px;
    }

    .form-group {
      margin-bottom: 16px;
    }

    .footer-meta {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 16px;
      padding-top: 12px;
      border-top: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .bottom-bar {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      justify-content: flex-end;
    }

    .field-error {
      font-size: 12px;
      color: var(--sl-color-danger-500, #d92d20);
      margin-top: 4px;
    }

    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: var(--or-z-toast, 9000);
    }

    .delete-btn-danger {
      color: var(--sl-color-danger-500, #d92d20);
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'entity-id' }) accessor entityId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // --- Internal state ---
  @state() private accessor _entity: Adapter | null = null;
  @state() private accessor _loading = false;
  @state() private accessor _saving = false;
  @state() private accessor _dirty = false;
  @state() private accessor _conflictServer: Record<string, unknown> | null = null;
  @state() private accessor _fieldErrors: Record<string, string> = {};
  @state() private accessor _configError = '';
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _showSavedToast = false;
  @state() private accessor _deleteConfirmOpen = false;
  @state() private accessor _deleteConfirmName = '';

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
  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadEntity();
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
        // JSONB config: display as pretty-printed JSON; null/undefined → empty textarea
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

    // Block save if config JSON is invalid
    if (this._configError) return;

    // Re-validate config on save
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

    // Client-side ajv validation
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
        // 409 version_conflict — consume from error.current (D6-03: Pitfall 9)
        // Cross-AI fix: also update _entity.version so a re-submit uses the server-current
        // version instead of looping into another 409 (Codex review HIGH).
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Adapter }).current;
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

  private _renderConflictBanner() {
    if (!this._conflictServer) return null;

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

  private _renderDeleteDialog() {
    return html`
      <sl-dialog
        label="Delete adapter ${this._entity?.name ?? ''}?"
        ?open=${this._deleteConfirmOpen}
        @sl-request-close=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}
      >
        <p>This is permanent and cannot be undone.</p>
        <sl-input
          placeholder="Type adapter name to confirm"
          value=${this._deleteConfirmName}
          @sl-input=${(e: Event) => {
            this._deleteConfirmName = (e.target as HTMLInputElement).value;
          }}
          aria-label="Type adapter name to confirm deletion"
        ></sl-input>
        <div slot="footer" style="display:flex;gap:8px;justify-content:flex-end">
          <sl-button
            variant="default"
            @click=${() => {
              this._deleteConfirmOpen = false;
              this._deleteConfirmName = '';
            }}
          >Cancel</sl-button>
          <sl-button
            variant="danger"
            ?disabled=${!this._canDelete}
            @click=${this._handleDelete}
          >Delete</sl-button>
        </div>
      </sl-dialog>
    `;
  }

  private _renderTopBar() {
    return html`
      <div class="top-bar">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Adapters
        </sl-button>
        <div class="top-bar-spacer"></div>

        ${when(
          this._entity,
          () => html`
            ${when(
              this._entity!.enabled,
              () => html`
                <sl-button
                  variant="default"
                  size="small"
                  @click=${this._handleDisable}
                >Disable</sl-button>
              `,
              () => html`
                <sl-button
                  variant="default"
                  size="small"
                  @click=${this._handleEnable}
                >Enable</sl-button>
              `
            )}
            <sl-button
              variant="default"
              size="small"
              class="delete-btn-danger"
              style="color:var(--sl-color-danger-500)"
              @click=${() => {
                this._deleteConfirmOpen = true;
                this._deleteConfirmName = '';
              }}
            >Delete</sl-button>
          `
        )}
      </div>
    `;
  }

  private _renderForm() {
    if (!this._entity) return null;
    const entity = this._entity;
    const relativeTime = (iso: string) => {
      try {
        const ms = Date.now() - new Date(iso).getTime();
        const min = Math.floor(ms / 60000);
        if (min < 60) return `${min}m ago`;
        const h = Math.floor(min / 60);
        if (h < 24) return `${h}h ago`;
        return `${Math.floor(h / 24)}d ago`;
      } catch { return iso; }
    };

    return html`
      <h1 class="page-title">${entity.name}</h1>

      ${this._renderConflictBanner()}

      ${when(
        this._apiError,
        () => html`
          <sl-alert variant="danger" open style="margin-bottom:16px">
            <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
            ${this._apiError}
          </sl-alert>
        `
      )}

      <div class="form-group">
        <or-code-input
          .value=${entity.code}
          .readonly=${true}
        ></or-code-input>
      </div>

      <div class="form-group">
        <sl-input
          label="Name"
          value=${this._form.name}
          required
          ?invalid=${!!this._fieldErrors['name']}
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, name: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        ${when(
          this._fieldErrors['name'],
          () => html`<div class="field-error">${this._fieldErrors['name']}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-input
          label="External ID"
          value=${this._form.external_id}
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
      </div>

      <div class="form-group">
        <sl-input
          label="Adapter Type"
          value=${this._form.adapter_type}
          required
          ?invalid=${!!this._fieldErrors['adapter_type']}
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, adapter_type: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        ${when(
          this._fieldErrors['adapter_type'],
          () => html`<div class="field-error">${this._fieldErrors['adapter_type']}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-textarea
          label="Config (JSON)"
          rows="8"
          style="font-family: monospace; white-space: pre;"
          value=${this._form.configText}
          placeholder='{"key": "value"}'
          ?invalid=${!!this._configError}
          @sl-input=${this._handleConfigInput}
        ></sl-textarea>
        ${when(
          this._configError,
          () => html`<div class="field-error">${this._configError}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-switch
          ?checked=${this._form.enabled}
          @sl-change=${(e: Event) => {
            this._form = { ...this._form, enabled: (e.target as HTMLInputElement).checked };
            this._markDirty();
          }}
        >Enabled</sl-switch>
      </div>

      <div class="footer-meta">
        version ${entity.version}
        · updated ${relativeTime(entity.updated_at ?? '')}
        · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
      </div>

      <div class="bottom-bar">
        <sl-button
          variant="default"
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
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${!this._dirty || this._saving || !!this._configError}
          @click=${this._handleSave}
        >
          ${this._saving ? html`<sl-spinner></sl-spinner> Saving…` : 'Save changes'}
        </sl-button>
      </div>
    `;
  }

  override render() {
    if (this._loading) {
      return html`<sl-spinner></sl-spinner>`;
    }

    if (!this._entity && this._apiError) {
      return html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          ${this._apiError}
        </sl-alert>
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
            <sl-alert variant="success" open>
              <sl-icon slot="icon" name="check-circle"></sl-icon>
              Saved
            </sl-alert>
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
