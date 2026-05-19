import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateSkill from '../../validators/CreateSkillRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

import { nameToCode } from '../primitives/code-input.js';
import '../primitives/form-wizard.js';
import '../primitives/code-input.js';

const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
];

interface FormData {
  code: string;
  name: string;
  external_id: string;
  description: string;
  skill_type: string;
  enabled: boolean;
}

/**
 * <or-skill-form> — Single-step form for creating a new Skill (Ember layout).
 *
 * D6-17: single-step (steps.length === 1) → or-form-wizard hides stepper.
 * Fields: code, name, external_id, description, skill_type, enabled
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/skills/{newId}
 * On 409 duplicate_code → inline error on code field
 */
@customElement('or-skill-form')
export class OrSkillForm extends LitElement {
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

    textarea.uk-input {
      min-height: 80px;
      resize: vertical;
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

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  @state() private _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    description: '',
    skill_type: '',
    enabled: true,
  };
  @state() private _errors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _submitting = false;
  private _codeAutoFill = true;

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

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || null,
      description: this._formData.description || null,
      skill_type: this._formData.skill_type,
      enabled: this._formData.enabled,
    };

    const validateFn = validateCreateSkill as unknown as {
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
      const result = await this.client.POST('/v1/orgs/{org_id}/skills' as never, {
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
        this._navigate(`/orgs/${this.orgId}/skills/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the skill's identity. Code cannot be changed after create.</p>

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
        <label class="form-label form-label-required" for="skill-name">Name</label>
        <input
          id="skill-name"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.name}
          placeholder="e.g. Billing Support"
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
        <label class="form-label form-label-required" for="skill-type">Skill Type</label>
        <input
          id="skill-type"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.skill_type}
          placeholder="e.g. support, technical, billing"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, skill_type: (e.target as HTMLInputElement).value };
          }}
        />
        <div class="field-help">Category that groups this skill for routing rules.</div>
        ${when(
          this._errors['skill_type'],
          () => html`<div class="field-error">${this._errors['skill_type']}</div>`
        )}
      </div>

      <div class="form-row">
        <label class="form-label" for="skill-ext-id">External ID</label>
        <input
          id="skill-ext-id"
          class="uk-input"
          type="text"
          .value=${this._formData.external_id}
          placeholder="Optional reference from your system"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
          }}
        />
        <div class="field-help">Match an ID from your CRM or workforce management system.</div>
      </div>

      <div class="form-row">
        <label class="form-label" for="skill-description">Description</label>
        <textarea
          id="skill-description"
          class="uk-input"
          .value=${this._formData.description}
          placeholder="Optional description of this skill"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, description: (e.target as HTMLTextAreaElement).value };
          }}
        ></textarea>
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
        <div class="field-help">Disabled skills cannot be assigned to agents.</div>
      </div>

      ${when(
        this._apiError,
        () => html`
          <div class="api-error">
            <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
            ${this._apiError}
            <button
              class="uk-button uk-button-default uk-button-small"
              style="margin-left:auto"
              @click=${this._handleSubmit}
            >Retry</button>
          </div>
        `
      )}

      <div class="form-actions">
        <button
          class="uk-button uk-button-default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
        >Cancel</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting
            ? html`<span style="opacity:.7">Creating…</span>`
            : 'Create skill'}
        </button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <button
          class="back-btn"
          @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Skills
        </button>
        <h1 class="page-title">Create skill</h1>
      </div>

      <div class="form-card">
        <or-form-wizard
          .steps=${WIZARD_STEPS}
          .currentStep=${0}
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
    'or-skill-form': OrSkillForm;
  }
}
