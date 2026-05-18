// Phase 6 Plan 07 Task 2: <or-skill-detail> — Skill entity detail/edit page.
// Adapted from or-agent-detail pattern (Plan 06-05) — simpler: no sub-table, no wrapup.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET (Pitfall 9).
// D6-V-40: delete confirm requires typing exact skill.name (case-sensitive).
// Skills is the simple entity per D6-17: single-column layout.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateSkill from '../../validators/UpdateSkillRequest.js';

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

// Primitives
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
 * <or-skill-detail> — Skill detail / edit page.
 *
 * Single-column layout (skills is a simple entity — no sub-table per D6-17).
 *
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

    .delete-btn-danger {
      color: var(--sl-color-danger-500, #d92d20);
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'entity-id' }) accessor entityId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // --- Internal state ---
  @state() private accessor _entity: Skill | null = null;
  @state() private accessor _loading = false;
  @state() private accessor _saving = false;
  @state() private accessor _dirty = false;
  @state() private accessor _conflictServer: Record<string, unknown> | null = null;
  @state() private accessor _fieldErrors: Record<string, string> = {};
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _showSavedToast = false;
  @state() private accessor _deleteConfirmOpen = false;
  @state() private accessor _deleteConfirmName = '';
  @state() private accessor _discardDialogOpen = false;

  private _form: Form = { name: '', external_id: '', description: '', skill_type: '', enabled: true };
  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

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
    if (changed.has('_discardDialogOpen') && this._discardDialogOpen) {
      this.setAttribute('aria-live', 'polite');
      this.updateComplete.then(() => {
        const btn = this.shadowRoot?.querySelector('.discard-dialog-btn') as HTMLElement;
        if (btn) btn.focus();
      });
    } else if (changed.has('_discardDialogOpen') && !this._discardDialogOpen) {
      this.removeAttribute('aria-live');
    }
    if (changed.has('_discardDialogOpen') && this._discardDialogOpen) {
      this.setAttribute('aria-live', 'polite');
      this.updateComplete.then(() => {
        const btn = this.shadowRoot?.querySelector('.discard-dialog-btn') as HTMLElement;
        if (btn) btn.focus();
      });
    } else if (changed.has('_discardDialogOpen') && !this._discardDialogOpen) {
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
      if (e.key === 'Escape' && this._discardDialogOpen) {
        this._discardDialogOpen = false;
      }
      if (e.key === 'Escape' && this._discardDialogOpen) {
        this._discardDialogOpen = false;
      }
    };
    document.addEventListener('keydown', this._onKeydown);
  }

  // --- Private methods ---

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
      if (min < 60) return `${min}m ago`;
      const h = Math.floor(min / 60);
      if (h < 24) return `${h}h ago`;
      return `${Math.floor(h / 24)}d ago`;
    } catch { return iso; }
  }

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    const body = {
      name: this._form.name,
      // Per D04_1-07: pass "" to clear external_id binding; null === omission
      external_id: this._form.external_id,
      // Per D04_1-07 / UpdateSkillRequest contract: use "" to CLEAR description; null === omission.
      // Sending "" causes the server to set description to SQL NULL.
      description: this._form.description,
      skill_type: this._form.skill_type,
      enabled: this._form.enabled,
      version: this._entity.version,
    };

    // Client-side validation
    // ajv standalone validators attach .errors dynamically; cast to access it.
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

  // --- Enable / Disable ---

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

  // --- Delete ---

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

  // --- Cancel ---

  _handleCancel(): void {
    if (!this._dirty) {
      this._navigate(`/orgs/${this.orgId}/skills`);
    } else {
      this._discardDialogOpen = true;
    }
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard' && this._conflictServer) {
      // Discard user edits — revert to server's latest state (from error.current, not stale _entity)
      const serverData = this._conflictServer as Record<string, unknown>;
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
      // Review / overwrite: bump local entity version to server version so next PATCH uses correct version
      // This prevents guaranteed re-409 on the next save attempt.
      if (this._entity && 'version' in this._conflictServer) {
        this._entity = { ...this._entity, version: this._conflictServer['version'] as number };
      }
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
        .userValue=${this._form as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
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
            <h3 id="confirm-title" class="confirm-title">Delete skill ${this._entity?.name ?? ''}?</h3>
            <p class="confirm-body">This is permanent and cannot be undone.</p>
            <div style="margin-bottom: 20px;">
              <sl-input
                placeholder="Type skill name to confirm"
                .value=${this._deleteConfirmName}
                @sl-input=${(e: Event) => {
                  this._deleteConfirmName = (e.target as HTMLInputElement).value;
                }}
                aria-label="Type skill name to confirm deletion"
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

  private _renderDiscardDialog() {
    return html`
      ${when(this._discardDialogOpen, () => html`
        <div class="confirm-overlay" role="presentation" @click=${() => { this._discardDialogOpen = false; }}>
          <div class="confirm-panel"
               role="dialog"
               aria-modal="true"
               aria-labelledby="discard-title"
               @click=${(e: Event) => e.stopPropagation()}>
            <h3 id="discard-title" class="confirm-title">Discard your changes?</h3>
            <p class="confirm-body">Unsaved edits will be lost.</p>
            <div class="action-row">
              <sl-button
                variant="default"
                size="small"
                @click=${() => { this._discardDialogOpen = false; }}
              >Keep editing</sl-button>
              <sl-button
                variant="danger"
                size="small"
                class="discard-dialog-btn"
                @click=${() => {
                  this._discardDialogOpen = false;
                  this._navigate(`/orgs/${this.orgId}/skills`);
                }}
              >Discard</sl-button>
            </div>
          </div>
        </div>
      `)}
    `;
  }

  private _renderTopBar() {
    return html`
      <div class="top-bar">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Skills
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

      <!-- Field 1: code (read-only always, D04_1-02: code is immutable post-create) -->
      <div class="form-group">
        <or-code-input
          .value=${entity.code}
          .readonly=${true}
        ></or-code-input>
      </div>

      <!-- Field 2: name (required) -->
      <!-- Use .value property binding (not value= attribute) for Shoelace programmatic resets -->
      <div class="form-group">
        <sl-input
          label="Name"
          .value=${this._form.name}
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

      <!-- Field 3: external_id (optional) -->
      <div class="form-group">
        <sl-input
          label="External ID"
          .value=${this._form.external_id}
          help-text="Optional integration mapping. Pass empty to clear."
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
      </div>

      <!-- Field 4: description (optional textarea) -->
      <div class="form-group">
        <sl-textarea
          label="Description"
          .value=${this._form.description}
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, description: (e.target as HTMLTextAreaElement).value };
            this._markDirty();
          }}
        ></sl-textarea>
      </div>

      <!-- Field 5: skill_type (required, freeform text; server validates against allowed values) -->
      <div class="form-group">
        <sl-input
          label="Skill Type"
          .value=${this._form.skill_type}
          required
          ?invalid=${!!this._fieldErrors['skill_type']}
          help-text="e.g. support, technical, billing"
          @sl-input=${(e: Event) => {
            this._form = { ...this._form, skill_type: (e.target as HTMLInputElement).value };
            this._markDirty();
          }}
        ></sl-input>
        ${when(
          this._fieldErrors['skill_type'],
          () => html`<div class="field-error">${this._fieldErrors['skill_type']}</div>`
        )}
      </div>

      <!-- Field 6: enabled switch -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._form.enabled}
          @sl-change=${(e: Event) => {
            this._form = { ...this._form, enabled: (e.target as HTMLInputElement).checked };
            this._markDirty();
          }}
        >Enabled</sl-switch>
      </div>

      <!-- Footer metadata bar (D6-V-14): version · updated · created -->
      <div class="footer-meta">
        version ${entity.version}
        ·
        <sl-tooltip content="${entity.updated_at ?? ''}">
          <span>updated ${this._relativeTime(entity.updated_at ?? '')}</span>
        </sl-tooltip>
        · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
      </div>

      <div class="bottom-bar">
        <sl-button
          variant="default"
          @click=${this._handleCancel}
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
      ${this._renderDiscardDialog()}

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
    'or-skill-detail': OrSkillDetail;
  }
}
