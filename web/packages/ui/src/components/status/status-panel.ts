// Phase 6 Plan 12: <or-status-panel> — agent status display with polling,
// state machine transition buttons, break reason picker, post-interaction state,
// wrapup countdown, and force-flag admin recovery affordance.
//
// D6-26: 5s polling when visible; pause on document.hidden; immediate GET + state_version
//   compare on resume (STATE-08).
// D6-27: No client-side cache — each poll = fresh GET.
// D6-03: 409 invalid_transition consumed from error.from/.to — no re-GET (D6-04 status variant).
// D-84: force=true admin override; hidden in Advanced disclosure; warning color.
// Per-component Shoelace imports (D6-08) for Phase 7 tree-shaking budget.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/dropdown/dropdown.js';
import '@shoelace-style/shoelace/dist/components/menu/menu.js';
import '@shoelace-style/shoelace/dist/components/radio-group/radio-group.js';
import '@shoelace-style/shoelace/dist/components/radio/radio.js';
import '@shoelace-style/shoelace/dist/components/badge/badge.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/details/details.js';
import '@shoelace-style/shoelace/dist/components/divider/divider.js';

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
// Status pill color map (D6-V-18)
// ---------------------------------------------------------------------------

export const STATUS_PILL_STYLES: Record<AgentStatus, { bg: string; text: string; icon: string }> = {
  Ready: {
    bg: 'color-mix(in srgb, var(--sl-color-success-500, #12b76a) 14%, transparent)',
    text: 'var(--sl-color-success-700, #027a48)',
    icon: 'circle-fill',
  },
  NotReady: {
    bg: 'color-mix(in srgb, var(--sl-color-danger-500, #f04438) 12%, transparent)',
    text: 'var(--sl-color-danger-700, #b42318)',
    icon: 'dash-circle-fill',
  },
  Break: {
    bg: 'color-mix(in srgb, var(--sl-color-warning-500, #b54708) 10%, transparent)',
    text: 'var(--sl-color-warning-500, #b54708)',
    icon: 'pause-circle-fill',
  },
  Engaged: {
    bg: 'color-mix(in srgb, var(--sl-color-success-500, #027a48) 10%, transparent)',
    text: 'var(--sl-color-success-500, #027a48)',
    icon: 'telephone-fill',
  },
  WrapUp: {
    bg: 'color-mix(in srgb, var(--sl-color-warning-500, #b54708) 5%, transparent)',
    text: 'var(--sl-color-warning-500, #b54708)',
    icon: 'hourglass-split',
  },
  Offline: {
    bg: 'var(--sl-color-neutral-100, #f5f5f5)',
    text: 'var(--or-color-text-muted, #737373)',
    icon: 'power',
  },
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * <or-status-panel> — agent status operational view.
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
    }

    .top-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 24px;
    }

    .top-bar-spacer { flex: 1; }

    .card {
      background: var(--or-color-card-bg, #ffffff);
      border: 1px solid var(--or-color-card-border, #e5e5e5);
      border-radius: var(--or-radius-md, 8px);
      padding: 24px;
      max-width: 600px;
    }

    .card-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 20px;
    }

    .card-title {
      font-size: 18px;
      font-weight: 600;
      color: var(--or-color-text-strong, #171717);
      margin: 0;
      flex: 1;
    }

    /* Status pill (D6-V-18) */
    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      height: 28px;
      padding: 4px 12px;
      border-radius: var(--or-radius-full, 9999px);
      font-size: 13px;
      font-weight: 500;
      white-space: nowrap;
    }

    .state-version-chip {
      font-size: 11px;
      font-family: var(--or-font-mono, monospace);
      color: var(--or-color-text-muted, #737373);
      background: var(--sl-color-neutral-100, #f5f5f5);
      padding: 2px 6px;
      border-radius: 3px;
    }

    /* Section zones */
    .zone {
      margin-top: 20px;
      padding-top: 16px;
      border-top: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .zone-label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--or-color-text-muted, #737373);
      margin-bottom: 12px;
    }

    .button-row {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      align-items: center;
    }

    /* Countdown */
    .wrapup-countdown {
      font-size: 32px;
      font-family: var(--or-font-mono, monospace);
      font-weight: 600;
      color: var(--sl-color-warning-500, #b54708);
      letter-spacing: 0.02em;
      margin-bottom: 12px;
    }

    /* Break reason picker */
    .break-picker-header {
      font-size: 13px;
      font-weight: 600;
      color: var(--or-color-text-strong, #171717);
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
    }

    .break-reason-item:hover {
      background: var(--or-color-row-hover, #fafafa);
      border-radius: 4px;
    }

    .break-reason-item input[type="radio"] {
      cursor: pointer;
    }

    .break-reason-name {
      flex: 1;
      font-size: 14px;
    }

    .routable-badge {
      font-size: 10px;
      padding: 1px 5px;
      border-radius: 3px;
      background: var(--sl-color-neutral-100, #f5f5f5);
      color: var(--or-color-text-muted, #737373);
      border: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .picker-actions {
      display: flex;
      gap: 8px;
      margin-top: 12px;
    }

    /* Post-interaction state picker */
    .pi-picker {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    /* Force flag (D6-V-19) */
    .force-section {
      margin-top: 24px;
      border: 1px solid var(--sl-color-warning-500, #b54708);
      border-radius: 4px;
      overflow: hidden;
    }

    .force-summary {
      background: color-mix(in srgb, var(--sl-color-warning-500, #b54708) 5%, transparent);
      padding: 8px 14px;
      cursor: pointer;
      font-size: 13px;
      font-weight: 600;
      color: var(--sl-color-warning-500, #b54708);
      display: flex;
      align-items: center;
      gap: 6px;
      user-select: none;
    }

    .force-body {
      padding: 16px;
    }

    .force-warning-text {
      font-size: 13px;
      color: var(--or-color-text-body, #404040);
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
    }

    .force-actions {
      display: flex;
      gap: 8px;
    }

    /* Loading / error */
    .loading-state {
      display: flex;
      align-items: center;
      gap: 12px;
      color: var(--or-color-text-muted, #737373);
      padding: 24px 0;
    }

    .break-current {
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
      margin-bottom: 8px;
    }
    /* Inline confirm panel — D7-04: <sl-dialog> has broken focus-trap inside nested Shadow DOM (shoelace#709, #1382). Embed mounts inside Shadow DOM, so this is replaced with an inline role=dialog + manual focus management. */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .confirm-panel {
      background: var(--or-color-card-bg, #ffffff);
      border: 1px solid var(--or-color-card-border, #d1d5db);
      border-radius: var(--sl-border-radius-medium, 6px);
      padding: 20px;
      max-width: 480px;
      width: 90%;
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
    }
    .confirm-title { margin: 0 0 12px 0; font-size: 18px; font-weight: 600; }
    .confirm-body { margin: 0 0 20px 0; }
    .action-row { display: flex; gap: 8px; justify-content: flex-end; }
    /* Inline confirm panel — D7-04: <sl-dialog> has broken focus-trap inside nested Shadow DOM (shoelace#709, #1382). Embed mounts inside Shadow DOM, so this is replaced with an inline role=dialog + manual focus management. */
    .confirm-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .confirm-panel {
      background: var(--or-color-card-bg, #ffffff);
      border: 1px solid var(--or-color-card-border, #d1d5db);
      border-radius: var(--sl-border-radius-medium, 6px);
      padding: 20px;
      max-width: 480px;
      width: 90%;
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
    }
    .confirm-title { margin: 0 0 12px 0; font-size: 18px; font-weight: 600; }
    .confirm-body { margin: 0 0 20px 0; }
    .action-row { display: flex; gap: 8px; justify-content: flex-end; }



    .break-current-label {
      font-weight: 500;
      color: var(--or-color-text-body, #404040);
    }
  `;

  // --- Public properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'agent-id' }) agentId = '';
  @property({ type: Object, attribute: false }) client!: ApiClient;
  @property({ type: Boolean }) embedded = false;

  // --- Internal state ---
  @state() private _status: AgentStatusResponse | null = null;
  @state() private _loading = true;
  @state() private _apiError: string | null = null;
  @state() private _lastKnownVersion = 0;
  @state() private _pollInterval: ReturnType<typeof setInterval> | null = null;
  @state() private _breakReasons: BreakReason[] = [];
  @state() private _breakReasonsLoading = false;
  @state() private _breakDropdownOpen = false;
  @state() private _selectedBreakReasonId: string | null = null;
  @state() private _forceExpanded = false;
  @state() private _selectedForceTarget: AgentStatus | null = null;
  @state() private _forceConfirmOpen = false;
  @state() private _wrapupSecondsLeft: number | null = null;
  @state() private _wrapupInterval: ReturnType<typeof setInterval> | null = null;
  @state() private _conflictError: { from: string; to: string } | null = null;
  @state() private _transitioning = false;
  @state() private _selectedPostInteractionState: 'ready' | 'not_ready' | null = null;

  // --- Lifecycle ---

  private _onKeydown?: (e: KeyboardEvent) => void;

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('_forceConfirmOpen') && this._forceConfirmOpen) {
      this.setAttribute('aria-live', 'polite');
      this.updateComplete.then(() => {
        const btn = this.shadowRoot?.querySelector('.force-btn') as HTMLElement;
        if (btn) btn.focus();
      });
    } else if (changed.has('_forceConfirmOpen') && !this._forceConfirmOpen) {
      this.removeAttribute('aria-live');
    }
  }

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

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('visibilitychange', this._handleVisibilityChange);
    if (this._pollInterval !== null) {
      clearInterval(this._pollInterval);
      this._pollInterval = null;
      if (this._onKeydown) {
      document.removeEventListener('keydown', this._onKeydown);
    }
  }
    if (this._wrapupInterval !== null) {
      clearInterval(this._wrapupInterval);
      this._wrapupInterval = null;
    }
  }

  // --- Polling (D6-26) ---

  private _handleVisibilityChange = (): void => {
    if (document.hidden) {
      // Pause polling while tab is hidden
      if (this._pollInterval !== null) {
        clearInterval(this._pollInterval);
        this._pollInterval = null;
      }
    } else {
      // Resume: fire immediate GET + restart 5s interval
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
          // Update post-interaction state selection from server
          if (data.post_interaction_state) {
            this._selectedPostInteractionState = data.post_interaction_state;
          }
          // Update wrapup countdown
          if (data.status === 'WrapUp' && data.wrapup_until) {
            this._startWrapupCountdown(data.wrapup_until);
          } else {
            this._stopWrapupCountdown();
          }
        } else if (!this._status) {
          // First load — set even if version is 0
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
        // WrapUp expired — poll immediately
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

  private _renderStatusPill() {
    if (!this._status) return null;
    const style = STATUS_PILL_STYLES[this._status.status] ?? STATUS_PILL_STYLES.Offline;
    return html`
      <div
        class="status-pill"
        style="background:${style.bg};color:${style.text};"
      >
        <sl-icon name="${style.icon}"></sl-icon>
        ${this._status.status}
      </div>
      <span class="state-version-chip" title="State version">v${this._status.state_version}</span>
    `;
  }

  private _renderTransitionButtons() {
    if (!this._status) return null;
    const status = this._status.status;

    switch (status) {
      case 'Ready':
        return html`
          <div class="zone">
            <div class="zone-label">Transition</div>
            <div class="button-row">
              <sl-button
                variant="default"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Set Not Ready</sl-button>
              <div style="position:relative">
                <sl-button
                  variant="primary"
                  size="small"
                  ?disabled=${this._transitioning}
                  @click=${() => void this._openBreakDropdown()}
                >
                  Go on Break
                  <sl-icon slot="suffix" name="chevron-down"></sl-icon>
                </sl-button>
              </div>
            </div>
            ${this._breakDropdownOpen ? this._renderBreakPicker() : null}
          </div>
        `;

      case 'NotReady':
        return html`
          <div class="zone">
            <div class="zone-label">Transition</div>
            <div class="button-row">
              <sl-button
                variant="primary"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Set Ready</sl-button>
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
              <sl-button
                variant="primary"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Back to Ready</sl-button>
              <sl-button
                variant="default"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Set Not Ready</sl-button>
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
              <sl-button
                variant="primary"
                size="small"
                ?disabled=${!this._selectedPostInteractionState || this._transitioning}
                @click=${() => {
                  if (this._selectedPostInteractionState) {
                    void this._patchStatus({ to: 'Engaged', post_interaction_state: this._selectedPostInteractionState });
                  }
                }}
              >Save</sl-button>
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
              <sl-button
                variant="primary"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'Ready' })}
              >Go to Ready now</sl-button>
              <sl-button
                variant="default"
                size="small"
                ?disabled=${this._transitioning}
                @click=${() => void this._patchStatus({ to: 'NotReady' })}
              >Go to Not Ready now</sl-button>
            </div>
          </div>
        `;

      case 'Offline':
        return html`
          <div class="zone">
            <p style="color:var(--or-color-text-muted,#737373);font-size:14px;margin:0">
              Agent is offline.
            </p>
          </div>
        `;

      default:
        return null;
    }
  }

  private _renderBreakPicker() {
    return html`
      <div style="margin-top:12px;padding:12px;border:1px solid var(--or-color-divider,#e5e5e5);border-radius:6px;max-width:280px">
        <p class="break-picker-header">Pick a break reason</p>
        ${when(
          this._breakReasonsLoading,
          () => html`<sl-spinner></sl-spinner>`,
          () => html`
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
              <sl-button
                variant="text"
                size="small"
                @click=${() => {
                  this._breakDropdownOpen = false;
                  this._selectedBreakReasonId = null;
                }}
              >Cancel</sl-button>
              <sl-button
                variant="primary"
                size="small"
                ?disabled=${!this._selectedBreakReasonId}
                @click=${() => void this._handleConfirmBreak()}
              >Confirm break</sl-button>
            </div>
          `
        )}
      </div>
    `;
  }

  private _renderForceFlagSection() {
    // D6-V-19: Force flag hidden behind Advanced disclosure; warning color
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
          <sl-icon name="exclamation-triangle"></sl-icon>
          Force transition (admin override)
          <sl-icon name="${this._forceExpanded ? 'chevron-up' : 'chevron-down'}" style="margin-left:auto"></sl-icon>
        </div>

        ${when(
          this._forceExpanded,
          () => html`
            <div class="force-body">
              <p class="force-warning-text">
                Force transition bypasses the agent state machine. The transition is
                logged at the server (WARN audit entry). Use for operational recovery
                only — the agent may lose interaction context. v0.1 stub auth allows
                any caller; v1 AUTH restricts to org_admin role.
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
                <sl-button
                  variant="text"
                  size="small"
                  @click=${() => {
                    this._forceExpanded = false;
                    this._selectedForceTarget = null;
                  }}
                >Cancel</sl-button>
                <sl-button
                  variant="warning"
                  size="small"
                  ?disabled=${!this._selectedForceTarget || this._transitioning}
                  @click=${() => { this._forceConfirmOpen = true; }}
                >
                  <sl-icon slot="prefix" name="exclamation-triangle"></sl-icon>
                  Force to ${this._selectedForceTarget ?? '…'}
                </sl-button>
              </div>
            </div>
          `
        )}
      </div>

      <!-- Force confirmation dialog -->
      ${when(this._forceConfirmOpen, () => html`
        <div class="confirm-overlay" role="presentation" @click=${() => { this._forceConfirmOpen = false; }}>
          <div class="confirm-panel"
               role="dialog"
               aria-modal="true"
               aria-labelledby="force-title"
               @click=${(e: Event) => e.stopPropagation()}>
            <h3 id="force-title" class="confirm-title">Force transition?</h3>
            <p class="confirm-body">
              Force transition to <strong>${this._selectedForceTarget}</strong>?
              This bypasses the state machine and is logged at the server.
            </p>
            <div class="action-row">
              <sl-button
                variant="default"
                size="small"
                @click=${() => { this._forceConfirmOpen = false; }}
              >Cancel</sl-button>
              <sl-button
                variant="warning"
                size="small"
                class="force-btn"
                @click=${() => void this._handleForceTransition()}
              >
                <sl-icon slot="prefix" name="exclamation-triangle"></sl-icon>
                Force transition
              </sl-button>
            </div>
          </div>
        </div>
      `)}
    `;
  }

  override render() {
    return html`
      ${when(
        !this.embedded,
        () => html`
          <div class="top-bar">
            <sl-button
              variant="text"
              @click=${() =>
                this._navigate(`/orgs/${this.orgId}/agents/${this.agentId}`)}
            >
              <sl-icon slot="prefix" name="arrow-left"></sl-icon>
              Back to Agent
            </sl-button>
            <div class="top-bar-spacer"></div>
          </div>
        `
      )}

      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Agent Status</h2>
          ${this._loading
            ? html`<sl-spinner></sl-spinner>`
            : this._renderStatusPill()}
        </div>

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
            <sl-alert variant="danger" open style="margin-bottom:16px">
              <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
              ${this._apiError}
            </sl-alert>
          `
        )}

        ${when(
          this._loading,
          () => html`
            <div class="loading-state">
              <sl-spinner></sl-spinner>
              Loading status…
            </div>
          `,
          () => this._renderTransitionButtons()
        )}

        <!-- Force flag advanced disclosure (D6-V-19) -->
        <div style="margin-top:24px">
          ${this._renderForceFlagSection()}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-status-panel': OrStatusPanel;
  }
}
