// Phase 6 Plan 07 Task 2: <or-skill-form> — Single-step wizard for Skill create.
// D6-17: single-step for Skills — same or-form-wizard component, steps=[{key:'basics',label:'Basics'}].
//   Single step causes wizard to hide stepper and show "Create skill" header.
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateSkill called before POST.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateSkill from '../../validators/CreateSkillRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/textarea/textarea.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/form-wizard.js';

// Single step — stepper hidden (D6-17: simple entity single-step form)
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
 * <or-skill-form> — Single-step wizard for creating a new Skill.
 *
 * D6-17: single-step (steps.length === 1) → or-form-wizard hides stepper,
 * shows only "Create skill" header with Cancel + Create buttons.
 *
 * Fields: code, name, external_id, description, skill_type, enabled
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/skills/{newId}
 * On 409 duplicate_code → inline error on code field
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-skill-form')
export class OrSkillForm extends LitElement {
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
  @state() private accessor _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    description: '',
    skill_type: '',
    enabled: true,
  };
  @state() private accessor _errors: Record<string, string> = {};
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

  // --- Submit ---

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

    // Client-side validation
    // ajv standalone validators attach .errors dynamically; cast to access it.
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
        // 409 duplicate_code — inline error on code field
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'duplicate_code'
        ) {
          this._errors = { code: 'This code is already in use.' };
          return;
        }
        // 5xx or other — show danger alert
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

  // --- Step content ---

  private _renderBasicsStep() {
    return html`
      <p class="step-helper">Define the skill's identity. Code cannot be changed after create.</p>

      <!-- code (or-code-input, required, writable on create) -->
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

      <!-- name (required) -->
      <!-- Use .value property binding (not value= attribute) for Shoelace programmatic resets -->
      <div class="form-group">
        <sl-input
          label="Name"
          required
          .value=${this._formData.name}
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

      <!-- external_id (optional) -->
      <div class="form-group">
        <sl-input
          label="External ID"
          .value=${this._formData.external_id}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, external_id: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
      </div>

      <!-- description (optional textarea) -->
      <div class="form-group">
        <sl-textarea
          label="Description"
          .value=${this._formData.description}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, description: (e.target as HTMLTextAreaElement).value };
          }}
        ></sl-textarea>
      </div>

      <!-- skill_type (required, freeform string) -->
      <div class="form-group">
        <sl-input
          label="Skill Type"
          required
          .value=${this._formData.skill_type}
          ?invalid=${!!this._errors['skill_type']}
          help-text="e.g. support, technical, billing"
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, skill_type: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
        ${when(
          this._errors['skill_type'],
          () => html`<div class="field-error">${this._errors['skill_type']}</div>`
        )}
      </div>

      <!-- enabled switch (default true) -->
      <div class="form-group">
        <sl-switch
          ?checked=${this._formData.enabled}
          @sl-change=${(e: Event) => {
            this._formData = { ...this._formData, enabled: (e.target as HTMLInputElement).checked };
          }}
        >Enabled</sl-switch>
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
        <sl-button
          variant="default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
        >Cancel</sl-button>
        <sl-button
          variant="primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? html`<sl-spinner></sl-spinner> Creating…` : 'Create skill'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/skills`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Skills
        </sl-button>
        <h1 class="page-title">Create skill</h1>
      </div>

      <!-- Single step wizard (D6-17): steps.length === 1 → or-form-wizard hides stepper -->
      <or-form-wizard
        .steps=${WIZARD_STEPS}
        .currentStep=${0}
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
    'or-skill-form': OrSkillForm;
  }
}
