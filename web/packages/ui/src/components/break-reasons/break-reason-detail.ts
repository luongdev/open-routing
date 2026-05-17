// Phase 6 Plan 09 Task 2: <or-break-reason-detail> — BreakReason entity detail/edit page.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET (Pitfall 9 avoided).
// UI-SPEC §5.4: routable → sl-switch, display_order → sl-input type="number" min="0" step="1"
// routable helper: "When on, agents on this break can still receive routed interactions."
// display_order helper: "Lower values appear first in the break picker."

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateBreakReason from '../../validators/UpdateBreakReasonRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
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

type BreakReason = components['schemas']['BreakReason'];

type FormData = {
  name: string;
  external_id: string;
  routable: boolean;
  display_order: number;
  enabled: boolean;
};

/**
 * <or-break-reason-detail> — BreakReason detail / edit page.
 *
 * Single-column form: code (readonly), name, external_id, routable (switch), display_order (number), enabled.
 * 409 conflict: consumed from error.current (D6-03) — no second GET.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the break reason UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-break-reason-detail')
export class OrBreakReasonDetail extends LitElement {
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

    .helper-text {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 4px;
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _entity: BreakReason | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() _dirty = false;
  @state() _conflictServer: Record<string, unknown> | null = null;
  @state() private _fieldErrors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';

  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

  _formData: FormData = {
    name: '',
    external_id: '',
    routable: true,
    display_order: 0,
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
  }

  // --- Private methods ---

  private async _loadEntity(): Promise<void> {
    if (!this.orgId || !this.entityId) return;
    this._loading = true;
    try {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/break-reasons/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load break reason';
        return;
      }
      const br = data as BreakReason;
      this._entity = br;
      this._formData = {
        name: br.name ?? '',
        external_id: br.external_id ?? '',
        routable: br.routable ?? true,
        display_order: br.display_order ?? 0,
        enabled: br.enabled ?? true,
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
      // Per UpdateBreakReasonRequest contract: pass "" to clear binding
      external_id: this._formData.external_id,
      routable: this._formData.routable,
      display_order: this._formData.display_order,
      enabled: this._formData.enabled,
      version: this._entity.version,
    };

    // Client-side validation
    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateUpdateBreakReason as unknown as {
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
      const result = await this.client.PATCH('/v1/orgs/{org_id}/break-reasons/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: BreakReason | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (D6-03, Pitfall 9: never call response.json())
        if (error && typeof error === 'object' && 'current' in error) {
          this._conflictServer = (error as { current: Record<string, unknown> }).current;
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
            this._apiError = (errObj.reason ?? 'Invalid value');
          }
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Save failed';
        return;
      }

      if (data) {
        const br = data as BreakReason;
        this._entity = br;
        this._formData = {
          name: br.name ?? '',
          external_id: br.external_id ?? '',
          routable: br.routable ?? true,
          display_order: br.display_order ?? 0,
          enabled: br.enabled ?? true,
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
    const body = { enabled: true, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/break-reasons/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: BreakReason | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: false, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/break-reasons/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: BreakReason | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._formData = { ...this._formData, enabled: false };
    }
  }

  // --- Delete ---

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/break-reasons/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/break-reasons`);
    }
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard') {
      if (this._entity) {
        this._formData = {
          name: this._entity.name ?? '',
          external_id: this._entity.external_id ?? '',
          routable: this._entity.routable ?? true,
          display_order: this._entity.display_order ?? 0,
          enabled: this._entity.enabled ?? true,
        };
        this._dirty = false;
      }
    }
    this._conflictServer = null;
  }

  // --- Render helpers ---

  private _relativeTime(iso: string): string {
    try {
      const ms = Date.now() - new Date(iso).getTime();
      const min = Math.floor(ms / 60000);
      if (min < 60) return `${min}m ago`;
      const h = Math.floor(min / 60);
      if (h < 24) return `${h}h ago`;
      return `${Math.floor(h / 24)}d ago`;
    } catch { return iso; }
  }

  private _renderTopBar() {
    return html`
      <div class="top-bar">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Break reasons
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

  private _renderConflictBanner() {
    if (!this._conflictServer) return null;
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._formData as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
    `;
  }

  private _renderDeleteDialog() {
    return html`
      <sl-dialog
        label="Delete break reason ${this._entity?.name ?? ''}?"
        ?open=${this._deleteConfirmOpen}
        @sl-request-close=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}
      >
        <p>This is permanent and cannot be undone.</p>
        <sl-input
          placeholder="Type break reason name to confirm"
          value=${this._deleteConfirmName}
          @sl-input=${(e: Event) => {
            this._deleteConfirmName = (e.target as HTMLInputElement).value;
          }}
          aria-label="Type break reason name to confirm deletion"
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

  private _renderForm() {
    if (!this._entity) return null;
    const entity = this._entity;

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

      <!-- 1. code → or-code-input readonly (D04_1-02) -->
      <div class="form-group">
        <or-code-input
          .value=${entity.code}
          .readonly=${true}
        ></or-code-input>
      </div>

      <!-- 2. name → sl-input required -->
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

      <!-- 3. external_id → sl-input optional -->
      <div class="form-group">
        <sl-input
          label="External ID"
          value=${this._formData.external_id}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        <div class="helper-text">Optional integration mapping. Pass empty to clear.</div>
      </div>

      <!-- 4. routable → sl-switch per UI-SPEC §5.4 -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.routable}
          @sl-change=${(e: Event) => {
            this._formData = { ...this._formData, routable: (e.target as HTMLInputElement).checked };
            this._markDirty();
          }}
        >Routable</sl-switch>
        <div class="helper-text">When on, agents on this break can still receive routed interactions.</div>
      </div>

      <!-- 5. display_order → sl-input type="number" min="0" step="1" per UI-SPEC §5.4 -->
      <div class="form-group">
        <sl-input
          label="Display Order"
          type="number"
          min="0"
          step="1"
          value=${String(this._formData.display_order)}
          required
          ?invalid=${!!this._fieldErrors['display_order']}
          @sl-input=${(e: Event) => {
            const val = parseInt((e.target as HTMLInputElement).value, 10);
            this._formData = { ...this._formData, display_order: isNaN(val) ? 0 : val };
            this._markDirty();
          }}
        ></sl-input>
        <div class="helper-text">Lower values appear first in the break picker.</div>
        ${when(
          this._fieldErrors['display_order'],
          () => html`<div class="field-error">${this._fieldErrors['display_order']}</div>`
        )}
      </div>

      <!-- 6. enabled → sl-switch -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.enabled}
          @sl-change=${(e: Event) => {
            this._formData = { ...this._formData, enabled: (e.target as HTMLInputElement).checked };
            this._markDirty();
          }}
        >Enabled</sl-switch>
      </div>

      <!-- Footer metadata bar (D6-V-14) -->
      <div class="footer-meta">
        version ${entity.version}
        · updated ${this._relativeTime(entity.updated_at ?? '')}
        · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
      </div>

      <div class="bottom-bar">
        <sl-button
          variant="default"
          @click=${() => {
            if (!this._dirty) {
              this._navigate(`/orgs/${this.orgId}/break-reasons`);
            } else {
              this._formData = {
                name: entity.name ?? '',
                external_id: entity.external_id ?? '',
                routable: entity.routable ?? true,
                display_order: entity.display_order ?? 0,
                enabled: entity.enabled ?? true,
              };
              this._dirty = false;
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
    'or-break-reason-detail': OrBreakReasonDetail;
  }
}
