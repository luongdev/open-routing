// Phase 6 Plan 12: <or-status-panel> — agent status display with polling,
// state machine transition buttons, break reason picker, post-interaction state,
// wrapup countdown, and force-flag admin recovery affordance.
//
// D6-26: 5s polling when visible; pause on document.hidden; immediate GET + state_version
//   compare on resume (STATE-08).
// D6-27: No client-side cache — each poll = fresh GET.
// D6-03: 409 invalid_transition consumed from error.from/.to — no re-GET (D6-04 status variant).
// D-84: force=true admin override; hidden in Advanced disclosure; warning color.
// W0.1-20: Ember dashboard layout — adoptShadowSheets, zero sl-* tags.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import '../primitives/conflict-banner.js';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AgentStatus = 'Ready' | 'NotReady' | 'Break' | 'Engaged' | 'WrapUp' | 'Offline';

export interface AgentStatusResponse {
  agent_id: string;
  status: AgentStatus;
  state_version: number;
  break_reason_id?: string | null;
  break_reason_name?: string | null;
  post_interaction_state?: 'ready' | 'not_ready' | null;
  wrapup_until?: string | null;
  updated_at: string;
}

interface BreakReason {
  id: string;
  code: string;
  name: string;
  routable: boolean;
  display_order?: number;
}

// ---------------------------------------------------------------------------
// Status pill color map — oklch tokens, no sl-* references
// ---------------------------------------------------------------------------

// icon = Bootstrap icon name (used by agent-status-list sl-icon).
// lucideIcon = Lucide icon name (used by status-panel uk-icon).
export const STATUS_PILL_STYLES: Record<AgentStatus, { bg: string; text: string; icon: string; lucideIcon: string }> = {
  Ready: {
    bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent)',
    text: 'oklch(0.45 0.18 145)',
    icon: 'circle-fill',
    lucideIcon: 'circle-check',
  },
  NotReady: {
    bg: 'color-mix(in oklch, var(--destructive) 12%, transparent)',
    text: 'var(--destructive)',
    icon: 'dash-circle-fill',
    lucideIcon: 'circle-x',
  },
  Break: {
    bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 18%, transparent)',
    text: 'oklch(0.5 0.18 80)',
    icon: 'pause-circle-fill',
    lucideIcon: 'pause-circle',
  },
  Engaged: {
    bg: 'color-mix(in oklch, oklch(0.55 0.2 250) 14%, transparent)',
    text: 'oklch(0.4 0.2 250)',
    icon: 'telephone-fill',
    lucideIcon: 'phone',
  },
  WrapUp: {
    bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 12%, transparent)',
    text: 'oklch(0.5 0.18 80)',
    icon: 'hourglass-split',
    lucideIcon: 'hourglass',
  },
  Offline: {
    bg: 'var(--muted)',
    text: 'var(--muted-foreground)',
    icon: 'power',
    lucideIcon: 'power',
  },
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * <or-status-panel> — agent status operational view (Ember dashboard style).
 *
 * Attributes:
 *   org-id   — current org UUID (from URL path)
 *   agent-id — target agent UUID
 *
 * Properties:
 *   client   — ApiClient instance from shell
 *   embedded — when true, hides "Back" button (embedded in agent-detail)
 *
 * Polling (D6-26):
 *   - Polls GET /v1/orgs/{orgId}/agents/{agentId}/status every 5 seconds.
 *   - Pauses when document.hidden === true (visibilitychange listener).
 *   - On resume: fires immediate GET; compares state_version; restarts 5s timer.
 *
 * 409 handling (D6-03): consumes error.from / error.to from response body;
 *   renders <or-conflict-banner mode="status"> with no additional GET.
 */
@customElement('or-status-panel')
export class OrStatusPanel extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ─────────────────────────────────────────────── */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 24px;
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
      font-size: 24px;
      font-weight: 700;
      margin: 0 0 2px;
      color: var(--foreground);
    }

    .page-subtitle {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0;
      font-family: var(--uk-font-monospace, monospace);
    }

    /* ── Status pill ─────────────────────────────────────────────── */
    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 6px 16px;
      border-radius: 9999px;
      font-size: 14px;
      font-weight: 600;
      white-space: nowrap;
    }

    .state-version-chip {
      font-size: 11px;
      font-family: var(--uk-font-monospace, monospace);
      color: var(--muted-foreground);
      background: var(--muted);
      padding: 2px 7px;
      border-radius: 4px;
    }

    /* ── Stats row ───────────────────────────────────────────────── */
    .stats-row {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 12px;
      margin-bottom: 20px;
    }

    @media (max-width: 640px) {
      .stats-row { grid-template-columns: repeat(2, 1fr); }
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px 16px;
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
      font-size: 20px;
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
      padding: 20px 24px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 16px;
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 16px;
      display: flex;
      align-items: center;
      gap: 7px;
    }

    .card-title uk-icon {
      color: var(--muted-foreground);
    }

    /* ── Zone sections ───────────────────────────────────────────── */
    .zone {
      margin-top: 16px;
      padding-top: 14px;
      border-top: 1px solid var(--border);
    }

    .zone-label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .05em;
      color: var(--muted-foreground);
      margin-bottom: 10px;
    }

    .button-row {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      align-items: center;
    }

    /* ── WrapUp countdown ────────────────────────────────────────── */
    .wrapup-countdown {
      font-size: 36px;
      font-family: var(--uk-font-monospace, monospace);
      font-weight: 700;
      color: oklch(0.5 0.18 80);
      letter-spacing: .02em;
      margin-bottom: 14px;
    }

    /* ── Break reason picker ─────────────────────────────────────── */
    .break-picker-header {
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      margin-bottom: 8px;
    }

    .break-reason-list {
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .break-reason-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 6px 4px;
      cursor: pointer;
      border-radius: 6px;
    }

    .break-reason-item:hover {
      background: var(--muted);
    }

    .break-reason-item input[type="radio"] { cursor: pointer; }

    .break-reason-name {
      flex: 1;
      font-size: 14px;
      color: var(--foreground);
    }

    .routable-badge {
      font-size: 10px;
      padding: 1px 6px;
      border-radius: 4px;
      background: var(--muted);
      color: var(--muted-foreground);
      border: 1px solid var(--border);
    }

    .picker-actions {
      display: flex;
      gap: 8px;
      margin-top: 12px;
    }

    /* ── Post-interaction state picker ───────────────────────────── */
    .pi-picker {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    /* ── Break current label ─────────────────────────────────────── */
    .break-current {
      font-size: 13px;
      color: var(--muted-foreground);
      margin-bottom: 10px;
    }

    .break-current-label {
      font-weight: 500;
      color: var(--foreground);
    }

    /* ── Force flag section (D-84) ───────────────────────────────── */
    .force-section {
      border: 1px solid color-mix(in oklch, oklch(0.75 0.18 80) 50%, transparent);
      border-radius: 8px;
      overflow: hidden;
      margin-top: 16px;
    }

    .force-summary {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 8%, transparent);
      padding: 10px 14px;
      cursor: pointer;
      font-size: 13px;
      font-weight: 600;
      color: oklch(0.5 0.18 80);
      display: flex;
      align-items: center;
      gap: 6px;
      user-select: none;
    }

    .force-summary:hover {
      background: color-mix(in oklch, oklch(0.75 0.18 80) 14%, transparent);
    }

    .force-body { padding: 16px; }

    .force-warning-text {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0 0 12px;
      line-height: 1.5;
    }

    .force-targets {
      display: flex;
      flex-direction: column;
      gap: 6px;
      margin-bottom: 12px;
    }

    .force-target-item {
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;
      font-size: 14px;
      color: var(--foreground);
      padding: 4px 2px;
    }

    .force-actions { display: flex; gap: 8px; }

    /* ── Alerts ──────────────────────────────────────────────────── */
    .alert {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px 14px;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 14px;
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

    /* ── Spinner ─────────────────────────────────────────────────── */
    .spinner {
      display: inline-block;
      width: 18px;
      height: 18px;
      border: 2px solid var(--border);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin .6s linear infinite;
      flex-shrink: 0;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    .loading-wrap {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 32px 0;
      color: var(--muted-foreground);
      font-size: 14px;
    }

    /* ── Confirm overlay (force transition dialog) ───────────────── */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: oklch(0 0 0 / 0.5);
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
      max-width: 440px;
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
      line-height: 1.5;
    }

    .action-row {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }

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
  `;

  // --- Public properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'agent-id' }) accessor agentId = '';
  @property({ type: Object, attribute: false }) accessor client!: ApiClient;
  @property({ type: Boolean }) accessor embedded = false;

  // --- Internal state ---
  @state() private accessor _status: AgentStatusResponse | null = null;
  @state() private accessor _loading = true;
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _lastKnownVersion = 0;
  @state() private accessor _pollInterval: ReturnType<typeof setInterval> | null = null;
  @state() private accessor _breakReasons: BreakReason[] = [];
  @state() private accessor _breakReasonsLoading = false;
  @state() private accessor _breakDropdownOpen = false;
  @state() private accessor _selectedBreakReasonId: string | null = null;
  @state() private accessor _forceExpanded = false;
  @state() private accessor _selectedForceTarget: AgentStatus | null = null;
  @state() private accessor _forceConfirmOpen = false;
  @state() private accessor _wrapupSecondsLeft: number | null = null;
  @state() private accessor _wrapupInterval: ReturnType<typeof setInterval> | null = null;
  @state() private accessor _conflictError: { from: string; to: string } | null = null;
  @state() private accessor _transitioning = false;
  @state() private accessor _selectedPostInteractionState: 'ready' | 'not_ready' | null = null;

  private _onKeydown?: (e: KeyboardEvent) => void;

  // --- createRenderRoot: adopt Frankenstyle sheets into shadow root ---

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Lifecycle ---

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('visibilitychange', this._handleVisibilityChange);
    this._startPolling();
    this._onKeydown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && this._forceConfirmOpen) {
        this._forceConfirmOpen = false;
      }
    };
    document.addEventListener('keydown', this._onKeydown);
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('_forceConfirmOpen') && this._forceConfirmOpen) {
      void this.updateComplete.then(() => {
        const cancelBtn = this.shadowRoot?.querySelector<HTMLElement>('.confirm-panel .uk-button-default');
        cancelBtn?.focus();
      });
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('visibilitychange', this._handleVisibilityChange);
    if (this._onKeydown) document.removeEventListener('keydown', this._onKeydown);
    if (this._pollInterval !== null) {
      clearInterval(this._pollInterval);
      this._pollInterval = null;
    }
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
  }

  // --- Polling (D6-26) ---

  private _handleVisibilityChange = (): void => {
    if (document.hidden) {
      if (this._pollInterval !== null) {
        clearInterval(this._pollInterval);
        this._pollInterval = null;
      }
    } else {
      // Resume: immediate GET + restart 5s interval
      void this._fetchStatus().then(() => {
        this._startPolling();
      });
    }
  };

  private _startPolling(): void {
    if (this._pollInterval !== null) {
      clearInterval(this._pollInterval);
    }
    void this._fetchStatus();
    this._pollInterval = setInterval(() => {
      void this._fetchStatus();
    }, 5000);
  }

  private async _fetchStatus(): Promise<void> {
    if (!this.orgId || !this.agentId || !this.client) return;

    try {
      const result = await this.client.GET('/v1/orgs/{org_id}/agents/{id}/status' as never, {
        params: { path: { org_id: this.orgId, id: this.agentId } },
      } as never);

      const { data, error } = result as {
        data: AgentStatusResponse | null;
        error: unknown;
      };

      if (error) {
        this._apiError = (error as { reason?: string })?.reason ?? 'Failed to load status';
        this._loading = false;
        return;
      }

      if (data) {
        // D6-26: compare state_version to detect changes during invisibility
        if (data.state_version !== this._lastKnownVersion) {
          this._status = data;
          this._lastKnownVersion = data.state_version;
          if (data.post_interaction_state) {
            this._selectedPostInteractionState = data.post_interaction_state;
          }
          if (data.status === 'WrapUp' && data.wrapup_until) {
            this._startWrapupCountdown(data.wrapup_until);
          } else {
            this._stopWrapupCountdown();
          }
        } else if (!this._status) {
          this._status = data;
          this._lastKnownVersion = data.state_version;
          if (data.status === 'WrapUp' && data.wrapup_until) {
            this._startWrapupCountdown(data.wrapup_until);
          }
        }
        this._apiError = null;
      }
    } catch {
      this._apiError = 'Network error fetching status';
    } finally {
      this._loading = false;
    }
  }

  // --- WrapUp countdown ---

  private _startWrapupCountdown(wrapupUntil: string): void {
    this._stopWrapupCountdown();
    const update = () => {
      const ms = new Date(wrapupUntil).getTime() - Date.now();
      const secs = Math.max(0, Math.floor(ms / 1000));
      this._wrapupSecondsLeft = secs;
      if (secs <= 0) {
        this._stopWrapupCountdown();
        void this._fetchStatus();
      }
    };
    update();
    this._wrapupInterval = setInterval(update, 1000);
  }

  private _stopWrapupCountdown(): void {
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
    this._wrapupSecondsLeft = null;
  }

  private _formatCountdown(seconds: number): string {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  }

  // --- Break reason picker ---

  async _openBreakDropdown(): Promise<void> {
    this._breakDropdownOpen = true;
    if (this._breakReasons.length === 0) {
      await this._fetchBreakReasons();
    }
  }

  private async _fetchBreakReasons(): Promise<void> {
    if (this._breakReasonsLoading || !this.orgId) return;
    this._breakReasonsLoading = true;
    try {
      const result = await this.client.GET('/v1/orgs/{org_id}/break-reasons' as never, {
        params: {
          path: { org_id: this.orgId },
          query: { include_disabled: false, limit: 100 },
        },
      } as never);
      const { data } = result as { data: { items?: BreakReason[] } | null; error: unknown };
      this._breakReasons = data?.items ?? [];
    } finally {
      this._breakReasonsLoading = false;
    }
  }

  private async _handleConfirmBreak(): Promise<void> {
    if (!this._selectedBreakReasonId) return;
    await this._patchStatus({ to: 'Break', break_reason_id: this._selectedBreakReasonId });
    this._breakDropdownOpen = false;
    this._selectedBreakReasonId = null;
  }

  // --- Transition PATCH ---

  private async _patchStatus(body: {
    to: AgentStatus;
    break_reason_id?: string | null;
    post_interaction_state?: 'ready' | 'not_ready';
    force?: boolean;
  }): Promise<void> {
    if (!this.orgId || !this.agentId || !this.client || this._transitioning) return;
    this._transitioning = true;
    this._conflictError = null;
    try {
      const result = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}/status' as never, {
        params: { path: { org_id: this.orgId, id: this.agentId } },
        body,
      } as never);

      const { data, error } = result as {
        data: AgentStatusResponse | null;
        error: unknown;
      };

      if (error) {
        // D6-03: consume 409 invalid_transition from error.from/error.to body (no re-GET)
        if (
          error &&
          typeof error === 'object' &&
          'from' in error &&
          'to' in error
        ) {
          this._conflictError = {
            from: String((error as { from: unknown }).from ?? ''),
            to: String((error as { to: unknown }).to ?? ''),
          };
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Transition failed';
        return;
      }

      if (data) {
        this._status = data;
        this._lastKnownVersion = data.state_version;
        this._apiError = null;
      } else {
        // 204 No Content — refetch to get updated state
        await this._fetchStatus();
      }
    } finally {
      this._transitioning = false;
    }
  }

  // --- Force transition ---

  async _handleForceTransition(): Promise<void> {
    if (!this._selectedForceTarget) return;
    await this._patchStatus({ to: this._selectedForceTarget, force: true });
    this._forceConfirmOpen = false;
    this._forceExpanded = false;
    this._selectedForceTarget = null;
  }

  // --- Navigate helper ---

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  // ---------------------------------------------------------------------------
  // Render helpers
  // ---------------------------------------------------------------------------

  private _renderPageHeader() {
    return html`
      <div class="page-header">
        <div class="page-header-left">
          ${when(
            !this.embedded,
            () => html`
              <button
                class="back-btn"
                type="button"
                title="Back to Agent"
                @click=${() => this._navigate(`/orgs/${this.orgId}/agents/${this.agentId}`)}
              >
                <uk-icon icon="arrow-left" width="18" height="18"></uk-icon>
              </button>
            `
          )}
          <div>
            <h1 class="page-title">Agent Status</h1>
            ${this._status
              ? html`<p class="page-subtitle">ID: ${this._status.agent_id}</p>`
              : nothing}
          </div>
        </div>
        <div style="display:flex;align-items:center;gap:8px">
          ${this._loading
            ? html`<span class="spinner" title="Loading…"></span>`
            : this._renderStatusPill()}
        </div>
      </div>
    `;
  }

  private _renderStatusPill() {
    if (!this._status) return nothing;
    const style = STATUS_PILL_STYLES[this._status.status] ?? STATUS_PILL_STYLES.Offline;
    return html`
      <div class="status-pill" style="background:${style.bg};color:${style.text};">
        <uk-icon icon="${style.lucideIcon}" width="14" height="14"></uk-icon>
        ${this._status.status}
      </div>
      <span class="state-version-chip" title="State version">v${this._status.state_version}</span>
    `;
  }

  private _renderStatsRow() {
    if (!this._status) return nothing;
    const s = this._status;
    const updatedAgo = (() => {
      try {
        const ms = Date.now() - new Date(s.updated_at).getTime();
        const min = Math.floor(ms / 60000);
        if (min < 1) return 'just now';
        if (min < 60) return `${min}m ago`;
        const h = Math.floor(min / 60);
        return h < 24 ? `${h}h ago` : `${Math.floor(h / 24)}d ago`;
      } catch { return s.updated_at; }
    })();

    return html`
      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-label">Status</div>
          <div class="stat-value" style="font-size:16px;padding-top:2px">${s.status}</div>
          <div class="stat-sub">current state</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Version</div>
          <div class="stat-value">${s.state_version}</div>
          <div class="stat-sub">state rev</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Last Updated</div>
          <div class="stat-value" style="font-size:14px;padding-top:4px">${updatedAgo}</div>
          <div class="stat-sub">${new Date(s.updated_at).toLocaleTimeString()}</div>
        </div>
      </div>
    `;
  }

  private _renderTransitionButtons() {
    if (!this._status) return nothing;
    const status = this._status.status;

    switch (status) {
      case 'Ready':
        return html`
          <div class="zone">
            <div class="zone-label">Transition</div>
            <div class="button-row">
              <button
                type="button"
                class="uk-button uk-button-default uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Set Not Ready</button>
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._openBreakDropdown()}
              >
                Go on Break
                <uk-icon icon="chevron-down" width="14" height="14"></uk-icon>
              </button>
            </div>
            ${this._breakDropdownOpen ? this._renderBreakPicker() : nothing}
          </div>
        `;

      case 'NotReady':
        return html`
          <div class="zone">
            <div class="zone-label">Transition</div>
            <div class="button-row">
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Set Ready</button>
            </div>
          </div>
        `;

      case 'Break':
        return html`
          <div class="zone">
            <div class="zone-label">Transition</div>
            ${when(
              this._status.break_reason_name,
              () => html`
                <p class="break-current">
                  <span class="break-current-label">Current reason:</span>
                  ${this._status!.break_reason_name}
                </p>
              `
            )}
            <div class="button-row">
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Back to Ready</button>
              <button
                type="button"
                class="uk-button uk-button-default uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Set Not Ready</button>
            </div>
          </div>
        `;

      case 'Engaged':
        return html`
          <div class="zone">
            <div class="zone-label">Post-Interaction State</div>
            <div class="pi-picker">
              <label class="break-reason-item">
                <input
                  type="radio"
                  name="pi-state"
                  value="ready"
                  ?checked=${this._selectedPostInteractionState === 'ready'}
                  @change=${() => { this._selectedPostInteractionState = 'ready'; }}
                />
                <span class="break-reason-name">Ready after wrap-up</span>
              </label>
              <label class="break-reason-item">
                <input
                  type="radio"
                  name="pi-state"
                  value="not_ready"
                  ?checked=${this._selectedPostInteractionState === 'not_ready'}
                  @change=${() => { this._selectedPostInteractionState = 'not_ready'; }}
                />
                <span class="break-reason-name">Not Ready after wrap-up</span>
              </label>
            </div>
            <div style="margin-top:10px">
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${!this._selectedPostInteractionState || this._transitioning}
                @click=${() => {
                  if (this._selectedPostInteractionState) {
                    void this._patchStatus({ to: 'Engaged', post_interaction_state: this._selectedPostInteractionState });
                  }
                }}
              >Save</button>
            </div>
          </div>
        `;

      case 'WrapUp':
        return html`
          <div class="zone">
            <div class="zone-label">Wrap-Up</div>
            ${when(
              this._wrapupSecondsLeft !== null && this._wrapupSecondsLeft > 0,
              () => html`
                <div class="wrapup-countdown">
                  ${this._formatCountdown(this._wrapupSecondsLeft!)}
                </div>
              `
            )}
            <div class="button-row">
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Go to Ready now</button>
              <button
                type="button"
                class="uk-button uk-button-default uk-button-small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Go to Not Ready now</button>
            </div>
          </div>
        `;

      case 'Offline':
        return html`
          <div class="zone">
            <p style="color:var(--muted-foreground);font-size:14px;margin:0">
              Agent is offline.
            </p>
          </div>
        `;

      default:
        return nothing;
    }
  }

  private _renderBreakPicker() {
    return html`
      <div style="margin-top:12px;padding:16px;border:1px solid var(--border);border-radius:8px;max-width:300px;background:var(--card)">
        <p class="break-picker-header">Pick a break reason</p>
        ${this._breakReasonsLoading
          ? html`<div class="loading-wrap"><span class="spinner"></span> Loading…</div>`
          : html`
            <ul class="break-reason-list">
              ${this._breakReasons.map(
                (reason) => html`
                  <li class="break-reason-item"
                    @click=${() => { this._selectedBreakReasonId = reason.id; }}
                  >
                    <input
                      type="radio"
                      name="break-reason"
                      value="${reason.id}"
                      ?checked=${this._selectedBreakReasonId === reason.id}
                      @change=${() => { this._selectedBreakReasonId = reason.id; }}
                    />
                    <span class="break-reason-name">${reason.name}</span>
                    <span class="routable-badge">
                      ${reason.routable ? 'routable' : 'not routable'}
                    </span>
                  </li>
                `
              )}
            </ul>
            <div class="picker-actions">
              <button
                type="button"
                class="uk-button uk-button-default uk-button-small"
                @click=${() => {
                  this._breakDropdownOpen = false;
                  this._selectedBreakReasonId = null;
                }}
              >Cancel</button>
              <button
                type="button"
                class="uk-button uk-button-primary uk-button-small"
                ?disabled=${!this._selectedBreakReasonId}
                @click=${() => void this._handleConfirmBreak()}
              >Confirm break</button>
            </div>
          `}
      </div>
    `;
  }

  private _renderForceFlagSection() {
    return html`
      <div class="force-section">
        <div
          class="force-summary"
          role="button"
          aria-expanded="${this._forceExpanded}"
          tabindex="0"
          @click=${() => { this._forceExpanded = !this._forceExpanded; }}
          @keydown=${(e: KeyboardEvent) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              this._forceExpanded = !this._forceExpanded;
            }
          }}
        >
          <uk-icon icon="triangle-alert" width="14" height="14"></uk-icon>
          Force transition (admin override)
          <uk-icon
            icon="${this._forceExpanded ? 'chevron-up' : 'chevron-down'}"
            width="14" height="14"
            style="margin-left:auto"
          ></uk-icon>
        </div>

        ${when(
          this._forceExpanded,
          () => html`
            <div class="force-body">
              <p class="force-warning-text">
                Force transition bypasses the agent state machine. The transition is
                logged at the server (WARN audit entry). Use for operational recovery
                only — the agent may lose interaction context.
              </p>

              <div class="force-targets">
                ${(['Ready', 'NotReady', 'Break', 'Offline'] as AgentStatus[]).map(
                  (target) => html`
                    <label class="force-target-item">
                      <input
                        type="radio"
                        name="force-target"
                        value="${target}"
                        ?checked=${this._selectedForceTarget === target}
                        @change=${() => { this._selectedForceTarget = target; }}
                      />
                      ${target}
                    </label>
                  `
                )}
              </div>

              <div class="force-actions">
                <button
                  type="button"
                  class="uk-button uk-button-default uk-button-small"
                  @click=${() => {
                    this._forceExpanded = false;
                    this._selectedForceTarget = null;
                  }}
                >Cancel</button>
                <button
                  type="button"
                  class="uk-button uk-button-danger uk-button-small"
                  ?disabled=${!this._selectedForceTarget || this._transitioning}
                  @click=${() => { this._forceConfirmOpen = true; }}
                  title="Force transition bypasses state machine — use with caution"
                >
                  <uk-icon icon="triangle-alert" width="14" height="14"></uk-icon>
                  Force to ${this._selectedForceTarget ?? '…'}
                </button>
              </div>
            </div>
          `
        )}
      </div>
    `;
  }

  private _renderForceConfirmDialog() {
    if (!this._forceConfirmOpen) return nothing;
    return html`
      <div
        class="confirm-overlay"
        @click=${(e: MouseEvent) => {
          if (e.target === e.currentTarget) this._forceConfirmOpen = false;
        }}
      >
        <div class="confirm-panel" role="dialog" aria-modal="true" aria-labelledby="force-dialog-title">
          <h2 class="confirm-title" id="force-dialog-title">Force transition?</h2>
          <p class="confirm-body">
            Force transition to <strong>${this._selectedForceTarget}</strong>?
            This bypasses the state machine and is logged at the server.
          </p>
          <div class="action-row">
            <button
              type="button"
              class="uk-button uk-button-default uk-button-small"
              @click=${() => { this._forceConfirmOpen = false; }}
            >Cancel</button>
            <button
              type="button"
              class="uk-button uk-button-danger uk-button-small"
              @click=${() => void this._handleForceTransition()}
            >
              <uk-icon icon="triangle-alert" width="14" height="14"></uk-icon>
              Force transition
            </button>
          </div>
        </div>
      </div>
    `;
  }

  override render() {
    return html`
      ${this._renderPageHeader()}

      ${this._loading
        ? html`<div class="loading-wrap"><span class="spinner"></span> Loading status…</div>`
        : html`
          ${this._renderStatsRow()}

          <div class="info-card">
            <h2 class="card-title">
              <uk-icon icon="activity" width="15" height="15"></uk-icon>
              State &amp; Transitions
            </h2>

            ${when(
              this._conflictError,
              () => html`
                <or-conflict-banner
                  mode="status"
                  .serverValue=${{ from: this._conflictError!.from, to: this._conflictError!.to }}
                  @open-routing:conflict-acknowledged=${() => {
                    this._conflictError = null;
                    void this._fetchStatus();
                  }}
                ></or-conflict-banner>
              `
            )}

            ${when(
              this._apiError && !this._conflictError,
              () => html`
                <div class="alert alert--danger">
                  <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
                  ${this._apiError}
                </div>
              `
            )}

            ${this._renderTransitionButtons()}

            <div style="margin-top:24px">
              ${this._renderForceFlagSection()}
            </div>
          </div>
        `}

      ${this._renderForceConfirmDialog()}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-status-panel': OrStatusPanel;
  }
}
