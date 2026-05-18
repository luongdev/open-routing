// Phase 6 Plan 08 Task 2: <or-queue-form> — Single-step wizard for Queue create.
// D6-17: Single-step form (queue is a simple entity, no multi-step needed).
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateQueue called on submit; channel_types required minItems=1.
// channel_types: sl-select multiple with voice/chat/email options.
// priority and acw_sec: integer fields (type="number" step="1").

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateQueue from '../../validators/CreateQueueRequest.js';

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

type ChannelType = 'voice' | 'chat' | 'email';

// Single-step wizard (D6-17: Queues use single-step form)
const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
];

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
 * <or-queue-form> — Single-step wizard for creating a new Queue.
 *
 * Fields: code (or-code-input), name, external_id, channel_types (sl-select multiple),
 * priority (number), acw_sec (number), enabled (sl-switch).
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

    .wizard-nav {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      justify-content: flex-end;
    }

    .step-helper {
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
      margin-bottom: 16px;
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // --- Internal state ---
  @state() private accessor _currentStep = 0;
  @state() accessor _formData: QueueFormData = {
    code: '',
    name: '',
    external_id: '',
    channel_types: [],
    priority: 0,
    acw_sec: 0,
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

  // --- channel_types change ---

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
    if (this._errors['channel_types']) {
      this._errors = { ...this._errors, channel_types: '' };
    }
  }

  // --- Submit ---

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;

    // Validate channel_types manually first (required, minItems=1)
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

    // Client-side full validation via ajv
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

  // --- Step content renderer ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the queue's identity. Code cannot be changed after create.</p>

      <!-- code -->
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

      <!-- name -->
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

      <!-- external_id -->
      <div class="form-group">
        <sl-input
          label="External ID"
          value=${this._formData.external_id}
          @sl-input=${(e: Event) => {
            this._formData = {
              ...this._formData,
              external_id: (e.target as HTMLInputElement).value,
            };
          }}
        ></sl-input>
        <div class="field-helper">Optional integration mapping.</div>
      </div>

      <!-- channel_types — sl-select multiple; required; minItems=1 -->
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
          this._errors['channel_types'],
          () => html`<div class="field-error">${this._errors['channel_types']}</div>`
        )}
      </div>

      <!-- priority -->
      <div class="form-group">
        <sl-input
          label="Priority"
          type="number"
          min="0"
          step="1"
          value=${String(this._formData.priority)}
          required
          ?invalid=${!!this._errors['priority']}
          @sl-input=${(e: Event) => {
            const v = parseInt((e.target as HTMLInputElement).value, 10);
            this._formData = { ...this._formData, priority: isNaN(v) ? 0 : v };
          }}
        ></sl-input>
        ${when(
          this._errors['priority'],
          () => html`<div class="field-error">${this._errors['priority']}</div>`
        )}
      </div>

      <!-- acw_sec -->
      <div class="form-group">
        <sl-input
          label="After-Call Work (seconds)"
          type="number"
          min="0"
          step="1"
          value=${String(this._formData.acw_sec)}
          required
          ?invalid=${!!this._errors['acw_sec']}
          @sl-input=${(e: Event) => {
            const v = parseInt((e.target as HTMLInputElement).value, 10);
            this._formData = { ...this._formData, acw_sec: isNaN(v) ? 0 : v };
          }}
        ></sl-input>
        <div class="field-helper">seconds</div>
        ${when(
          this._errors['acw_sec'],
          () => html`<div class="field-error">${this._errors['acw_sec']}</div>`
        )}
      </div>

      <!-- enabled -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.enabled}
          @sl-change=${(e: Event) => {
            this._formData = {
              ...this._formData,
              enabled: (e.target as HTMLInputElement).checked,
            };
          }}
        >Enabled</sl-switch>
      </div>

      ${when(
        this._apiError,
        () => html`
          <sl-alert variant="danger" open style="margin-bottom:16px">
            <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
            ${this._apiError}
          </sl-alert>
        `
      )}

      <div class="wizard-nav">
        <sl-button
          variant="default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? html`<sl-spinner></sl-spinner> Creating…` : 'Create queue'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Queues
        </sl-button>
        <h1 class="page-title">Create queue</h1>
      </div>

      <or-form-wizard
        .steps=${WIZARD_STEPS}
        .currentStep=${this._currentStep}
        .hideNav=${true}
      >
        <div slot="step-basics">
          ${this._renderBasicsStep()}
        </div>
      </or-form-wizard>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-form': OrQueueForm;
  }
}
