// Phase 6 Plan 08 Task 2: <or-queue-form> — Single-step form for Queue create.
// D6-17: Single-step form (queue is a simple entity, no multi-step needed).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateQueue called on submit; channel_types required minItems=1.
// W0.1-12: redesigned to Ember healthcare-dashboard style (no sl-* in shadow).

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import validateCreateQueue from '../../validators/CreateQueueRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/code-input.js';

type ChannelType = 'voice' | 'chat' | 'email';

interface QueueFormData {
  code: string;
  name: string;
  external_id: string;
  channel_types: ChannelType[];
  priority: number;
  acw_sec: number;
  enabled: boolean;
}

/**
 * <or-queue-form> — Single-step form for creating a new Queue (Ember style).
 *
 * Fields: code, name, external_id, channel_types (checkbox group),
 * priority, acw_sec, enabled.
 *
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/queues/{newId}
 * On 409 duplicate_code → inline error on code field
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-queue-form')
export class OrQueueForm extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 20px 24px;
      max-width: 680px;
    }

    .page-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 16px;
    }

    .page-title {
      font-size: 22px;
      font-weight: 700;
      margin: 0;
      color: var(--foreground);
    }

    .back-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: none;
      border: none;
      cursor: pointer;
      font-size: 14px;
      color: var(--muted-foreground);
      padding: 4px 8px;
      border-radius: 6px;
      transition: color .12s, background .12s;
    }

    .back-btn:hover {
      color: var(--foreground);
      background: var(--muted);
    }

    .form-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      padding: 24px;
    }

    .step-helper {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0 0 14px;
    }

    .form-row {
      margin-bottom: 12px;
    }

    .form-label {
      display: block;
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      margin-bottom: 4px;
    }

    .form-label-required::after {
      content: ' *';
      color: var(--destructive);
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

    /* Channel type checkboxes */
    .channel-group {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
    }

    .channel-chip {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 6px 14px;
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

    /* Two-column layout for numeric fields */
    .two-col {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }

    @media (max-width: 500px) {
      .two-col { grid-template-columns: 1fr; }
    }

    /* Toggle switch */
    .switch-wrap {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      cursor: pointer;
      font-size: 14px;
      color: var(--foreground);
      user-select: none;
    }

    .switch-wrap input[type="checkbox"] {
      position: absolute;
      opacity: 0;
      width: 0;
      height: 0;
    }

    .switch-track {
      position: relative;
      width: 36px;
      height: 20px;
      border-radius: 9999px;
      background: var(--border);
      transition: background .15s;
      flex-shrink: 0;
    }

    .switch-wrap:has(input[type="checkbox"]:checked) .switch-track {
      background: var(--primary);
    }

    .switch-thumb {
      position: absolute;
      top: 2px;
      left: 2px;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: white;
      box-shadow: 0 1px 3px rgba(0,0,0,.2);
      transition: transform .15s;
    }

    .switch-wrap:has(input[type="checkbox"]:checked) .switch-thumb {
      transform: translateX(16px);
    }

    /* Error banner */
    .api-error {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
      border-radius: 8px;
      padding: 10px 14px;
      font-size: 13px;
      margin-bottom: 12px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    /* Form actions */
    .form-actions {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 16px;
      padding-top: 14px;
      border-top: 1px solid var(--border);
    }

    /* Loading spinner */
    .spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid var(--border);
      border-top-color: var(--primary-foreground);
      border-radius: 50%;
      animation: spin .6s linear infinite;
      margin-right: 6px;
      vertical-align: middle;
    }

    @keyframes spin { to { transform: rotate(360deg); } }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _formData: QueueFormData = {
    code: '',
    name: '',
    external_id: '',
    channel_types: [],
    priority: 0,
    acw_sec: 0,
    enabled: true,
  };
  @state() private _errors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _submitting = false;
  @state() private _codeEditable = false;
  @state() private _codeDraft = '';
  private _codeAutoFill = true;
  private _codeDraftTouched = false;

  // --- Navigation ---

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  // --- Channel types ---

  private _toggleChannelType(type: ChannelType): void {
    const current = this._formData.channel_types;
    const next = current.includes(type)
      ? current.filter((t) => t !== type)
      : [...current, type];
    this._formData = { ...this._formData, channel_types: next };
    if (this._errors['channel_types']) {
      this._errors = { ...this._errors, channel_types: '' };
    }
  }

  // --- Submit ---

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;
    if (!this._ensureCodeEditClosed()) return;

    if (!this._formData.channel_types || this._formData.channel_types.length === 0) {
      this._errors = { ...this._errors, channel_types: 'Select at least one channel type.' };
      return;
    }

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || null,
      channel_types: this._formData.channel_types,
      priority: this._formData.priority,
      acw_sec: this._formData.acw_sec,
      enabled: this._formData.enabled,
    };

    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateCreateQueue as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(body)) {
      const errors: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errors[field] = err.message ?? 'Invalid value';
      }
      this._errors = errors;
      return;
    }
    this._errors = {};
    this._apiError = null;
    this._submitting = true;

    try {
      const result = await this.client.POST('/v1/orgs/{org_id}/queues' as never, {
        params: { path: { org_id: this.orgId } },
        body,
      } as never);

      const { data, error } = result as { data: { id: string; code: string } | null; error: unknown };

      if (error) {
        // 409 duplicate_code — inline error on code
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'duplicate_code'
        ) {
          this._errors = { code: 'This code is already in use.' };
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Create failed';
        return;
      }

      if (data?.id) {
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'queue' },
            bubbles: true,
            composed: true,
          })
        );
        this._navigate(`/orgs/${this.orgId}/queues/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  private _clearFieldError(field: string): void {
    if (!this._errors[field]) return;
    const errors = { ...this._errors };
    delete errors[field];
    this._errors = errors;
  }

  private _handleCodeEdit(): void {
    const code = this._formData.code || nameToCode(this._formData.name);
    this._codeEditable = true;
    this._codeDraft = code;
    this._codeDraftTouched = false;
    if (this._codeAutoFill && this._formData.code !== code) {
      this._formData = { ...this._formData, code };
    }
  }

  private _handleCodeInput(e: CustomEvent<{ value: string }>): void {
    this._codeDraft = e.detail.value;
    this._codeDraftTouched = true;
    this._clearFieldError('code');
  }

  private _handleCodeSave(): void {
    const code = this._codeDraft;
    if (!code) {
      this._errors = { ...this._errors, code: 'Code is required.' };
      return;
    }
    if (!/^[a-z][a-z0-9_]{0,63}$/.test(code)) {
      this._errors = { ...this._errors, code: 'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores.' };
      return;
    }
    this._formData = { ...this._formData, code };
    this._codeAutoFill = false;
    this._codeEditable = false;
    this._codeDraft = '';
    this._codeDraftTouched = false;
    this._clearFieldError('code');
  }

  private _handleCodeCancel(): void {
    this._codeEditable = false;
    this._codeDraft = '';
    this._codeDraftTouched = false;
    if (this._codeAutoFill) {
      this._formData = { ...this._formData, code: nameToCode(this._formData.name) };
    }
    this._clearFieldError('code');
  }

  private _handleNameInput(name: string): void {
    const code = nameToCode(name);
    this._formData = {
      ...this._formData,
      name,
      ...(this._codeAutoFill ? { code } : {}),
    };
    if (this._codeEditable && this._codeAutoFill && !this._codeDraftTouched) {
      this._codeDraft = code;
    }
    if (name) this._clearFieldError('name');
    if (this._codeAutoFill && code) this._clearFieldError('code');
  }

  private _ensureCodeEditClosed(): boolean {
    if (!this._codeEditable) return true;
    this._errors = { ...this._errors, code: 'Save or cancel code before continuing.' };
    return false;
  }

  // --- Render ---

  override render() {
    const { code: _code, name, channel_types, priority, acw_sec, enabled, external_id } = this._formData;
    void _code;

    return html`
      <div class="page-header">
        <button
          class="back-btn"
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Queues
        </button>
        <h1 class="page-title">Create queue</h1>
      </div>

      <div class="form-card">
        <p class="step-helper">Define the queue's identity. Code cannot be changed after create.</p>

        <div class="form-row">
          <label class="form-label form-label-required" for="queue-name">Name</label>
          <input
            id="queue-name"
            class="uk-input"
            type="text"
            required
            .value=${name}
            placeholder="e.g. Billing Support Queue"
            @input=${(e: Event) => {
              this._handleNameInput((e.target as HTMLInputElement).value);
            }}
          />
          ${when(
            this._errors['name'],
            () => html`<div class="field-error">${this._errors['name']}</div>`
          )}
        </div>

        <div class="form-row">
          <or-code-input
            .value=${this._codeEditable ? this._codeDraft : this._formData.code}
            .required=${true}
            .readonly=${!this._codeEditable}
            .editButton=${!this._codeEditable}
            .saveButton=${this._codeEditable}
            .cancelButton=${this._codeEditable}
            .helperText=${this._codeEditable
              ? 'Custom code. Cannot be changed after create.'
              : 'Generated from name. Cannot be changed after create.'}
            @or-code-edit=${() => this._handleCodeEdit()}
            @or-code-input=${(e: CustomEvent<{ value: string }>) => this._handleCodeInput(e)}
            @or-code-save=${() => this._handleCodeSave()}
            @or-code-cancel=${() => this._handleCodeCancel()}
          ></or-code-input>
          ${when(
            this._errors['code'],
            () => html`<div class="field-error">${this._errors['code']}</div>`
          )}
        </div>

        <!-- external_id -->
        <div class="form-row">
          <label class="form-label" for="queue-ext-id">External ID</label>
          <input
            id="queue-ext-id"
            class="uk-input"
            type="text"
            .value=${external_id}
            placeholder="Optional integration reference"
            @input=${(e: Event) => {
              this._formData = {
                ...this._formData,
                external_id: (e.target as HTMLInputElement).value,
              };
            }}
          />
          <div class="field-help">Optional integration mapping.</div>
        </div>

        <!-- channel_types — pill checkboxes -->
        <div class="form-row">
          <label class="form-label form-label-required">Channel Types</label>
          <div class="channel-group">
            ${(['voice', 'chat', 'email'] as ChannelType[]).map(
              (ct) => html`
                <label class="channel-chip">
                  <input
                    type="checkbox"
                    ?checked=${channel_types.includes(ct)}
                    @change=${() => this._toggleChannelType(ct)}
                  />
                  <uk-icon
                    icon=${ct === 'voice' ? 'phone' : ct === 'chat' ? 'message-circle' : 'mail'}
                    width="14"
                    height="14"
                  ></uk-icon>
                  ${ct}
                </label>
              `
            )}
          </div>
          ${when(
            this._errors['channel_types'],
            () => html`<div class="field-error">${this._errors['channel_types']}</div>`
          )}
        </div>

        <!-- priority + acw_sec side by side -->
        <div class="two-col">
          <div class="form-row">
            <label class="form-label form-label-required" for="queue-priority">Priority</label>
            <input
              id="queue-priority"
              class="uk-input"
              type="number"
              min="0"
              step="1"
              .value=${String(priority)}
              required
              @input=${(e: Event) => {
                const v = parseInt((e.target as HTMLInputElement).value, 10);
                this._formData = { ...this._formData, priority: isNaN(v) ? 0 : v };
              }}
            />
            ${when(
              this._errors['priority'],
              () => html`<div class="field-error">${this._errors['priority']}</div>`
            )}
          </div>

          <div class="form-row">
            <label class="form-label form-label-required" for="queue-acw">After-Call Work (s)</label>
            <input
              id="queue-acw"
              class="uk-input"
              type="number"
              min="0"
              step="1"
              .value=${String(acw_sec)}
              required
              @input=${(e: Event) => {
                const v = parseInt((e.target as HTMLInputElement).value, 10);
                this._formData = { ...this._formData, acw_sec: isNaN(v) ? 0 : v };
              }}
            />
            ${when(
              this._errors['acw_sec'],
              () => html`<div class="field-error">${this._errors['acw_sec']}</div>`
            )}
          </div>
        </div>

        <!-- enabled toggle -->
        <div class="form-row">
          <label class="switch-wrap">
            <input
              type="checkbox"
              .checked=${enabled}
              @change=${(e: Event) => {
                this._formData = {
                  ...this._formData,
                  enabled: (e.target as HTMLInputElement).checked,
                };
              }}
            />
            <span class="switch-track"><span class="switch-thumb"></span></span>
            <span>Enabled</span>
          </label>
          <div class="field-help" style="margin-top:6px">Disabled queues do not receive new interactions.</div>
        </div>

        ${this._apiError
          ? html`
              <div class="api-error">
                <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
                ${this._apiError}
              </div>
            `
          : nothing}

        <div class="form-actions">
          <button
            class="uk-button uk-button-default"
            @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
          >Cancel</button>
          <button
            class="uk-button uk-button-primary"
            ?disabled=${this._submitting}
            @click=${this._handleSubmit}
          >
            ${this._submitting
              ? html`<span class="spinner"></span>Creating…`
              : 'Create queue'}
          </button>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-form': OrQueueForm;
  }
}
