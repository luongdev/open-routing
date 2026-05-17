// Phase 6 Plan 05 Task 3: <or-agent-form> — 3-step wizard for Agent create.
// Steps: Basics (code/name/email/enabled) → Skills → Review & Create.
// Uses or-form-wizard with 3 steps; stepper visible (steps > 1).
// D6-17: multi-step for Agent — Basics → Skills → Review.
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST/GET — never direct fetch().
// ajv validateCreateAgent called on final submit.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateAgent from '../../validators/CreateAgentRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/menu/menu.js';
import '@shoelace-style/shoelace/dist/components/menu-item/menu-item.js';
import '@shoelace-style/shoelace/dist/components/dropdown/dropdown.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/badge/badge.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/form-wizard.js';

const WIZARD_STEPS: OrFormWizardStep[] = [
  { key: 'basics', label: 'Basics' },
  { key: 'skills', label: 'Skills' },
  { key: 'review', label: 'Review' },
];

interface FormData {
  code: string;
  name: string;
  email: string;
  external_id: string;
  enabled: boolean;
}

interface SkillRow {
  skill_id: string;
  name?: string;
  proficiency: number;
}

/**
 * <or-agent-form> — 3-step wizard for creating a new Agent.
 *
 * Step 1 (Basics): code, name, email, external_id, enabled
 * Step 2 (Skills): initial skill assignments (optional)
 * Step 3 (Review): summary + "Create agent" button → POST
 *
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/agents/{newId}
 * On 409 duplicate_code → back to Step 1 with inline code error
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-agent-form')
export class OrAgentForm extends LitElement {
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

    .skills-section {
      padding: 8px 0;
    }

    .skills-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
      margin-bottom: 12px;
    }

    .skills-table th {
      padding: 8px 10px;
      text-align: left;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      color: var(--or-color-text-muted, #737373);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .skills-table td {
      padding: 8px 10px;
      vertical-align: middle;
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .skills-table code {
      font-family: var(--or-font-mono, monospace);
      color: var(--or-color-code-fg, #1f6e77);
      font-size: 13px;
    }

    .empty-skills {
      color: var(--or-color-text-muted, #737373);
      font-size: 14px;
      margin-bottom: 12px;
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
      flex: 0 0 120px;
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
  @state() private _formData: FormData = {
    code: '',
    name: '',
    email: '',
    external_id: '',
    enabled: true,
  };
  @state() private _assignedSkills: SkillRow[] = [];
  @state() private _errors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _submitting = false;
  @state() private _skillSearchResults: Array<{ id: string; name: string; code: string }> = [];
  @state() private _skillSearchDebounce?: ReturnType<typeof setTimeout>;

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
      // Validate Step 1: code (required + pattern), name (required), email (format if provided)
      const errors: Record<string, string> = {};

      if (!this._formData.code) {
        errors['code'] = 'Code is required.';
      } else if (!/^[a-z][a-z0-9_]{0,63}$/.test(this._formData.code)) {
        errors['code'] = 'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores.';
      }

      if (!this._formData.name) {
        errors['name'] = 'Name is required.';
      }

      // Email is optional but must be valid format if provided (Gemini HIGH #2)
      if (this._formData.email) {
        const emailRegex = /^[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/i;
        if (!emailRegex.test(this._formData.email)) {
          errors['email'] = 'Enter a valid email address.';
        }
      }

      if (Object.keys(errors).length > 0) {
        this._errors = errors;
        return;
      }
      this._errors = {};
    }

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

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      email: this._formData.email,
      external_id: this._formData.external_id || null,
      enabled: this._formData.enabled,
      skills: this._assignedSkills.map((s) => ({
        skill_id: s.skill_id,
        proficiency: s.proficiency,
      })),
    };

    // Client-side full validation
    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateCreateAgent as unknown as {
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
      const result = await this.client.POST('/v1/orgs/{org_id}/agents' as never, {
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
        // Dispatch entity-created event
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'agent' },
            bubbles: true,
            composed: true,
          })
        );
        // Navigate to detail page
        this._navigate(`/orgs/${this.orgId}/agents/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  // --- Skills ---

  private _handleSkillSearch(e: Event): void {
    const query = (e.target as HTMLInputElement).value;
    clearTimeout(this._skillSearchDebounce);
    this._skillSearchDebounce = setTimeout(async () => {
      if (!query) {
        this._skillSearchResults = [];
        return;
      }
      const skillsResult = await this.client.GET('/v1/orgs/{org_id}/skills' as never, {
        params: { path: { org_id: this.orgId }, query: { name: query, limit: 100 } },
      } as never);
      const skillsData = (skillsResult as { data?: unknown }).data;
      this._skillSearchResults = ((skillsData as { items?: unknown[] } | undefined)?.items ?? []) as Array<{
        id: string;
        name: string;
        code: string;
      }>;
    }, 300);
  }

  private _handleAddSkill(skill: { id: string; name: string; code: string }): void {
    if (this._assignedSkills.some((s) => s.skill_id === skill.id)) return;
    this._assignedSkills = [
      ...this._assignedSkills,
      { skill_id: skill.id, name: skill.name, proficiency: 5 },
    ];
    this._skillSearchResults = [];
  }

  private _handleRemoveSkill(skillId: string): void {
    this._assignedSkills = this._assignedSkills.filter((s) => s.skill_id !== skillId);
  }

  private _handleProficiencyChange(skillId: string, value: number): void {
    this._assignedSkills = this._assignedSkills.map((s) =>
      s.skill_id === skillId ? { ...s, proficiency: value } : s
    );
  }

  // --- Step content renderers ---

  private _renderBasicsStep() {
    const proficiencyOptions = Array.from({ length: 10 }, (_, i) => i + 1);
    void proficiencyOptions; // used in skills step

    return html`
      <p class="step-helper">Define the agent's identity. Code cannot be changed after create.</p>

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

      <div class="form-group">
        <sl-input
          label="Email"
          type="email"
          value=${this._formData.email}
          @sl-input=${(e: Event) => {
            this._formData = { ...this._formData, email: (e.target as HTMLInputElement).value };
          }}
        ></sl-input>
      </div>

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

      <div class="wizard-nav">
        <sl-button
          variant="default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
        >Cancel</sl-button>
        <sl-button variant="primary" @click=${this._handleNext}>Next: Skills →</sl-button>
      </div>
    `;
  }

  private _renderSkillsStep() {
    const proficiencyOptions = Array.from({ length: 10 }, (_, i) => i + 1);

    return html`
      <p class="step-helper">Assign initial skills. You can also edit skills after creating the agent.</p>

      <div class="skills-section">
        ${when(
          this._assignedSkills.length === 0,
          () => html`<p class="empty-skills">No skills assigned yet.</p>`,
          () => html`
            <table class="skills-table">
              <thead>
                <tr>
                  <th>Skill</th>
                  <th>Proficiency</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                ${this._assignedSkills.map(
                  (skill) => html`
                    <tr>
                      <td><code>${skill.name ?? skill.skill_id}</code></td>
                      <td>
                        <sl-select
                          size="small"
                          value=${String(skill.proficiency)}
                          @sl-change=${(e: Event) =>
                            this._handleProficiencyChange(
                              skill.skill_id,
                              parseInt((e.target as HTMLSelectElement).value, 10)
                            )}
                        >
                          ${proficiencyOptions.map(
                            (n) => html`<sl-option value=${String(n)}>${n}</sl-option>`
                          )}
                        </sl-select>
                      </td>
                      <td>
                        <sl-icon-button
                          name="x-lg"
                          label="Remove skill"
                          @click=${() => this._handleRemoveSkill(skill.skill_id)}
                        ></sl-icon-button>
                      </td>
                    </tr>
                  `
                )}
              </tbody>
            </table>
          `
        )}

        <sl-dropdown>
          <sl-button slot="trigger" size="small" variant="default">+ Add skill</sl-button>
          <sl-menu>
            <sl-input
              placeholder="Search skills…"
              size="small"
              @sl-input=${this._handleSkillSearch}
            ></sl-input>
            ${this._skillSearchResults.map(
              (skill) => html`
                <sl-menu-item @click=${() => this._handleAddSkill(skill)}>
                  ${skill.name} <code>${skill.code}</code>
                </sl-menu-item>
              `
            )}
            ${when(
              this._skillSearchResults.length === 0,
              () => html`<sl-menu-item disabled>Type to search skills</sl-menu-item>`
            )}
          </sl-menu>
        </sl-dropdown>
      </div>

      <div class="wizard-nav">
        <sl-button variant="default" @click=${this._handleBack}>← Back</sl-button>
        <sl-button variant="primary" @click=${this._handleNext}>Next: Review →</sl-button>
      </div>
    `;
  }

  private _renderReviewStep() {
    const { code, name, email, external_id, enabled } = this._formData;
    const skillsSummary =
      this._assignedSkills.length === 0
        ? 'None'
        : this._assignedSkills.map((s) => `${s.name ?? s.skill_id}:${s.proficiency}`).join(', ');

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
          <span class="review-label">Email</span>
          <span class="review-value">${email || '—'}</span>
        </div>
        <div class="review-row">
          <span class="review-label">External ID</span>
          <span class="review-value">${external_id || '—'}</span>
        </div>
        <div class="review-row">
          <span class="review-label">Enabled</span>
          <span class="review-value">${enabled ? 'Yes' : 'No'}</span>
        </div>
        <div class="review-row">
          <span class="review-label">Skills</span>
          <span class="review-value">
            ${this._assignedSkills.length} assigned
            ${this._assignedSkills.length > 0 ? html`(${skillsSummary})` : ''}
          </span>
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
            : 'Create agent'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Agents
        </sl-button>
        <h1 class="page-title">Create agent</h1>
      </div>

      <or-form-wizard
        .steps=${WIZARD_STEPS}
        .currentStep=${this._currentStep}
        .hideNav=${true}
      >
        <div slot="step-basics">
          ${this._renderBasicsStep()}
        </div>
        <div slot="step-skills">
          ${this._renderSkillsStep()}
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
    'or-agent-form': OrAgentForm;
  }
}
