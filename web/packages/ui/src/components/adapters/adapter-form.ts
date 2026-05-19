// Phase 6 Plan 11 Task 2: <or-adapter-form> — Single-step form for Adapter create.
// D6-17: Adapter is a SIMPLE entity — single-step form (stepper hidden when steps.length <= 1).
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateAdapter called on submit.
// Config field: native <textarea> with JSON validation; empty → null.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateAdapter from '../../validators/CreateAdapterRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/form-wizard.js';
import '../primitives/code-input.js';

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

@customElement('or-adapter-form')
export class OrAdapterForm extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      max-width: 680px;
    }

    .page-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 24px;
    }

    .page-title {
      font-size: 24px;
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
      padding: 32px;
    }

    .step-helper {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0 0 20px;
    }

    .form-row {
      margin-bottom: 16px;
    }

    .form-label {
      display: block;
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      margin-bottom: 6px;
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

    /* Config textarea — same visual treatment as uk-input */
    .config-textarea {
      display: block;
      width: 100%;
      box-sizing: border-box;
      min-height: 160px;
      padding: 8px 12px;
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
      line-height: 1.5;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--foreground);
      resize: vertical;
      transition: border-color .12s, box-shadow .12s;
    }

    .config-textarea:focus,
    .config-textarea:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 25%, transparent);
    }

    .config-textarea::placeholder {
      color: var(--muted-foreground);
    }

    .config-textarea--error {
      border-color: var(--destructive);
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

    /* API error banner */
    .api-error {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
      border-radius: 8px;
      padding: 12px 16px;
      font-size: 13px;
      margin-bottom: 16px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    /* Form action row */
    .form-actions {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 24px;
      padding-top: 20px;
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

  // --- Internal state ---
  @state() private _currentStep = 0;
  @state() _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    adapter_type: '',
    configText: '',
    enabled: true,
  };
  @state() private _errors: Record<string, string> = {};
  @state() private _configError = '';
  @state() private _apiError: string | null = null;
  @state() private _submitting = false;
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

    if (this._configError) return;

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

      <div class="form-row">
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

      <div class="form-row">
        <label class="form-label form-label-required" for="adapter-name">Name</label>
        <input
          id="adapter-name"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.name}
          placeholder="e.g. FreeSWITCH Bridge - DC1"
          @input=${(e: Event) => {
            const name = (e.target as HTMLInputElement).value;
            this._formData = {
              ...this._formData,
              name,
              ...(this._codeAutoFill ? { code: nameToCode(name) } : {}),
            };
          }}
        />
        ${when(
          this._errors['name'],
          () => html`<div class="field-error">${this._errors['name']}</div>`
        )}
      </div>

      <div class="form-row">
        <label class="form-label" for="adapter-ext-id">External ID</label>
        <input
          id="adapter-ext-id"
          class="uk-input"
          type="text"
          .value=${this._formData.external_id}
          placeholder="Optional reference from your system"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
          }}
        />
        <div class="field-help">Match an ID from your telephony or integration platform.</div>
      </div>

      <div class="form-row">
        <label class="form-label form-label-required" for="adapter-type">Adapter Type</label>
        <input
          id="adapter-type"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.adapter_type}
          placeholder="e.g. freeswitch, livekit, twilio"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, adapter_type: (e.target as HTMLInputElement).value };
          }}
        />
        <div class="field-help">Free-text identifier for the adapter kind.</div>
        ${when(
          this._errors['adapter_type'],
          () => html`<div class="field-error">${this._errors['adapter_type']}</div>`
        )}
      </div>

      <div class="form-row">
        <label class="form-label" for="adapter-config">Config (JSON)</label>
        <textarea
          id="adapter-config"
          class="config-textarea ${this._configError ? 'config-textarea--error' : ''}"
          rows="8"
          placeholder='{"key": "value"}'
          .value=${this._formData.configText}
          @input=${this._handleConfigInput}
        ></textarea>
        ${when(
          this._configError,
          () => html`<div class="field-error">${this._configError}</div>`
        )}
        <div class="field-help">Optional JSON configuration blob. Leave blank to send null.</div>
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
        <div class="field-help">Disabled adapters do not receive routing traffic.</div>
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
        <button
          class="uk-button uk-button-default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
        >Cancel</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? 'Creating…' : 'Create adapter'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/adapters`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Adapters
        </button>
        <h1 class="page-title">Create adapter</h1>
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
        </or-form-wizard>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-adapter-form': OrAdapterForm;
  }
}
