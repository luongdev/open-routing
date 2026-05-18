// Phase 6 Plan 11 Task 2: <or-adapter-form> — Single-step wizard for Adapter create.
// D6-17: Adapter is a SIMPLE entity — single-step form (no Skills or Review step).
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateAdapter called on submit.
// Config field: sl-textarea with monospace font + JSON validation; empty → null.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateAdapter from '../../validators/CreateAdapterRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/textarea/textarea.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/form-wizard.js';

// Single-step wizard (stepper hidden when steps.length <= 1 in or-form-wizard)
const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
];

interface FormData {
  code: string;
  name: string;
  external_id: string;
  adapter_type: string;
  configText: string;
  enabled: boolean;
}

/**
 * <or-adapter-form> — Single-step wizard for creating a new Adapter.
 *
 * Step 1 (Basics): code, name, external_id, adapter_type (required), config (JSONB textarea), enabled
 *
 * Config handling:
 * - sl-textarea with monospace font
 * - JSON.parse try/catch on input; "Config must be valid JSON" error copy
 * - Empty textarea → send config: null in POST body
 *
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/adapters/{newId}
 * On 409 duplicate_code → inline code error
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-adapter-form')
export class OrAdapterForm extends LitElement {
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
  @state() accessor _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    adapter_type: '',
    configText: '',
    enabled: true,
  };
  @state() private accessor _errors: Record<string, string> = {};
  @state() private accessor _configError = '';
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _submitting = false;
  private _codeAutoFill = true;

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

  // --- Config JSON validation ---

  private _handleConfigInput(e: Event): void {
    const value = (e.target as HTMLTextAreaElement).value;
    this._formData = { ...this._formData, configText: value };
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
  }

  // --- Submit ---

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;

    // Block if config JSON is invalid
    if (this._configError) return;

    // Validate code format client-side
    const errors: Record<string, string> = {};
    if (!this._formData.code) {
      errors['code'] = 'Code is required.';
    } else if (!/^[a-z][a-z0-9_]{0,63}$/.test(this._formData.code)) {
      errors['code'] = 'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores.';
    }
    if (!this._formData.name) {
      errors['name'] = 'Name is required.';
    }
    if (!this._formData.adapter_type) {
      errors['adapter_type'] = 'Adapter type is required.';
    }
    if (Object.keys(errors).length > 0) {
      this._errors = errors;
      return;
    }

    // Parse config
    let configPayload: Record<string, unknown> | null = null;
    if (this._formData.configText.trim() !== '') {
      try {
        configPayload = JSON.parse(this._formData.configText) as Record<string, unknown>;
      } catch {
        this._configError = 'Config must be valid JSON';
        return;
      }
    } else {
      configPayload = null;
    }

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || null,
      adapter_type: this._formData.adapter_type,
      config: configPayload,
      enabled: this._formData.enabled,
    };

    // Client-side full validation via ajv
    const validateFn = validateCreateAdapter as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(body)) {
      const errs: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errs[field] = err.message ?? 'Invalid value';
      }
      this._errors = errs;
      return;
    }
    this._errors = {};
    this._apiError = null;
    this._submitting = true;

    try {
      const result = await this.client.POST('/v1/orgs/{org_id}/adapters' as never, {
        params: { path: { org_id: this.orgId } },
        body,
      } as never);

      const { data, error } = result as { data: { id: string; code: string } | null; error: unknown };

      if (error) {
        // 409 duplicate_code — inline code error
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
            detail: { entityId: data.id, entityType: 'adapter' },
            bubbles: true,
            composed: true,
          })
        );
        this._navigate(`/orgs/${this.orgId}/adapters/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  // --- Step content renderer ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the adapter's identity and configuration. Code cannot be changed after create.</p>

      <div class="form-group">
        <or-code-input
          .value=${this._formData.code}
          .required=${true}
          @or-code-input=${(e: CustomEvent) => {
            this._codeAutoFill = false;
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
            const name = (e.target as HTMLInputElement).value;
            this._formData = {
              ...this._formData,
              name,
              ...(this._codeAutoFill ? { code: nameToCode(name) } : {}),
            };
          }}
        ></sl-input>
        ${when(
          this._errors['name'],
          () => html`<div class="field-error">${this._errors['name']}</div>`
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
        <sl-input
          label="Adapter Type"
          required
          value=${this._formData.adapter_type}
          ?invalid=${!!this._errors['adapter_type']}
          placeholder="e.g. freeswitch, sip, webrtc"
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, adapter_type: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
        ${when(
          this._errors['adapter_type'],
          () => html`<div class="field-error">${this._errors['adapter_type']}</div>`
        )}
      </div>

      <div class="form-group">
        <sl-textarea
          label="Config (JSON)"
          rows="8"
          style="font-family: monospace; white-space: pre;"
          value=${this._formData.configText}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting
            ? html`<sl-spinner></sl-spinner> Creating…`
            : 'Create adapter'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Adapters
        </sl-button>
        <h1 class="page-title">Create adapter</h1>
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
    'or-adapter-form': OrAdapterForm;
  }
}
