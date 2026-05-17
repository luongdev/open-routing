// Phase 6 Plan 05: <or-agent-detail> — Agent entity detail/edit page.
// Task 2a: core read+edit form (code readonly, name/email/external_id/enabled, Save/Cancel).
// Task 2b: extensions — 409 conflict banner, delete/enable/disable, skills sub-table, wrapup countdown.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET.
// D6-V-40: delete confirm requires typing exact agent.name (case-sensitive).
// Pitfall 9: never call response.json() — openapi-fetch parses error body for us.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateAgent from '../../validators/UpdateAgentRequest.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/switch/switch.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/dialog/dialog.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/badge/badge.js';
import '@shoelace-style/shoelace/dist/components/dropdown/dropdown.js';
import '@shoelace-style/shoelace/dist/components/menu/menu.js';
import '@shoelace-style/shoelace/dist/components/menu-item/menu-item.js';

// Primitives
import '../primitives/code-input.js';
import '../primitives/conflict-banner.js';

type Agent = components['schemas']['Agent'];

interface SkillRow {
  id?: string;
  skill_id: string;
  name?: string;
  proficiency: number;
}

type Form = {
  name: string;
  email: string;
  external_id: string;
  enabled: boolean;
};

/**
 * <or-agent-detail> — Agent detail / edit page.
 *
 * 2-column layout at ≥1280px: 640px form (left) + flex-1 skills sub-table (right).
 * On mobile/tablet: stacks vertically.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - entityId: (attribute 'entity-id') — the agent UUID
 *   - client: ApiClient — passed from shell at boot
 *   - wrapupUntil: (attribute 'wrapup-until') — ISO datetime string or null
 */
@customElement('or-agent-detail')
export class OrAgentDetail extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
    }

    .top-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 24px;
      flex-wrap: wrap;
    }

    .top-bar-spacer { flex: 1; }

    .page-title {
      font-size: var(--or-text-display, 24px);
      font-weight: 700;
      color: var(--or-color-text-strong, #171717);
      margin: 0 0 4px;
    }

    .two-col {
      display: grid;
      grid-template-columns: 640px 1fr;
      gap: 32px;
    }

    @media (max-width: 1279px) {
      .two-col {
        grid-template-columns: 1fr;
      }
    }

    .form-col {
      min-width: 0;
    }

    .skills-col {
      min-width: 0;
    }

    .form-group {
      margin-bottom: 16px;
    }

    .footer-meta {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 16px;
      padding-top: 12px;
      border-top: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .bottom-bar {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      justify-content: flex-end;
    }

    .wrapup-alert {
      margin-bottom: 16px;
    }

    .section-heading {
      font-size: 16px;
      font-weight: 600;
      color: var(--or-color-text-strong, #171717);
      margin: 0 0 12px;
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

    .skill-helper {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 8px;
    }

    .empty-skills {
      color: var(--or-color-text-muted, #737373);
      font-size: 14px;
      padding: 16px 0;
    }

    .add-skill-row {
      margin-top: 8px;
    }

    .field-error {
      font-size: 12px;
      color: var(--sl-color-danger-500, #d92d20);
      margin-top: 4px;
    }

    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: var(--or-z-toast, 9000);
    }

    .delete-btn-danger {
      color: var(--sl-color-danger-500, #d92d20);
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'entity-id' }) entityId = '';
  @property({ type: Object }) client!: ApiClient;
  @property({ type: String, attribute: 'wrapup-until' }) wrapupUntil: string | null = null;

  // --- Internal state ---
  @state() private _entity: Agent | null = null;
  @state() private _loading = false;
  @state() private _saving = false;
  @state() private _dirty = false;
  @state() private _conflictServer: Record<string, unknown> | null = null;
  @state() private _fieldErrors: Record<string, string> = {};
  @state() private _apiError: string | null = null;
  @state() private _showSavedToast = false;
  @state() private _deleteConfirmOpen = false;
  @state() private _deleteConfirmName = '';
  @state() private _wrapupSecondsLeft: number | null = null;
  @state() private _assignedSkills: SkillRow[] = [];
  @state() private _skillSearchResults: Array<{ id: string; name: string; code: string }> = [];
  @state() private _skillSearchDebounce?: ReturnType<typeof setTimeout>;

  private _wrapupInterval: ReturnType<typeof setInterval> | null = null;
  private _form: Form = { name: '', email: '', external_id: '', enabled: true };
  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

  // --- Computed ---
  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  // --- Lifecycle ---
  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadEntity();
    this._startWrapupCountdown();
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('wrapupUntil')) {
      this._startWrapupCountdown();
    }
  }

  // --- Private methods ---

  private _startWrapupCountdown(): void {
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
    if (!this.wrapupUntil) {
      this._wrapupSecondsLeft = null;
      return;
    }
    const update = () => {
      const secondsLeft = Math.max(
        0,
        Math.floor((new Date(this.wrapupUntil!).getTime() - Date.now()) / 1000)
      );
      this._wrapupSecondsLeft = secondsLeft;
      if (secondsLeft === 0 && this._wrapupInterval !== null) {
        clearInterval(this._wrapupInterval);
        this._wrapupInterval = null;
      }
    };
    update();
    this._wrapupInterval = setInterval(update, 1000);
  }

  private async _loadEntity(): Promise<void> {
    if (!this.orgId || !this.entityId) return;
    this._loading = true;
    try {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/agents/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
      } as never);
      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load agent';
        return;
      }
      const agent = data as Agent;
      this._entity = agent;
      this._form = {
        name: agent.name ?? '',
        email: agent.email ?? '',
        external_id: agent.external_id ?? '',
        enabled: agent.enabled ?? true,
      };
      this._assignedSkills = (agent.skills ?? []).map((s) => ({
        id: (s as { id?: string })?.id,
        skill_id: (s as { skill_id: string }).skill_id,
        name: (s as { name?: string })?.name,
        proficiency: (s as { proficiency: number }).proficiency,
      }));
      this._dirty = false;
    } finally {
      this._loading = false;
    }
  }

  private _markDirty(): void {
    this._dirty = true;
    this._conflictServer = null; // clear conflict banner on new edit
  }

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _showSaved(): void {
    this._showSavedToast = true;
    clearTimeout(this._savedToastTimeout);
    this._savedToastTimeout = setTimeout(() => {
      this._showSavedToast = false;
    }, 3000);
  }

  private _formatCountdown(seconds: number): string {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  }

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    const body = {
      name: this._form.name,
      email: this._form.email,
      external_id: this._form.external_id || null,
      enabled: this._form.enabled,
      version: this._entity.version,
      skills: this._assignedSkills.map((s) => ({
        skill_id: s.skill_id,
        proficiency: s.proficiency,
      })),
    };

    // Client-side validation
    if (!validateUpdateAgent(body)) {
      const errors: Record<string, string> = {};
      for (const err of validateUpdateAgent.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errors[field] = err.message ?? 'Invalid value';
      }
      this._fieldErrors = errors;
      return;
    }
    this._fieldErrors = {};

    this._saving = true;
    try {
      const result = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}' as never, {
        params: { path: { org_id: this.orgId, id: this.entityId } },
        body,
      } as never);

      const { data, error } = result as { data: Agent | null; error: unknown };

      if (error) {
        // 409 version_conflict — consume from error.current (Pitfall 9: never call response.json())
        if (error && typeof error === 'object' && 'current' in error) {
          this._conflictServer = (error as { current: Record<string, unknown> }).current;
          return;
        }
        // 422 immutable_field
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'immutable_field'
        ) {
          this._apiError = 'Code cannot be changed after create.';
          return;
        }
        // 422 invalid_value
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'invalid_value'
        ) {
          const errObj = error as { field?: string; reason?: string };
          if (errObj.field) {
            this._fieldErrors = { [errObj.field]: errObj.reason ?? 'Invalid value' };
          } else {
            this._apiError = (errObj.reason ?? 'Invalid value');
          }
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Save failed';
        return;
      }

      if (data) {
        this._entity = data;
        this._form = {
          name: data.name ?? '',
          email: data.email ?? '',
          external_id: data.external_id ?? '',
          enabled: data.enabled ?? true,
        };
        this._assignedSkills = (data.skills ?? []).map((s) => ({
          id: (s as { id?: string })?.id,
          skill_id: (s as { skill_id: string }).skill_id,
          name: (s as { name?: string })?.name,
          proficiency: (s as { proficiency: number }).proficiency,
        }));
        this._dirty = false;
        this._conflictServer = null;
        this._showSaved();
      }
    } finally {
      this._saving = false;
    }
  }

  // --- Enable / Disable ---

  async _handleEnable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: true, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Agent | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: true };
    }
  }

  async _handleDisable(): Promise<void> {
    if (!this._entity) return;
    const body = { enabled: false, version: this._entity.version };
    const result = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
      body,
    } as never);
    const { data, error } = result as { data: Agent | null; error: unknown };
    if (!error && data) {
      this._entity = data;
      this._form = { ...this._form, enabled: false };
    }
  }

  // --- Delete ---

  private async _handleDelete(): Promise<void> {
    if (!this._canDelete || !this._entity) return;
    const result = await this.client.DELETE('/v1/orgs/{org_id}/agents/{id}' as never, {
      params: { path: { org_id: this.orgId, id: this.entityId } },
    } as never);
    const { error } = result as { error: unknown };
    if (!error) {
      this._navigate(`/orgs/${this.orgId}/agents`);
    }
  }

  // --- Skills ---

  private _handleRemoveSkill(skillId: string): void {
    this._assignedSkills = this._assignedSkills.filter((s) => s.skill_id !== skillId);
    this._markDirty();
  }

  private _handleProficiencyChange(skillId: string, value: number): void {
    this._assignedSkills = this._assignedSkills.map((s) =>
      s.skill_id === skillId ? { ...s, proficiency: value } : s
    );
    this._markDirty();
  }

  private _handleSkillSearch(e: Event): void {
    const query = (e.target as HTMLInputElement).value;
    clearTimeout(this._skillSearchDebounce);
    this._skillSearchDebounce = setTimeout(async () => {
      if (!query) {
        this._skillSearchResults = [];
        return;
      }
      const { data } = await this.client.GET('/v1/orgs/{org_id}/skills' as never, {
        params: { path: { org_id: this.orgId }, query: { name: query, limit: 100 } },
      } as never);
      this._skillSearchResults = ((data as { items?: unknown[] })?.items ?? []) as Array<{
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
    this._markDirty();
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard') {
      // Revert to last known entity state
      if (this._entity) {
        this._form = {
          name: this._entity.name ?? '',
          email: this._entity.email ?? '',
          external_id: this._entity.external_id ?? '',
          enabled: this._entity.enabled ?? true,
        };
        this._dirty = false;
      }
    }
    // On 'review': keep user's form as-is for re-submission
    this._conflictServer = null;
  }

  // --- Render helpers ---

  private _renderWrapupBanner() {
    if (!this._wrapupSecondsLeft || this._wrapupSecondsLeft <= 0) return null;
    return html`
      <sl-alert class="wrapup-alert" variant="warning" open>
        <sl-icon slot="icon" name="clock"></sl-icon>
        Agent in wrapup — ${this._formatCountdown(this._wrapupSecondsLeft)} remaining
      </sl-alert>
    `;
  }

  private _renderConflictBanner() {
    if (!this._conflictServer) return null;
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._form as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
    `;
  }

  private _renderSkillsColumn() {
    const proficiencyOptions = Array.from({ length: 10 }, (_, i) => i + 1);

    return html`
      <div class="skills-col">
        <h2 class="section-heading">Skills</h2>

        ${when(
          this._assignedSkills.length === 0,
          () => html`<p class="empty-skills">No skills assigned.</p>`,
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
                          aria-label="Proficiency for ${skill.name}"
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

        <div class="add-skill-row">
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

        <p class="skill-helper">Tip: skill changes save with the form.</p>
      </div>
    `;
  }

  private _renderDeleteDialog() {
    return html`
      <sl-dialog
        label="Delete agent ${this._entity?.name ?? ''}?"
        ?open=${this._deleteConfirmOpen}
        @sl-request-close=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}
      >
        <p>This is permanent and cannot be undone.</p>
        <sl-input
          placeholder="Type agent name to confirm"
          value=${this._deleteConfirmName}
          @sl-input=${(e: Event) => {
            this._deleteConfirmName = (e.target as HTMLInputElement).value;
          }}
          aria-label="Type agent name to confirm deletion"
        ></sl-input>
        <div slot="footer" style="display:flex;gap:8px;justify-content:flex-end">
          <sl-button
            variant="default"
            @click=${() => {
              this._deleteConfirmOpen = false;
              this._deleteConfirmName = '';
            }}
          >Cancel</sl-button>
          <sl-button
            variant="danger"
            ?disabled=${!this._canDelete}
            @click=${this._handleDelete}
          >Delete</sl-button>
        </div>
      </sl-dialog>
    `;
  }

  private _renderTopBar() {
    return html`
      <div class="top-bar">
        <sl-button
          variant="text"
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
        >
          <sl-icon slot="prefix" name="arrow-left"></sl-icon>
          Back to Agents
        </sl-button>
        <div class="top-bar-spacer"></div>

        ${when(
          this._entity,
          () => html`
            ${when(
              this._entity!.enabled,
              () => html`
                <sl-button
                  variant="default"
                  size="small"
                  @click=${this._handleDisable}
                >Disable</sl-button>
              `,
              () => html`
                <sl-button
                  variant="default"
                  size="small"
                  @click=${this._handleEnable}
                >Enable</sl-button>
              `
            )}
            <sl-button
              variant="default"
              size="small"
              class="delete-btn-danger"
              style="color:var(--sl-color-danger-500)"
              @click=${() => {
                this._deleteConfirmOpen = true;
                this._deleteConfirmName = '';
              }}
            >Delete</sl-button>
          `
        )}
      </div>
    `;
  }

  private _renderForm() {
    if (!this._entity) return null;
    const entity = this._entity;
    const relativeTime = (iso: string) => {
      try {
        const ms = Date.now() - new Date(iso).getTime();
        const min = Math.floor(ms / 60000);
        if (min < 60) return `${min}m ago`;
        const h = Math.floor(min / 60);
        if (h < 24) return `${h}h ago`;
        return `${Math.floor(h / 24)}d ago`;
      } catch { return iso; }
    };

    return html`
      <div class="form-col">
        <h1 class="page-title">${entity.name}</h1>

        ${this._renderWrapupBanner()}
        ${this._renderConflictBanner()}

        ${when(
          this._apiError,
          () => html`
            <sl-alert variant="danger" open style="margin-bottom:16px">
              <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
              ${this._apiError}
            </sl-alert>
          `
        )}

        <div class="form-group">
          <or-code-input
            .value=${entity.code}
            .readonly=${true}
          ></or-code-input>
        </div>

        <div class="form-group">
          <sl-input
            label="Name"
            value=${this._form.name}
            required
            ?invalid=${!!this._fieldErrors['name']}
            @sl-input=${(e: Event) => {
              this._form = { ...this._form, name: (e.target as HTMLInputElement).value };
              this._markDirty();
            }}
          ></sl-input>
          ${when(
            this._fieldErrors['name'],
            () => html`<div class="field-error">${this._fieldErrors['name']}</div>`
          )}
        </div>

        <div class="form-group">
          <sl-input
            label="Email"
            type="email"
            value=${this._form.email}
            @sl-input=${(e: Event) => {
              this._form = { ...this._form, email: (e.target as HTMLInputElement).value };
              this._markDirty();
            }}
          ></sl-input>
          ${when(
            this._fieldErrors['email'],
            () => html`<div class="field-error">${this._fieldErrors['email']}</div>`
          )}
        </div>

        <div class="form-group">
          <sl-input
            label="External ID"
            value=${this._form.external_id}
            @sl-input=${(e: Event) => {
              this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
              this._markDirty();
            }}
          ></sl-input>
        </div>

        <div class="form-group">
          <sl-switch
            ?checked=${this._form.enabled}
            @sl-change=${(e: Event) => {
              this._form = { ...this._form, enabled: (e.target as HTMLInputElement).checked };
              this._markDirty();
            }}
          >Enabled</sl-switch>
        </div>

        <div class="footer-meta">
          version ${entity.version}
          · updated ${relativeTime(entity.updated_at ?? '')}
          · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
        </div>

        <div class="bottom-bar">
          <sl-button
            variant="default"
            @click=${() => {
              if (!this._dirty) {
                this._navigate(`/orgs/${this.orgId}/agents`);
              } else {
                // Reset form to entity state (simple cancel — could show dialog)
                this._form = {
                  name: entity.name ?? '',
                  email: entity.email ?? '',
                  external_id: entity.external_id ?? '',
                  enabled: entity.enabled ?? true,
                };
                this._dirty = false;
              }
            }}
          >Cancel</sl-button>
          <sl-button
            variant="primary"
            ?disabled=${!this._dirty || this._saving}
            @click=${this._handleSave}
          >
            ${this._saving ? html`<sl-spinner></sl-spinner> Saving…` : 'Save changes'}
          </sl-button>
        </div>
      </div>
    `;
  }

  override render() {
    if (this._loading) {
      return html`<sl-spinner></sl-spinner>`;
    }

    if (!this._entity && this._apiError) {
      return html`
        <sl-alert variant="danger" open>
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          ${this._apiError}
        </sl-alert>
      `;
    }

    return html`
      ${this._renderTopBar()}

      <div class="two-col">
        ${this._renderForm()}
        ${this._renderSkillsColumn()}
      </div>

      ${this._renderDeleteDialog()}

      ${when(
        this._showSavedToast,
        () => html`
          <div class="toast-container">
            <sl-alert variant="success" open>
              <sl-icon slot="icon" name="check-circle"></sl-icon>
              Saved
            </sl-alert>
          </div>
        `
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-agent-detail': OrAgentDetail;
  }
}
