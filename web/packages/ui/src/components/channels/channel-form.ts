// Phase 6 Plan 10 Task 2: <or-channel-form> — 3-step wizard for Channel create.
// Steps: Basics (code/name/channel_type/external_id/enabled) → Default queue (or-queue-picker) → Review.
// D6-17: multi-step for Channel — same or-form-wizard component with steps count=3.
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateChannel called on final submit.
// T-06-10-01: channel_type enum enforced client-side by ajv + server-side 422.
// Step 2: default_queue_id is optional (null allowed) per plan.
// W0.1-14: Ember dashboard style — adoptShadowSheets, uk-button/uk-input, uk-icon, zero sl-*.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateChannel from '../../validators/CreateChannelRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/form-wizard.js';
import '../primitives/code-input.js';
import '../primitives/queue-picker.js';

type ChannelType = 'voice' | 'chat' | 'email' | 'sms' | 'social';

const CHANNEL_TYPE_OPTIONS: Array<{ value: ChannelType; label: string }> = [
  { value: 'voice', label: 'Voice' },
  { value: 'chat', label: 'Chat' },
  { value: 'email', label: 'Email' },
  { value: 'sms', label: 'SMS' },
  { value: 'social', label: 'Social' },
];

const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
  { key: 'default-queue', label: 'Default queue' },
  { key: 'review', label: 'Review' },
];

interface FormData {
  code: string;
  name: string;
  external_id: string;
  channel_type: ChannelType | '';
  default_queue_id: string | null;
  enabled: boolean;
}

/**
 * <or-channel-form> — 3-step wizard for creating a new Channel (Ember dashboard style).
 *
 * Step 1 (Basics): code, name, channel_type (required), external_id, enabled
 * Step 2 (Default queue): or-queue-picker — optional, null allowed
 * Step 3 (Review): summary + "Create channel" button → POST
 *
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/channels/{newId}
 * On 409 duplicate_code → back to Step 1 with inline code error
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-channel-form')
export class OrChannelForm extends LitElement {
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

    /* CSS-only toggle switch */
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

    /* Review summary */
    .review-card {
      background: var(--muted);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px 16px;
      margin-bottom: 12px;
    }

    .review-row {
      display: flex;
      gap: 8px;
      margin-bottom: 6px;
      font-size: 14px;
    }

    .review-row:last-child {
      margin-bottom: 0;
    }

    .review-label {
      flex: 0 0 140px;
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted-foreground);
      padding-top: 1px;
    }

    .review-value {
      color: var(--foreground);
      word-break: break-all;
      font-size: 14px;
    }

    .review-value code {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--muted-foreground);
      font-size: 13px;
    }

    /* API error banner */
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

    /* Form action row */
    .form-actions {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 16px;
      padding-top: 14px;
      border-top: 1px solid var(--border);
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  // Expose steps for test inspection
  _wizardSteps: OrFormWizardStep[] = WIZARD_STEPS;

  // --- Internal state ---
  @state() _currentStep = 0;
  @state() _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    channel_type: '',
    default_queue_id: null,
    enabled: true,
  };
  @state() _errors: Record<string, string> = {};
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

  async _handleNext(): Promise<void> {
    if (this._currentStep === 0) {
      if (!this._ensureCodeEditClosed()) return;

      const errors: Record<string, string> = {};

      if (!this._formData.code) {
        errors['code'] = 'Code is required.';
      } else if (!/^[a-z][a-z0-9_]{0,63}$/.test(this._formData.code)) {
        errors['code'] = 'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores.';
      }

      if (!this._formData.name) {
        errors['name'] = 'Name is required.';
      }

      if (!this._formData.channel_type) {
        errors['channel_type'] = 'Channel type is required.';
      }

      if (Object.keys(errors).length > 0) {
        this._errors = errors;
        return;
      }
      this._errors = {};
    }

    // Step 2 → Step 3: queue is optional, advance unconditionally
    if (this._currentStep < WIZARD_STEPS.length - 1) {
      this._currentStep += 1;
    }
  }

  private _handleBack(): void {
    if (this._currentStep > 0) {
      this._currentStep -= 1;
    }
  }

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;
    if (!this._ensureCodeEditClosed()) {
      this._currentStep = 0;
      return;
    }

    // Build validation body — omit default_queue_id if null to work around
    // generated ajv validator's allOf+nullable UUID handling (erroneously fails null).
    const validationBody: Record<string, unknown> = {
      code: this._formData.code,
      name: this._formData.name,
      channel_type: this._formData.channel_type as ChannelType,
      enabled: this._formData.enabled,
    };
    if (this._formData.external_id) {
      validationBody['external_id'] = this._formData.external_id;
    }
    if (this._formData.default_queue_id !== null) {
      validationBody['default_queue_id'] = this._formData.default_queue_id;
    }

    // Client-side full validation (T-06-10-01 enum enforcement)
    const validateFn = validateCreateChannel as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(validationBody)) {
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

    // Full POST body includes explicit null for default_queue_id (nullable field)
    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || undefined,
      channel_type: this._formData.channel_type as ChannelType,
      default_queue_id: this._formData.default_queue_id,
      enabled: this._formData.enabled,
    };

    try {
      const result = await this.client.POST('/v1/orgs/{org_id}/channels' as never, {
        params: { path: { org_id: this.orgId } },
        body,
      } as never);

      const { data, error } = result as { data: { id: string; code: string } | null; error: unknown };

      if (error) {
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'duplicate_code'
        ) {
          this._currentStep = 0;
          this._errors = { code: 'This code is already in use.' };
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Create failed';
        return;
      }

      if (data?.id) {
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'channel' },
            bubbles: true,
            composed: true,
          })
        );
        this._navigate(`/orgs/${this.orgId}/channels/${data.id}`);
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

  // --- Step content renderers ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the channel's identity. Code cannot be changed after create.</p>

      <div class="form-row">
        <label class="form-label form-label-required" for="ch-name">Name</label>
        <input
          id="ch-name"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.name}
          placeholder="e.g. Main Voice Line"
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

      <div class="form-row">
        <label class="form-label form-label-required" for="ch-type">Channel Type</label>
        <select
          id="ch-type"
          class="uk-select"
          @change=${(e: Event) => {
            const val = (e.target as HTMLSelectElement).value as ChannelType;
            this._formData = { ...this._formData, channel_type: val };
            if (this._errors['channel_type']) {
              this._errors = { ...this._errors, channel_type: '' };
            }
          }}
          aria-label="Channel type"
        >
          <option value="" ?selected=${this._formData.channel_type === ''} disabled>Select channel type</option>
          ${CHANNEL_TYPE_OPTIONS.map(
            (opt) => html`<option value=${opt.value} ?selected=${this._formData.channel_type === opt.value}>${opt.label}</option>`
          )}
        </select>
        ${when(
          this._errors['channel_type'],
          () => html`<div class="field-error">${this._errors['channel_type']}</div>`
        )}
      </div>

      <div class="form-row">
        <label class="form-label" for="ch-ext-id">External ID</label>
        <input
          id="ch-ext-id"
          class="uk-input"
          type="text"
          .value=${this._formData.external_id}
          placeholder="Optional reference from your system"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
          }}
        />
      </div>

      <div class="form-row">
        <label class="switch-wrap">
          <input
            type="checkbox"
            .checked=${this._formData.enabled}
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
        <div class="field-help">Disabled channels will not accept incoming interactions.</div>
      </div>

      <div class="form-actions">
        <button
          class="uk-button uk-button-default"
          type="button"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels`)}
        >Cancel</button>
        <button class="uk-button uk-button-primary" type="button" @click=${this._handleNext}>Next: Default queue →</button>
      </div>
    `;
  }

  private _renderDefaultQueueStep() {
    return html`
      <p class="step-helper">Optionally assign a default queue. Channels can route to a queue automatically.</p>

      <div class="form-row">
        <label class="form-label">Default Queue <span style="font-weight:400;color:var(--muted-foreground)">(optional)</span></label>
        <or-queue-picker
          .orgId=${this.orgId}
          .client=${this.client}
          .value=${this._formData.default_queue_id}
          @or-queue-picker-change=${(e: CustomEvent) => {
            this._formData = { ...this._formData, default_queue_id: e.detail.queueId };
          }}
        ></or-queue-picker>
      </div>

      <div class="form-actions">
        <button class="uk-button uk-button-default" type="button" @click=${this._handleBack}>← Back</button>
        <button class="uk-button uk-button-primary" type="button" @click=${this._handleNext}>Next: Review →</button>
      </div>
    `;
  }

  private _renderReviewStep() {
    const { code, name, external_id, channel_type, default_queue_id, enabled } = this._formData;

    return html`
      <div class="review-card">
        <div class="review-row">
          <span class="review-label">Code</span>
          <span class="review-value"><code>${code}</code></span>
        </div>
        <div class="review-row">
          <span class="review-label">Name</span>
          <span class="review-value">${name}</span>
        </div>
        <div class="review-row">
          <span class="review-label">Channel Type</span>
          <span class="review-value">${channel_type || '—'}</span>
        </div>
        <div class="review-row">
          <span class="review-label">External ID</span>
          <span class="review-value">${external_id || '—'}</span>
        </div>
        <div class="review-row">
          <span class="review-label">Default Queue</span>
          <span class="review-value">
            ${default_queue_id
              ? html`<code>${default_queue_id.slice(0, 8)}…</code>`
              : '(none)'}
          </span>
        </div>
        <div class="review-row">
          <span class="review-label">Enabled</span>
          <span class="review-value">${enabled ? 'Yes' : 'No'}</span>
        </div>
      </div>

      ${when(
        this._apiError,
        () => html`
          <div class="api-error">
            <uk-icon icon="x" height="16" width="16"></uk-icon>
            ${this._apiError}
          </div>
        `
      )}

      <div class="form-actions">
        <button class="uk-button uk-button-default" type="button" @click=${this._handleBack}>← Back</button>
        <button
          class="uk-button uk-button-primary"
          type="button"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? 'Creating…' : 'Create channel'}
        </button>
      </div>
    `;
  }

  // --- Main render ---

  override render() {
    return html`
      <div class="page-header">
        <button
          class="back-btn"
          type="button"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Channels
        </button>
        <h1 class="page-title">Create channel</h1>
      </div>

      <div class="form-card">
        <or-form-wizard
          .steps=${WIZARD_STEPS}
          .currentStep=${this._currentStep}
          .hideNav=${true}
        >
          <div slot="step-basics">
            ${this._renderBasicsStep()}
          </div>
          <div slot="step-default-queue">
            ${this._renderDefaultQueueStep()}
          </div>
          <div slot="step-review">
            ${this._renderReviewStep()}
          </div>
        </or-form-wizard>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-channel-form': OrChannelForm;
  }
}
