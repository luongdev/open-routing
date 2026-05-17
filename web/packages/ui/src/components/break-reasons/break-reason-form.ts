// Phase 6 Plan 09 Task 2: <or-break-reason-form> — Single-step wizard for BreakReason create.
// D6-17: single-step for BreakReason (simple entity).
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateBreakReason called on submit.
// UI-SPEC §5.5 D6-V-16: Fields: code, name, external_id, routable (default true), display_order (required), enabled.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateBreakReason from '../../validators/CreateBreakReasonRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/form-wizard.js';

const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
];

interface FormData {
  code: string;
  name: string;
  external_id: string;
  routable: boolean;
  display_order: number | '';
  enabled: boolean;
}

/**
 * <or-break-reason-form> — Single-step wizard for creating a new BreakReason.
 *
 * Single step (Basics): code, name, external_id, routable (default true), display_order (required), enabled.
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/break-reasons/{newId}
 * On 409 duplicate_code → inline code error
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-break-reason-form')
export class OrBreakReasonForm extends LitElement {
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

    .helper-text {
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
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  // --- Internal state ---
  @state() private _currentStep = 0;
  @state() _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    routable: true,        // default true per UI-SPEC D6-V-16
    display_order: '',     // required; empty = invalid
    enabled: true,
  };
  @state() _errors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _submitting = false;

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

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;

    // Validate: display_order required
    const displayOrderVal = this._formData.display_order;
    if (displayOrderVal === '' || displayOrderVal === null || displayOrderVal === undefined) {
      this._errors = { display_order: 'Display order is required.' };
      return;
    }

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || null,
      routable: this._formData.routable,
      display_order: Number(this._formData.display_order),
      enabled: this._formData.enabled,
    };

    // Client-side full validation via ajv
    const validateFn = validateCreateBreakReason as unknown as {
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
      const result = await this.client.POST('/v1/orgs/{org_id}/break-reasons' as never, {
        params: { path: { org_id: this.orgId } },
        body,
      } as never);

      const { data, error } = result as { data: { id: string; code: string } | null; error: unknown };

      if (error) {
        // 409 duplicate_code → inline code error
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
        // Dispatch entity-created event
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'break-reason' },
            bubbles: true,
            composed: true,
          })
        );
        // Navigate to detail page
        this._navigate(`/orgs/${this.orgId}/break-reasons/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  // --- Step content renderer ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the break reason. Code cannot be changed after create.</p>

      <!-- code → or-code-input required -->
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

      <!-- name → sl-input required -->
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

      <!-- external_id → sl-input optional -->
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
      </div>

      <!-- routable → sl-switch, default checked=true per UI-SPEC D6-V-16 -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.routable}
          @sl-change=${(e: Event) => {
            this._formData = {
              ...this._formData,
              routable: (e.target as HTMLInputElement).checked,
            };
          }}
        >Routable</sl-switch>
        <div class="helper-text">When on, agents on this break can still receive routed interactions.</div>
      </div>

      <!-- display_order → sl-input type="number" min="0" step="1" required -->
      <div class="form-group">
        <sl-input
          label="Display Order"
          type="number"
          min="0"
          step="1"
          value=${this._formData.display_order === '' ? '' : String(this._formData.display_order)}
          required
          ?invalid=${!!this._errors['display_order']}
          @sl-input=${(e: Event) => {
            const val = (e.target as HTMLInputElement).value;
            this._formData = {
              ...this._formData,
              display_order: val === '' ? '' : parseInt(val, 10),
            };
          }}
        ></sl-input>
        <div class="helper-text">Lower values appear first in the break picker.</div>
        ${when(
          this._errors['display_order'],
          () => html`<div class="field-error">${this._errors['display_order']}</div>`
        )}
      </div>

      <!-- enabled → sl-switch, default true -->
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons`)}
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting
            ? html`<sl-spinner></sl-spinner> Creating…`
            : 'Create break reason'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Break reasons
        </sl-button>
        <h1 class="page-title">Create break reason</h1>
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
    'or-break-reason-form': OrBreakReasonForm;
  }
}
