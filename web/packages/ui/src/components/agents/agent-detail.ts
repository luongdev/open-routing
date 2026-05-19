// Phase 6 Plan 05: <or-agent-detail> — Agent entity detail/edit page.
// Task 2a: core read+edit form (code readonly, name/email/external_id/enabled, Save/Cancel).
// Task 2b: extensions — 409 conflict banner, delete/enable/disable, skills sub-table, wrapup countdown.
// ADMIN-04: only this.client.GET/PATCH/DELETE — never direct fetch().
// D04_1-02: code field is ALWAYS read-only after create.
// D6-03: 409 body comes from error.current — no re-GET.
// D6-V-40: delete confirm requires typing exact agent.name (case-sensitive).
// Pitfall 9: never call response.json() — openapi-fetch parses error body for us.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import validateUpdateAgent from '../../validators/UpdateAgentRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

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
 * <or-agent-detail> — Agent detail / edit page (Ember healthcare-dashboard style).
 *
 * Page-header + section cards layout.
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
      padding: 20px 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 16px;
      gap: 16px;
    }

    .page-header-left {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      min-width: 0;
    }

    .back-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 6px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      margin-top: 2px;
      flex-shrink: 0;
      transition: color .12s, background .12s;
    }

    .back-btn:hover {
      color: var(--foreground);
      background: var(--muted);
    }

    .page-title {
      font-size: 22px;
      font-weight: 700;
      margin: 0 0 2px;
      color: var(--foreground);
    }

    .page-subtitle {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0;
    }

    .page-header-right {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-shrink: 0;
    }

    /* ── Status badge ────────────────────────────────────────────── */
    .status-badge {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 4px 12px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 600;
      flex-shrink: 0;
    }

    .status-badge--active {
      background: color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent);
      color: oklch(0.45 0.18 145);
    }

    .status-badge--disabled {
      background: var(--muted);
      color: var(--muted-foreground);
    }

    .status-badge--wrapup {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 20%, transparent);
      color: oklch(0.5 0.18 80);
    }

    /* ── Stats row ───────────────────────────────────────────────── */
    .stats-row {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 10px;
      margin-bottom: 14px;
    }

    @media (max-width: 900px) {
      .stats-row {
        grid-template-columns: repeat(2, 1fr);
      }
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px 14px;
      box-shadow: var(--shadow-xs);
    }

    .stat-label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .04em;
      color: var(--muted-foreground);
      margin-bottom: 4px;
    }

    .stat-value {
      font-size: 18px;
      font-weight: 700;
      color: var(--foreground);
    }

    .stat-sub {
      font-size: 11px;
      color: var(--muted-foreground);
      margin-top: 2px;
    }

    /* ── Info cards ──────────────────────────────────────────────── */
    .info-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px 20px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 12px;
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 12px;
      display: flex;
      align-items: center;
      gap: 7px;
    }

    .card-title uk-icon {
      color: var(--muted-foreground);
    }

    /* ── Two-column grid for info row ────────────────────────────── */
    .two-col-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10px 20px;
    }

    @media (max-width: 640px) {
      .two-col-grid { grid-template-columns: 1fr; }
    }

    .form-group { margin-bottom: 0; }

    .field-label {
      display: block;
      font-size: 12px;
      font-weight: 600;
      color: var(--muted-foreground);
      margin-bottom: 5px;
    }

    .field-error {
      font-size: 12px;
      color: var(--destructive);
      margin-top: 4px;
    }

    /* ── Toggle row ──────────────────────────────────────────────── */
    .toggle-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 0;
    }

    .toggle-row + .toggle-row {
      border-top: 1px solid var(--border);
    }

    .toggle-label {
      font-size: 14px;
      font-weight: 500;
      color: var(--foreground);
    }

    .toggle-sub {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 1px;
    }

    /* Frankenstyle checkbox used as toggle */
    .uk-toggle {
      position: relative;
      display: inline-block;
      width: 40px;
      height: 22px;
      flex-shrink: 0;
    }

    .uk-toggle input { opacity: 0; width: 0; height: 0; }

    .uk-toggle-slider {
      position: absolute;
      inset: 0;
      background: var(--muted);
      border-radius: 9999px;
      cursor: pointer;
      transition: background .15s;
    }

    .uk-toggle-slider::before {
      content: '';
      position: absolute;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: white;
      left: 3px;
      top: 3px;
      transition: transform .15s;
      box-shadow: 0 1px 3px rgba(0,0,0,.2);
    }

    .uk-toggle input:checked + .uk-toggle-slider {
      background: var(--primary);
    }

    .uk-toggle input:checked + .uk-toggle-slider::before {
      transform: translateX(18px);
    }

    /* ── Wrapup alert ────────────────────────────────────────────── */
    .alert {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 12px;
    }

    .alert--warning {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 15%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.65 0.18 80) 40%, transparent);
      color: oklch(0.45 0.18 75);
    }

    .alert--danger {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent);
      color: var(--destructive);
    }

    /* ── Skills section ──────────────────────────────────────────── */
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

    .skill-name-code {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
      color: var(--foreground);
    }

    .skill-name-code::before {
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
      padding: 8px 0 4px;
    }

    .skill-helper {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 8px;
    }

    /* ── Skill search dropdown ───────────────────────────────────── */
    .skill-add-wrap {
      position: relative;
      display: inline-block;
      margin-top: 12px;
    }

    .skill-dropdown-popup {
      position: absolute;
      top: 100%;
      left: 0;
      margin-top: 4px;
      min-width: 280px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: var(--shadow-md);
      z-index: 1020;
      overflow: hidden;
    }

    .skill-search-input-wrap {
      padding: 8px;
      border-bottom: 1px solid var(--border);
    }

    .skill-search-results {
      max-height: 200px;
      overflow-y: auto;
    }

    .skill-result-item {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 8px 12px;
      cursor: pointer;
      font-size: 13px;
      color: var(--foreground);
      transition: background .1s;
    }

    .skill-result-item:hover { background: var(--muted); }

    .skill-result-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
    }

    .skill-empty-hint {
      padding: 12px;
      text-align: center;
      font-size: 13px;
      color: var(--muted-foreground);
    }

    /* ── Footer meta ─────────────────────────────────────────────── */
    .footer-meta {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 12px;
      padding-top: 10px;
      border-top: 1px solid var(--border);
    }

    .bottom-bar {
      display: flex;
      gap: 8px;
      margin-top: 14px;
      justify-content: flex-end;
    }

    /* ── Loading / spinner ───────────────────────────────────────── */
    .spinner {
      display: inline-block;
      width: 20px;
      height: 20px;
      border: 2px solid var(--border);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin .6s linear infinite;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    .loading-wrap {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 40px 0;
      color: var(--muted-foreground);
      font-size: 14px;
    }

    /* ── Delete confirm inline panel (D7-04 pattern) ─────────────── */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: var(--shadow-overlay, oklch(0 0 0 / 0.5));
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }

    .confirm-panel {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 24px;
      max-width: 480px;
      width: 90%;
      box-shadow: var(--shadow-lg);
    }

    .confirm-title {
      margin: 0 0 10px;
      font-size: 18px;
      font-weight: 600;
      color: var(--foreground);
    }

    .confirm-body {
      margin: 0 0 16px;
      font-size: 14px;
      color: var(--muted-foreground);
    }

    .action-row {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }

    /* ── Toast ───────────────────────────────────────────────────── */
    .toast-container {
      position: fixed;
      bottom: 24px;
      right: 24px;
      z-index: var(--or-z-toast, 9000);
    }

    .toast {
      display: flex;
      align-items: center;
      gap: 8px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px 16px;
      box-shadow: var(--shadow-md);
      font-size: 14px;
      color: var(--foreground);
    }

    .toast uk-icon { color: oklch(0.45 0.18 145); }

    /* ── Icon button ─────────────────────────────────────────────── */
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
      color: var(--foreground);
      background: var(--muted);
    }

    .icon-btn--danger:hover {
      color: var(--destructive);
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
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
  @state() private _skillDropdownOpen = false;
  @state() private _skillSearchDebounce: ReturnType<typeof setTimeout> | undefined;

  private _wrapupInterval: ReturnType<typeof setInterval> | null = null;
  private _form: Form = { name: '', email: '', external_id: '', enabled: true };
  private _savedToastTimeout?: ReturnType<typeof setTimeout>;

  // --- Computed ---
  get _canDelete(): boolean {
    return this._deleteConfirmName === this._entity?.name;
  }

  // --- Lifecycle ---
  private _onKeydown?: (e: KeyboardEvent) => void;
  private _onDocumentClick?: (e: MouseEvent) => void;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadEntity();
    this._startWrapupCountdown();
    this._onKeydown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (this._deleteConfirmOpen) {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }
        if (this._skillDropdownOpen) {
          this._skillDropdownOpen = false;
          this._skillSearchResults = [];
        }
      }
    };
    this._onDocumentClick = (e: MouseEvent) => {
      const path = e.composedPath();
      const inSkillWrap = path.some(
        (n) => n instanceof Element && n.classList.contains('skill-add-wrap')
      );
      if (!inSkillWrap) {
        this._skillDropdownOpen = false;
      }
    };
    document.addEventListener('keydown', this._onKeydown);
    document.addEventListener('click', this._onDocumentClick);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
    if (this._onKeydown) document.removeEventListener('keydown', this._onKeydown);
    if (this._onDocumentClick) document.removeEventListener('click', this._onDocumentClick);
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('wrapupUntil')) {
      this._startWrapupCountdown();
    }
    if (changed.has('_deleteConfirmOpen')) {
      if (this._deleteConfirmOpen) {
        this.setAttribute('aria-live', 'polite');
        void this.updateComplete.then(() => {
          const input = this.shadowRoot?.querySelector('.confirm-panel input') as HTMLElement | null;
          if (input) input.focus();
        });
      } else {
        this.removeAttribute('aria-live');
      }
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
    this._conflictServer = null;
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

  private _relativeTime(iso: string): string {
    try {
      const ms = Date.now() - new Date(iso).getTime();
      const min = Math.floor(ms / 60000);
      if (min < 1) return 'just now';
      if (min < 60) return `${min}m ago`;
      const h = Math.floor(min / 60);
      if (h < 24) return `${h}h ago`;
      return `${Math.floor(h / 24)}d ago`;
    } catch { return iso; }
  }

  // --- Save / PATCH ---

  async _handleSave(): Promise<void> {
    if (!this._entity || this._saving) return;

    const body = {
      name: this._form.name,
      email: this._form.email,
      // Per UpdateAgentRequest contract (D04_1-07): pass "" to clear binding; null === omission.
      // We always include external_id so user clearing the field sends "" (not null/omission).
      external_id: this._form.external_id,
      enabled: this._form.enabled,
      version: this._entity.version,
      skills: this._assignedSkills.map((s) => ({
        skill_id: s.skill_id,
        proficiency: s.proficiency,
      })),
    };

    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateUpdateAgent as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(body)) {
      const errors: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
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
        // Cross-AI fix: also update _entity.version so a re-submit uses the server-current
        // version instead of looping into another 409 (D6-03 + Codex review HIGH).
        if (error && typeof error === 'object' && 'current' in error) {
          const current = (error as { current: Agent }).current;
          this._conflictServer = current as unknown as Record<string, unknown>;
          this._entity = current;
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
            this._apiError = errObj.reason ?? 'Invalid value';
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
    this._skillDropdownOpen = false;
    this._markDirty();
  }

  // --- Conflict banner handlers ---

  private _handleConflictAcknowledged(e: CustomEvent): void {
    if (e.detail?.action === 'discard') {
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

  private _renderPageHeader() {
    const entity = this._entity;
    const statusBadge = entity
      ? this._wrapupSecondsLeft && this._wrapupSecondsLeft > 0
        ? html`<span class="status-badge status-badge--wrapup">
            <uk-icon icon="clock" width="12" height="12"></uk-icon>
            Wrapup ${this._formatCountdown(this._wrapupSecondsLeft)}
          </span>`
        : entity.enabled
        ? html`<span class="status-badge status-badge--active">Active</span>`
        : html`<span class="status-badge status-badge--disabled">Disabled</span>`
      : nothing;

    return html`
      <div class="page-header">
        <div class="page-header-left">
          <button
            class="back-btn"
            title="Back to Agents"
            @click=${() => this._navigate(`/orgs/${this.orgId}/agents`)}
          >
            <uk-icon icon="arrow-left" width="18" height="18"></uk-icon>
          </button>
          <div>
            <h1 class="page-title">${entity?.name ?? 'Agent Detail'}</h1>
            ${entity ? html`<p class="page-subtitle">${entity.code}</p>` : nothing}
          </div>
        </div>

        <div class="page-header-right">
          ${statusBadge}

          ${entity
            ? html`
                <button
                  class="uk-button uk-button-default uk-button-small"
                  @click=${() => this._navigate(`/orgs/${this.orgId}/agents/${this.entityId}/status`)}
                >
                  <uk-icon icon="route" width="14" height="14"></uk-icon>
                  Status
                </button>

                ${entity.enabled
                  ? html`<button class="uk-button uk-button-default uk-button-small" @click=${this._handleDisable}>Disable</button>`
                  : html`<button class="uk-button uk-button-default uk-button-small" @click=${this._handleEnable}>Enable</button>`}

                <button
                  class="uk-button uk-button-danger uk-button-small"
                  @click=${() => {
                    this._deleteConfirmOpen = true;
                    this._deleteConfirmName = '';
                  }}
                >
                  <uk-icon icon="trash-2" width="14" height="14"></uk-icon>
                  Delete
                </button>
              `
            : nothing}
        </div>
      </div>
    `;
  }

  private _renderStatsRow() {
    const entity = this._entity;
    if (!entity) return nothing;

    return html`
      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-label">Skills</div>
          <div class="stat-value">${this._assignedSkills.length}</div>
          <div class="stat-sub">assigned</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Status</div>
          <div class="stat-value" style="font-size:14px;padding-top:4px">${entity.enabled ? 'Active' : 'Disabled'}</div>
          <div class="stat-sub">current</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Version</div>
          <div class="stat-value">${entity.version}</div>
          <div class="stat-sub">lock rev</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Last Updated</div>
          <div class="stat-value" style="font-size:14px;padding-top:4px">${this._relativeTime(entity.updated_at ?? '')}</div>
          <div class="stat-sub">${entity.updated_at ? new Date(entity.updated_at).toLocaleDateString() : ''}</div>
        </div>
      </div>
    `;
  }

  private _renderWrapupBanner() {
    if (!this._wrapupSecondsLeft || this._wrapupSecondsLeft <= 0) return nothing;
    return html`
      <div class="alert alert--warning">
        <uk-icon icon="clock" width="16" height="16"></uk-icon>
        Agent in wrapup — ${this._formatCountdown(this._wrapupSecondsLeft)} remaining
      </div>
    `;
  }

  private _renderConflictBanner() {
    if (!this._conflictServer) return nothing;
    return html`
      <or-conflict-banner
        mode="crud"
        .serverValue=${this._conflictServer}
        .userValue=${this._form as Record<string, unknown>}
        @open-routing:conflict-acknowledged=${this._handleConflictAcknowledged}
      ></or-conflict-banner>
    `;
  }

  private _renderIdentityCard() {
    const entity = this._entity;
    if (!entity) return nothing;

    return html`
      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="users" width="15" height="15"></uk-icon>
          Identity
        </h2>

        ${this._apiError
          ? html`<div class="alert alert--danger" style="margin-bottom:16px">
              <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
              ${this._apiError}
            </div>`
          : nothing}

        <div class="form-group" style="margin-bottom:16px">
          <or-code-input
            .value=${entity.code}
            .readonly=${true}
          ></or-code-input>
        </div>

        <div class="two-col-grid">
          <div class="form-group">
            <label class="field-label" for="agent-name">Name</label>
            <input
              id="agent-name"
              class="uk-input"
              type="text"
              .value=${this._form.name}
              required
              @input=${(e: Event) => {
                this._form = { ...this._form, name: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['name']
              ? html`<div class="field-error">${this._fieldErrors['name']}</div>`
              : nothing}
          </div>

          <div class="form-group">
            <label class="field-label" for="agent-email">Email</label>
            <input
              id="agent-email"
              class="uk-input"
              type="email"
              .value=${this._form.email}
              @input=${(e: Event) => {
                this._form = { ...this._form, email: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
            ${this._fieldErrors['email']
              ? html`<div class="field-error">${this._fieldErrors['email']}</div>`
              : nothing}
          </div>

          <div class="form-group">
            <label class="field-label" for="agent-ext-id">External ID</label>
            <input
              id="agent-ext-id"
              class="uk-input"
              type="text"
              .value=${this._form.external_id}
              @input=${(e: Event) => {
                this._form = { ...this._form, external_id: (e.target as HTMLInputElement).value };
                this._markDirty();
              }}
            />
          </div>
        </div>

        <div class="footer-meta">
          version ${entity.version}
          · updated ${this._relativeTime(entity.updated_at ?? '')}
          · created ${entity.created_at ? new Date(entity.created_at).toLocaleDateString() : ''}
        </div>
      </div>
    `;
  }

  private _renderStatusCard() {
    if (!this._entity) return nothing;

    return html`
      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="plug" width="15" height="15"></uk-icon>
          Status &amp; Routing
        </h2>

        ${this._renderWrapupBanner()}

        <div class="toggle-row">
          <div>
            <div class="toggle-label">Enabled</div>
            <div class="toggle-sub">Agent receives new interactions when enabled</div>
          </div>
          <label class="uk-toggle">
            <input
              type="checkbox"
              ?checked=${this._form.enabled}
              @change=${(e: Event) => {
                this._form = { ...this._form, enabled: (e.target as HTMLInputElement).checked };
                this._markDirty();
              }}
            />
            <span class="uk-toggle-slider"></span>
          </label>
        </div>
      </div>
    `;
  }

  private _renderSkillsCard() {
    const proficiencyOptions = Array.from({ length: 10 }, (_, i) => i + 1);

    return html`
      <div class="info-card">
        <h2 class="card-title">
          <uk-icon icon="tag" width="15" height="15"></uk-icon>
          Skills
        </h2>

        ${when(
          this._assignedSkills.length === 0,
          () => html`<p class="empty-skills">No skills assigned.</p>`,
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
                      <td><span class="skill-name-code">${skill.name ?? skill.skill_id}</span></td>
                      <td>
                        <select
                          class="uk-select"
                          .value=${String(skill.proficiency)}
                          @change=${(e: Event) =>
                            this._handleProficiencyChange(
                              skill.skill_id,
                              parseInt((e.target as HTMLSelectElement).value, 10)
                            )}
                          aria-label="Proficiency for ${skill.name}"
                        >
                          ${proficiencyOptions.map(
                            (n) => html`<option value=${String(n)} ?selected=${n === skill.proficiency}>${n}</option>`
                          )}
                        </select>
                      </td>
                      <td style="text-align:right">
                        <button
                          class="icon-btn icon-btn--danger"
                          title="Remove skill"
                          aria-label="Remove ${skill.name ?? skill.skill_id}"
                          @click=${() => this._handleRemoveSkill(skill.skill_id)}
                        >
                          <uk-icon icon="trash-2" width="15" height="15"></uk-icon>
                        </button>
                      </td>
                    </tr>
                  `
                )}
              </tbody>
            </table>
          `
        )}

        <div class="skill-add-wrap">
          <button
            class="uk-button uk-button-default uk-button-small"
            @click=${(e: Event) => {
              e.stopPropagation();
              this._skillDropdownOpen = !this._skillDropdownOpen;
              if (!this._skillDropdownOpen) this._skillSearchResults = [];
            }}
          >
            <uk-icon icon="plus" width="13" height="13"></uk-icon>
            Add skill
          </button>

          ${this._skillDropdownOpen
            ? html`
                <div class="skill-dropdown-popup">
                  <div class="skill-search-input-wrap">
                    <input
                      class="uk-input"
                      type="text"
                      placeholder="Search skills…"
                      @click=${(e: Event) => e.stopPropagation()}
                      @input=${this._handleSkillSearch}
                    />
                  </div>
                  <div class="skill-search-results">
                    ${this._skillSearchResults.length > 0
                      ? this._skillSearchResults.map(
                          (skill) => html`
                            <div
                              class="skill-result-item"
                              @click=${() => this._handleAddSkill(skill)}
                            >
                              <span>${skill.name}</span>
                              <span class="skill-result-code">${skill.code}</span>
                            </div>
                          `
                        )
                      : html`<div class="skill-empty-hint">Type to search skills</div>`}
                  </div>
                </div>
              `
            : nothing}
        </div>

        <p class="skill-helper">Tip: skill changes save with the form.</p>
      </div>
    `;
  }

  private _renderFormActions() {
    if (!this._entity) return nothing;
    const entity = this._entity;

    return html`
      <div class="bottom-bar">
        <button
          class="uk-button uk-button-default"
          @click=${() => {
            if (!this._dirty) {
              this._navigate(`/orgs/${this.orgId}/agents`);
            } else {
              this._form = {
                name: entity.name ?? '',
                email: entity.email ?? '',
                external_id: entity.external_id ?? '',
                enabled: entity.enabled ?? true,
              };
              this._dirty = false;
            }
          }}
        >Cancel</button>
        <button
          class="uk-button uk-button-primary"
          ?disabled=${!this._dirty || this._saving}
          @click=${this._handleSave}
        >
          ${this._saving
            ? html`<span class="spinner" style="width:14px;height:14px;border-width:2px;margin-right:6px"></span> Saving…`
            : 'Save changes'}
        </button>
      </div>
    `;
  }

  private _renderDeleteDialog() {
    return when(this._deleteConfirmOpen, () => html`
      <div
        class="confirm-overlay"
        role="presentation"
        @click=${() => {
          this._deleteConfirmOpen = false;
          this._deleteConfirmName = '';
        }}
      >
        <div
          class="confirm-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="confirm-title"
          @click=${(e: Event) => e.stopPropagation()}
        >
          <h3 id="confirm-title" class="confirm-title">Delete agent "${this._entity?.name ?? ''}"?</h3>
          <p class="confirm-body">This is permanent and cannot be undone. Type the agent name to confirm.</p>
          <div style="margin-bottom: 20px;">
            <label class="field-label" for="delete-confirm-input">Agent name</label>
            <input
              id="delete-confirm-input"
              class="uk-input"
              type="text"
              placeholder="Type agent name to confirm"
              .value=${this._deleteConfirmName}
              aria-label="Type agent name to confirm deletion"
              @input=${(e: Event) => {
                this._deleteConfirmName = (e.target as HTMLInputElement).value;
              }}
            />
          </div>
          <div class="action-row">
            <button
              class="uk-button uk-button-default uk-button-small"
              @click=${() => {
                this._deleteConfirmOpen = false;
                this._deleteConfirmName = '';
              }}
            >Cancel</button>
            <button
              class="uk-button uk-button-danger uk-button-small"
              ?disabled=${!this._canDelete}
              @click=${this._handleDelete}
            >Delete</button>
          </div>
        </div>
      </div>
    `);
  }

  override render() {
    if (this._loading) {
      return html`
        <div class="loading-wrap">
          <span class="spinner"></span>
          Loading agent…
        </div>
      `;
    }

    if (!this._entity && this._apiError) {
      return html`
        <div class="alert alert--danger" style="margin:24px 0">
          <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
          ${this._apiError}
        </div>
      `;
    }

    return html`
      ${this._renderPageHeader()}
      ${this._renderStatsRow()}
      ${this._renderConflictBanner()}
      ${this._renderIdentityCard()}
      ${this._renderStatusCard()}
      ${this._renderSkillsCard()}
      ${this._renderFormActions()}
      ${this._renderDeleteDialog()}

      ${when(
        this._showSavedToast,
        () => html`
          <div class="toast-container">
            <div class="toast">
              <uk-icon icon="check" width="16" height="16"></uk-icon>
              Saved
            </div>
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
