// Phase 6 Plan 08 Task 2: <or-queue-detail> — Queue entity detail/edit page.
// D6-03: 409 body comes from error.current — no re-GET (Pitfall 9).
// D04_1-02: code field is ALWAYS read-only after create.
// D6-V-40: delete confirm requires typing exact queue.name (case-sensitive).
// channel_types: sl-select multiple with voice/chat/email options; minItems=1 enforced.
// priority and acw_sec: integer fields (type="number" min="0" step="1").
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateQueue from '../../validators/UpdateQueueRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';

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
 * <or-queue-detail> — Queue detail / edit page.
 *
 * Single-column form layout.
 *
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
      padding: 24px;
      max-width: 800px;
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

    .field-helper {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 4px;
    }
    /* Inline confirm panel — D7-04: <sl-dialog> has broken focus-trap inside nested Shadow DOM (shoelace#709, #1382). Embed mounts inside Shadow DOM, so this is replaced with an inline role=dialog + manual focus management. */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .confirm-panel {
      background: var(--or-color-card-bg, #ffffff);
      border: 1px solid var(--or-color-card-border, #d1d5db);
      border-radius: var(--sl-border-radius-medium, 6px);
      padding: 20px;
      max-width: 480px;
      width: 90%;
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
    }
    .confirm-title { margin: 0 0 12px 0; font-size: 18px; font-weight: 600; }
    .confirm-body { margin: 0 0 20px 0; }
    .action-row { display: flex; gap: 8px; justify-content: flex-end; }
    /* Inline confirm panel — D7-04: <sl-dialog> has broken focus-trap inside nested Shadow DOM (shoelace#709, #1382). Embed mounts inside Shadow DOM, so this is replaced with an inline role=dialog + manual focus management. */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .confirm-panel {
      background: var(--or-color-card-bg, #ffffff);
      border: 1px solid var(--or-color-card-border, #d1d5db);
      border-radius: var(--sl-border-radius-medium, 6px);
      padding: 20px;
      max-width: 480px;
      width: 90%;
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
    }
    .confirm-title { margin: 0 0 12px 0; font-size: 18px; font-weight: 600; }
    .confirm-body { margin: 0 0 20px 0; }
    .action-row { display: flex; gap: 8px; justify-content: flex-end; }



    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: var(--or-z-toast, 9000);
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'entity-id' }) accessor entityId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // --- Internal state ---
  @state() private accessor _entity: Queue | null = null;
  @state() private accessor _loading = false;
  @state() private accessor _saving = false;
  @state() private accessor _dirty = false;
  @state() accessor _conflictServer: Record<string, unknown> | null = null;
  @state() accessor _fieldErrors: Record<string, string> = {};
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _showSavedToast = false;
  @state() private accessor _deleteConfirmOpen = false;
  @state() private accessor _deleteConfirmName = '';

  private _savedToastTimeout?: ReturnType<typeof setTimeout>;
  _formData: QueueForm = {
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
  private _onKeydown?: (e: KeyboardEvent) => void;

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._onKeydown) {
      document.removeEventListener('keydown', this._onKeydown);
    }
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('_deleteConfirmOpen') && this._deleteConfirmOpen) {
      this.setAttribute('aria-live', 'polite');
      this.updateComplete.then(() => {
        const input = this.shadowRoot?.querySelector('sl-input[aria-label^="Type"]') as HTMLElement;
        if (input) input.focus();
      });
    } else if (changed.has('_deleteConfirmOpen') && !this._deleteConfirmOpen) {
      this.removeAttribute('aria-live');
    }
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

    // Client-side validation — ajv cast for .errors access
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
        // Storing only _conflictServer left _entity.version stale; re-submit PATCH would fail again.
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Queue }).current;
          this._conflictServer = current as unknown as Record<string, unknown>;
          // Update _entity so next PATCH body.version is the server's current version
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
      // Surface delete error inline (e.g. referential integrity blocking delete)
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

  // --- channel_types change handler ---

  private _handleChannelTypesChange(e: Event): void {
    // sl-select multiple returns string[] when multiple values selected.
    // HTMLSelectElement.value is always string, but sl-select extends with string[].
    const select = e.target as HTMLSelectElement & { value: string | string[] };
    const val = select.value;
    let types: ChannelType[];
    if (Array.isArray(val)) {
      types = val as ChannelType[];
    } else if (typeof val === 'string' && val) {
      types = val.split(' ').filter(Boolean) as ChannelType[];
    } else {
      types = [];
    }
    this._formData = { ...this._formData, channel_types: types };
    this._markDirty();
    // Clear channel_types field error on change
    if (this._fieldErrors['channel_types']) {
      this._fieldErrors = { ...this._fieldErrors, channel_types: '' };
    }
  }

  // --- Render ---

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
    return html`
      <div class="top-bar">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Queues
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

  private _renderDeleteDialog() {
    return html`
      ${when(this._deleteConfirmOpen, () => html`
        <div class="confirm-overlay" role="presentation" @click=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}>
          <div class="confirm-panel"
               role="dialog"
               aria-modal="true"
               aria-labelledby="confirm-title"
               @click=${(e: Event) => e.stopPropagation()}>
            <h3 id="confirm-title" class="confirm-title">Delete queue ${this._entity?.name ?? ''}?</h3>
            <p class="confirm-body">This is permanent and cannot be undone.</p>
            <div style="margin-bottom: 20px;">
              <sl-input
                placeholder="Type queue name to confirm"
                .value=${this._deleteConfirmName}
                @sl-input=${(e: Event) => {
                  this._deleteConfirmName = (e.target as HTMLInputElement).value;
                }}
                aria-label="Type queue name to confirm deletion"
              ></sl-input>
            </div>
            <div class="action-row">
              <sl-button
                variant="default"
                size="small"
                @click=${() => {
                  this._deleteConfirmOpen = false;
                  this._deleteConfirmName = '';
                }}
              >Cancel</sl-button>
              <sl-button
                variant="danger"
                size="small"
                ?disabled=${!this._canDelete}
                @click=${this._handleDelete}
              >Delete</sl-button>
            </div>
          </div>
        </div>
      `)}
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

      <!-- 1. code — read-only (D04_1-02) -->
      <div class="form-group">
        <or-code-input
          .value=${entity.code}
          .readonly=${true}
        ></or-code-input>
      </div>

      <!-- 2. name -->
      <div class="form-group">
        <sl-input
          label="Name"
          value=${this._formData.name}
          required
          ?invalid=${!!this._fieldErrors['name']}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, name: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        ${when(
          this._fieldErrors['name'],
          () => html`<div class="field-error">${this._fieldErrors['name']}</div>`
        )}
      </div>

      <!-- 3. external_id -->
      <div class="form-group">
        <sl-input
          label="External ID"
          value=${this._formData.external_id}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        <div class="field-helper">Optional integration mapping. Pass empty to clear.</div>
      </div>

      <!-- 4. channel_types — sl-select multiple; voice/chat/email -->
      <div class="form-group">
        <label style="font-size:14px;font-weight:500;margin-bottom:4px;display:block">
          Channel Types <span style="color:var(--sl-color-danger-500)">*</span>
        </label>
        <sl-select
          multiple
          placeholder="Select channel types"
          .value=${this._formData.channel_types}
          @sl-change=${this._handleChannelTypesChange}
          aria-label="Channel types"
        >
          <sl-option value="voice">voice</sl-option>
          <sl-option value="chat">chat</sl-option>
          <sl-option value="email">email</sl-option>
        </sl-select>
        ${when(
          this._fieldErrors['channel_types'],
          () => html`<div class="field-error">${this._fieldErrors['channel_types']}</div>`
        )}
      </div>

      <!-- 5. priority -->
      <div class="form-group">
        <sl-input
          label="Priority"
          type="number"
          min="0"
          step="1"
          value=${String(this._formData.priority)}
          required
          ?invalid=${!!this._fieldErrors['priority']}
          @sl-input=${(e: Event) => {
            const v = parseInt((e.target as HTMLInputElement).value, 10);
            this._formData = { ...this._formData, priority: isNaN(v) ? 0 : v };
            this._markDirty();
          }}
        ></sl-input>
        ${when(
          this._fieldErrors['priority'],
          () => html`<div class="field-error">${this._fieldErrors['priority']}</div>`
        )}
      </div>

      <!-- 6. acw_sec -->
      <div class="form-group">
        <sl-input
          label="After-Call Work (seconds)"
          type="number"
          min="0"
          step="1"
          value=${String(this._formData.acw_sec)}
          required
          ?invalid=${!!this._fieldErrors['acw_sec']}
          @sl-input=${(e: Event) => {
            const v = parseInt((e.target as HTMLInputElement).value, 10);
            this._formData = { ...this._formData, acw_sec: isNaN(v) ? 0 : v };
            this._markDirty();
          }}
        ></sl-input>
        <div class="field-helper">After-call work time in seconds.</div>
        ${when(
          this._fieldErrors['acw_sec'],
          () => html`<div class="field-error">${this._fieldErrors['acw_sec']}</div>`
        )}
      </div>

      <!-- 7. enabled -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.enabled}
          @sl-change=${(e: Event) => {
            this._formData = { ...this._formData, enabled: (e.target as HTMLInputElement).checked };
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
              this._navigate(`/orgs/${this.orgId}/queues`);
            } else {
              if (this._entity) {
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
            }
          }}
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${!this._dirty || this._saving}
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
    'or-queue-detail': OrQueueDetail;
  }
}
