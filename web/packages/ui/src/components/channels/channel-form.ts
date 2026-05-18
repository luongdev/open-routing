// Phase 6 Plan 10 Task 2: <or-channel-form> — 3-step wizard for Channel create.
// Steps: Basics (code/name/channel_type/external_id/enabled) → Default queue (or-queue-picker) → Review.
// D6-17: multi-step for Channel — same or-form-wizard component with steps count=3.
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateChannel called on final submit.
// T-06-10-01: channel_type enum enforced client-side by ajv + server-side 422.
// Step 2: default_queue_id is optional (null allowed) per plan.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateChannel from '../../validators/CreateChannelRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/form-wizard.js';
import '../primitives/queue-picker.js';

type ChannelType = 'voice' | 'chat' | 'email';

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
 * <or-channel-form> — 3-step wizard for creating a new Channel.
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
      padding: 24px;
      max-width: 640px;
    }

    .page-header {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 24px;
    }

    .page-title {
      font-size: var(--or-text-display, 24px);
      font-weight: 700;
      color: var(--or-color-text-strong, #171717);
      margin: 0;
    }

    .form-group {
      margin-bottom: 16px;
    }

    .field-label {
      font-size: 14px;
      font-weight: 500;
      margin-bottom: 4px;
      display: block;
    }

    .field-error {
      font-size: 12px;
      color: var(--sl-color-danger-500, #d92d20);
      margin-top: 4px;
    }

    .step-helper {
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
      margin-bottom: 16px;
    }

    .review-section {
      background: var(--or-color-card-bg, #fff);
      border: 1px solid var(--or-color-divider, #e5e5e5);
      border-radius: 4px;
      padding: 16px;
      margin-bottom: 16px;
    }

    .review-row {
      display: flex;
      gap: 8px;
      margin-bottom: 8px;
      font-size: 14px;
    }

    .review-label {
      flex: 0 0 140px;
      font-weight: 600;
      color: var(--or-color-text-muted, #737373);
    }

    .review-value {
      color: var(--or-color-text-body, #404040);
      word-break: break-all;
    }

    .review-value code {
      font-family: var(--or-font-mono, monospace);
      color: var(--or-color-code-fg, #1f6e77);
      font-size: 13px;
    }

    .wizard-nav {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      justify-content: flex-end;
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // Expose steps for test inspection
  _wizardSteps: OrFormWizardStep[] = WIZARD_STEPS;

  // --- Internal state ---
  @state() accessor _currentStep = 0;
  @state() accessor _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    channel_type: '',
    default_queue_id: null,
    enabled: true,
  };
  @state() accessor _errors: Record<string, string> = {};
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _submitting = false;

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
      // Validate Step 1: code, name, channel_type are required
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

    // Build validation body — omit default_queue_id if null to work around
    // generated ajv validator's allOf+nullable UUID handling (erroneously fails null).
    // The actual POST body includes default_queue_id: null (explicitly serialized).
    const validationBody: Record<string, unknown> = {
      code: this._formData.code,
      name: this._formData.name,
      channel_type: this._formData.channel_type as ChannelType,
      enabled: this._formData.enabled,
    };
    if (this._formData.external_id) {
      validationBody['external_id'] = this._formData.external_id;
    }
    // Only include default_queue_id in validation body if it's a non-null UUID string
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
        // 409 duplicate_code — back to step 1 with inline error
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

  // --- Step content renderers ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the channel's identity. Code cannot be changed after create.</p>

      <div class="form-group">
        <or-code-input
          .value=${this._formData.code}
          .required=${true}
          @or-code-input=${(e: CustomEvent) => {
            this._formData = { ...this._formData, code: e.detail.value };
            if (this._errors['code']) {
              this._errors = { ...this._errors, code: '' };
            }
          }}
        ></or-code-input>
        ${when(
          this._errors['code'],
          () => html`<div class="field-error">${this._errors['code']}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-input
          label="Name"
          required
          value=${this._formData.name}
          ?invalid=${!!this._errors['name']}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, name: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
        ${when(
          this._errors['name'],
          () => html`<div class="field-error">${this._errors['name']}</div>`
        )}
      </div>

      <!-- channel_type: required single-select (voice/chat/email) -->
      <div class="form-group">
        <label class="field-label">
          Channel Type <span style="color:var(--sl-color-danger-500)">*</span>
        </label>
        <sl-select
          .value=${this._formData.channel_type}
          placeholder="Select channel type"
          ?invalid=${!!this._errors['channel_type']}
          @sl-change=${(e: Event) => {
            const val = (e.target as HTMLElement & { value: string }).value;
            this._formData = { ...this._formData, channel_type: val as ChannelType };
            if (this._errors['channel_type']) {
              this._errors = { ...this._errors, channel_type: '' };
            }
          }}
          aria-label="Channel type"
        >
          <sl-option value="voice">voice</sl-option>
          <sl-option value="chat">chat</sl-option>
          <sl-option value="email">email</sl-option>
        </sl-select>
        ${when(
          this._errors['channel_type'],
          () => html`<div class="field-error">${this._errors['channel_type']}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-input
          label="External ID"
          value=${this._formData.external_id}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
      </div>

      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.enabled}
          @sl-change=${(e: Event) => {
            this._formData = { ...this._formData, enabled: (e.target as HTMLInputElement).checked };
          }}
        >Enabled</sl-switch>
      </div>

      <div class="wizard-nav">
        <sl-button
          variant="default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels`)}
        >Cancel</sl-button>
        <sl-button variant="primary" @click=${this._handleNext}>Next: Default queue →</sl-button>
      </div>
    `;
  }

  private _renderDefaultQueueStep() {
    return html`
      <p class="step-helper">Optionally assign a default queue. Channels can route to a queue automatically.</p>

      <div class="form-group">
        <label class="field-label">Default Queue <span style="color:var(--or-color-text-muted,#737373);font-weight:400">(optional)</span></label>
        <or-queue-picker
          .orgId=${this.orgId}
          .client=${this.client}
          .value=${this._formData.default_queue_id}
          @or-queue-picker-change=${(e: CustomEvent) => {
            this._formData = { ...this._formData, default_queue_id: e.detail.queueId };
          }}
        ></or-queue-picker>
      </div>

      <div class="wizard-nav">
        <sl-button variant="default" @click=${this._handleBack}>← Back</sl-button>
        <sl-button variant="primary" @click=${this._handleNext}>Next: Review →</sl-button>
      </div>
    `;
  }

  private _renderReviewStep() {
    const { code, name, external_id, channel_type, default_queue_id, enabled } = this._formData;

    return html`
      <div class="review-section">
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
          <sl-alert variant="danger" open style="margin-bottom:16px">
            <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
            ${this._apiError}
            <sl-button size="small" @click=${this._handleSubmit}>Retry</sl-button>
          </sl-alert>
        `
      )}

      <div class="wizard-nav">
        <sl-button variant="default" @click=${this._handleBack}>← Back</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting
            ? html`<sl-spinner></sl-spinner> Creating…`
            : 'Create channel'}
        </sl-button>
      </div>
    `;
  }

  // --- Main render ---

  override render() {
    return html`
      <div class="page-header">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Channels
        </sl-button>
        <h1 class="page-title">Create channel</h1>
      </div>

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
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-channel-form': OrChannelForm;
  }
}
