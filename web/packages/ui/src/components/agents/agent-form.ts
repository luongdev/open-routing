// Phase 6 Plan 05 Task 3: <or-agent-form> — 3-step wizard for Agent create.
// Steps: Basics (code/name/email/enabled) → Skills → Review & Create.
// Uses or-form-wizard with 3 steps; stepper visible (steps > 1).
// D6-17: multi-step for Agent — Basics → Skills → Review.
// ADMIN-04: only this.client.POST/GET — never direct fetch().
// ajv validateCreateAgent called on final submit.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { OrFormWizardStep } from '../primitives/form-wizard.js';
import validateCreateAgent from '../../validators/CreateAgentRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/form-wizard.js';
import '../primitives/code-input.js';

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

    .form-section {
      margin-bottom: 16px;
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

    /* Toggle switch built purely with CSS + checkbox */
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

    /* Skills table */
    .skills-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
      margin-bottom: 8px;
    }

    .skills-table th {
      padding: 8px 10px;
      text-align: left;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }

    .skills-table th.col-proficiency { width: 130px; }
    .skills-table th.col-action { width: 40px; text-align: right; }

    .skills-table td {
      padding: 10px;
      vertical-align: middle;
      border-bottom: 1px solid var(--border);
    }

    .skills-table tr:last-child td { border-bottom: none; }

    .skill-name-cell {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      color: var(--foreground);
    }

    .skill-name-cell::before {
      content: '';
      display: inline-block;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: color-mix(in oklch, var(--primary) 60%, transparent);
      flex-shrink: 0;
    }

    .skills-table .uk-select {
      width: 100%;
      max-width: 110px;
    }

    .empty-skills {
      color: var(--muted-foreground);
      font-size: 14px;
      margin-bottom: 8px;
      padding: 8px 0;
    }

    /* Skill search picker */
    .skill-picker {
      position: relative;
      display: inline-block;
    }

    .skill-dropdown {
      position: absolute;
      top: calc(100% + 4px);
      left: 0;
      z-index: 100;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      box-shadow: var(--shadow-md);
      min-width: 280px;
      padding: 8px;
    }

    .skill-dropdown input {
      margin-bottom: 8px;
      width: 100%;
      box-sizing: border-box;
    }

    .skill-results {
      max-height: 200px;
      overflow-y: auto;
    }

    .skill-result-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 7px 8px;
      border-radius: 6px;
      cursor: pointer;
      font-size: 13px;
      color: var(--foreground);
      transition: background .1s;
      width: 100%;
      background: none;
      border: none;
      text-align: left;
    }

    .skill-result-item:hover,
    .skill-result-item:focus-visible {
      background: var(--muted);
      outline: none;
    }

    .skill-result-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
    }

    .skill-no-results {
      padding: 8px;
      font-size: 13px;
      color: var(--muted-foreground);
      text-align: center;
    }

    .icon-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 4px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: color .12s, background .12s;
    }

    .icon-btn:hover {
      color: var(--destructive);
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
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
      flex: 0 0 120px;
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
  @state() private _skillPickerOpen = false;
  @state() private _skillSearchDebounce: ReturnType<typeof setTimeout> | undefined;
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

      // Email required by CreateAgentRequest schema; validate format too
      if (!this._formData.email) {
        errors['email'] = 'Email is required.';
      } else {
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
    if (!this._ensureCodeEditClosed()) {
      this._currentStep = 0;
      return;
    }

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
      // Redirect back to step 0 if any step-1 fields failed validation
      const step0Fields = ['code', 'name', 'email'];
      if (step0Fields.some(f => errors[f])) {
        this._currentStep = 0;
      }
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
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'agent' },
            bubbles: true,
            composed: true,
          })
        );
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
    this._skillPickerOpen = false;
  }

  private _handleRemoveSkill(skillId: string): void {
    this._assignedSkills = this._assignedSkills.filter((s) => s.skill_id !== skillId);
  }

  private _handleProficiencyChange(skillId: string, value: number): void {
    this._assignedSkills = this._assignedSkills.map((s) =>
      s.skill_id === skillId ? { ...s, proficiency: value } : s
    );
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
      <p class="step-helper">Define the agent's identity. Code cannot be changed after create.</p>

      <div class="form-row">
        <label class="form-label form-label-required" for="agent-name">Name</label>
        <input
          id="agent-name"
          class="uk-input"
          type="text"
          required
          .value=${this._formData.name}
          placeholder="e.g. Billing Support Agent"
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
        <label class="form-label form-label-required" for="agent-email">Email</label>
        <input
          id="agent-email"
          class="uk-input"
          type="email"
          required
          .value=${this._formData.email}
          placeholder="agent@example.com"
          @input=${(e: Event) => {
            this._formData = { ...this._formData, email: (e.target as HTMLInputElement).value };
          }}
        />
        ${when(
          this._errors['email'],
          () => html`<div class="field-error">${this._errors['email']}</div>`
        )}
      </div>

      <div class="form-row">
        <label class="form-label" for="agent-ext-id">External ID</label>
        <input
          id="agent-ext-id"
          class="uk-input"
          type="text"
          .value=${this._formData.external_id}
          placeholder="Optional reference from your system"
          @input=${(e: Event) => {
            this._formData = {
              ...this._formData,
              external_id: (e.target as HTMLInputElement).value,
            };
          }}
        />
        <div class="field-help">Match an ID from your CRM, HR system, or identity provider.</div>
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
        <div class="field-help">Disabled agents cannot log in or receive interactions.</div>
      </div>

      <div class="form-actions">
        <button
          class="uk-button uk-button-default"
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
        >Cancel</button>
        <button class="uk-button uk-button-primary" @click=${this._handleNext}>Next: Skills →</button>
      </div>
    `;
  }

  private _renderSkillsStep() {
    const proficiencyOptions = Array.from({ length: 10 }, (_, i) => i + 1);

    return html`
      <p class="step-helper">Assign initial skills. You can also edit skills after creating the agent.</p>

      <div class="form-section">
        ${when(
          this._assignedSkills.length === 0,
          () => html`<p class="empty-skills">No skills assigned yet.</p>`,
          () => html`
            <table class="skills-table">
              <thead>
                <tr>
                  <th>Skill</th>
                  <th class="col-proficiency">Proficiency</th>
                  <th class="col-action"></th>
                </tr>
              </thead>
              <tbody>
                ${this._assignedSkills.map(
                  (skill) => html`
                    <tr>
                      <td><span class="skill-name-cell">${skill.name ?? skill.skill_id}</span></td>
                      <td>
                        <select
                          class="uk-select"
                          aria-label="Proficiency for ${skill.name ?? skill.skill_id}"
                          @change=${(e: Event) =>
                            this._handleProficiencyChange(
                              skill.skill_id,
                              parseInt((e.target as HTMLSelectElement).value, 10)
                            )}
                        >
                          ${proficiencyOptions.map(
                            (n) => html`<option value=${String(n)} ?selected=${n === skill.proficiency}>${n}</option>`
                          )}
                        </select>
                      </td>
                      <td style="text-align:right">
                        <button
                          class="icon-btn"
                          title="Remove skill"
                          aria-label="Remove ${skill.name ?? skill.skill_id}"
                          @click=${() => this._handleRemoveSkill(skill.skill_id)}
                        >
                          <uk-icon icon="trash-2" height="15" width="15"></uk-icon>
                        </button>
                      </td>
                    </tr>
                  `
                )}
              </tbody>
            </table>
          `
        )}

        <div class="skill-picker">
          <button
            class="uk-button uk-button-default"
            style="font-size:13px"
            @click=${() => {
              this._skillPickerOpen = !this._skillPickerOpen;
              this._skillSearchResults = [];
            }}
          >
            <uk-icon icon="plus" height="14" width="14" style="margin-right:4px"></uk-icon>
            Add skill
          </button>
          ${this._skillPickerOpen ? html`
            <div class="skill-dropdown">
              <input
                class="uk-input"
                type="search"
                placeholder="Search skills…"
                @input=${this._handleSkillSearch}
              />
              <div class="skill-results">
                ${this._skillSearchResults.length > 0
                  ? this._skillSearchResults.map(
                      (skill) => html`
                        <button
                          class="skill-result-item"
                          role="option"
                          @click=${() => this._handleAddSkill(skill)}
                          @keydown=${(e: KeyboardEvent) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault();
                              this._handleAddSkill(skill);
                            }
                          }}
                        >
                          <span>${skill.name}</span>
                          <span class="skill-result-code">${skill.code}</span>
                        </button>
                      `
                    )
                  : html`<div class="skill-no-results">Type to search skills</div>`}
              </div>
            </div>
          ` : nothing}
        </div>
      </div>

      <div class="form-actions">
        <button class="uk-button uk-button-default" @click=${this._handleBack}>← Back</button>
        <button class="uk-button uk-button-primary" @click=${this._handleNext}>Next: Review →</button>
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
          <div class="api-error">
            <uk-icon icon="x" height="16" width="16"></uk-icon>
            ${this._apiError}
          </div>
        `
      )}

      <div class="form-actions">
        <button class="uk-button uk-button-default" @click=${this._handleBack}>← Back</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? 'Creating…' : 'Create agent'}
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
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Agents
        </button>
        <h1 class="page-title">Create agent</h1>
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
          <div slot="step-skills">
            ${this._renderSkillsStep()}
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
    'or-agent-form': OrAgentForm;
  }
}
