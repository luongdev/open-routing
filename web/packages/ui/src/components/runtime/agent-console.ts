// v0.4 W5 — <or-agent-console>: a minimal agent surface to take a routed call.
// Pick an agent, toggle presence (Ready/NotReady), then watch ringing offers and
// accept/reject them; an accepted call shows an active-call panel + Complete.
//
// Transport: REST + 2s polling (mirrors route-tester / ops-view). The WS gateway
// is the engine's real-time push path, but a browser can't set the
// X-Org-Id/X-Agent-Id headers it authenticates on — real-time WS-push for browsers
// needs a gateway browser-auth mechanism (deferred). Call AUDIO (LiveKit client)
// is stubbed here; it lands with the real-media adapter + a media environment.
//
// AUTH SCOPE (cross-AI review HIGH): this console acts via the org-trusted REST
// accept/reject/complete endpoints, which have NO agent-ownership/lease fence (that
// lives on the WS command path). So it is an ADMIN/DEV tool under the trusted-host
// X-Org-Id model. A real per-agent self-service console MUST enforce agent-scoped
// auth (the deferred browser-auth) or sit behind an embedding BFF — do NOT expose
// this directly to non-admin agents as-is.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

type AgentStatus = 'Offline' | 'NotReady' | 'Ready' | 'Engaged' | 'WrapUp' | 'Break';
type ReservationState = 'offered' | 'accepted' | 'rejected' | 'timeout' | 'cancelled' | 'completed';

interface Reservation {
  id: string;
  route_request_id: string;
  agent_id: string;
  state: ReservationState;
  attempt: number;
  offered_at: string;
  expires_at: string;
  reason?: string | null;
}

const PILL_MUTED = { bg: 'var(--muted)', text: 'var(--muted-foreground)' };
const STATUS_PILL: Record<string, { bg: string; text: string }> = {
  Ready: { bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent)', text: 'oklch(0.45 0.18 145)' },
  Engaged: { bg: 'color-mix(in oklch, oklch(0.55 0.2 250) 16%, transparent)', text: 'oklch(0.4 0.2 250)' },
  WrapUp: { bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 18%, transparent)', text: 'oklch(0.5 0.18 80)' },
  NotReady: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
  Break: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
  Offline: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
};

/**
 * <or-agent-console> — minimal agent console (presence + take/handle a call).
 * Attributes: org-id, agent-id. Properties: client.
 */
@customElement('or-agent-console')
export class OrAgentConsole extends LitElement {
  static override styles = css`
    :host { display: block; padding: 24px; background: var(--background); min-height: 100%; }
    .page-title { font-size: 24px; font-weight: 700; margin: 0 0 2px; color: var(--foreground); }
    .page-subtitle { font-size: 13px; color: var(--muted-foreground); margin: 0 0 20px; }
    .card {
      background: var(--card); border: 1px solid var(--border); border-radius: 12px;
      padding: 20px 22px; box-shadow: var(--shadow-sm); margin-bottom: 16px; max-width: 560px;
    }
    .card-title { font-size: 15px; font-weight: 600; margin: 0 0 14px; display: flex; align-items: center; gap: 7px; }
    .field-label { display: block; font-size: 12px; font-weight: 600; margin-bottom: 5px; }
    .field input {
      width: 100%; box-sizing: border-box; padding: 8px 10px; border: 1px solid var(--border);
      border-radius: 8px; background: var(--background); color: var(--foreground); font-size: 14px;
      font-family: var(--uk-font-monospace, monospace);
    }
    .pill { display: inline-flex; align-items: center; gap: 5px; padding: 3px 11px; border-radius: 9999px; font-size: 12px; font-weight: 600; }
    .row { display: flex; align-items: center; gap: 10px; }
    .presence-actions { display: flex; gap: 8px; margin-top: 12px; }
    .res-row { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-top: 1px solid var(--border); }
    .res-row:first-of-type { border-top: none; }
    .res-meta { flex: 1; min-width: 0; }
    .res-route { font-family: var(--uk-font-monospace, monospace); font-size: 12px; color: var(--foreground); }
    .res-sub { font-size: 11px; color: var(--muted-foreground); margin-top: 2px; }
    .actions { display: flex; gap: 6px; }
    .empty { color: var(--muted-foreground); font-size: 13px; padding: 12px 0; }
    .audio-note {
      margin-top: 10px; padding: 8px 10px; border-radius: 8px; font-size: 12px;
      background: var(--muted); color: var(--muted-foreground);
    }
    .alert--danger {
      display: flex; gap: 9px; padding: 10px 13px; border-radius: 8px; font-size: 13px; margin-bottom: 12px;
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent); color: var(--destructive);
    }
    .spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid var(--border); border-top-color: var(--primary); border-radius: 50%; animation: spin .6s linear infinite; vertical-align: middle; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .live-chip { font-size: 11px; color: var(--muted-foreground); display: inline-flex; align-items: center; gap: 5px; margin-top: 10px; }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'agent-id' }) accessor agentId = '';
  @property({ type: Object, attribute: false }) accessor client!: ApiClient;

  @state() private accessor _status: AgentStatus | null = null;
  @state() private accessor _reservations: Reservation[] = [];
  @state() private accessor _error: string | null = null;
  @state() private accessor _busyId: string | null = null;

  private _pollTimer: ReturnType<typeof setInterval> | null = null;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override updated(changed: Map<string, unknown>): void {
    if ((changed.has('agentId') || changed.has('client') || changed.has('orgId')) && this.orgId && this.agentId && this.client) {
      void this._refresh();
      this._startPolling();
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._stopPolling();
  }

  private _startPolling(): void {
    this._stopPolling();
    this._pollTimer = setInterval(() => void this._refresh(), 2000);
  }

  private _stopPolling(): void {
    if (this._pollTimer !== null) { clearInterval(this._pollTimer); this._pollTimer = null; }
  }

  private async _refresh(): Promise<void> {
    if (!this.orgId || !this.agentId || !this.client) return;
    const [statusRes, resvRes] = await Promise.all([
      this.client.GET('/v1/orgs/{org_id}/agents/{id}/status' as never, { params: { path: { org_id: this.orgId, id: this.agentId } } } as never),
      this.client.GET('/v1/orgs/{org_id}/agents/{id}/reservations' as never, { params: { path: { org_id: this.orgId, id: this.agentId } } } as never),
    ]);
    const st = (statusRes as { data: { status?: AgentStatus } | null }).data;
    if (st?.status) this._status = st.status;
    const rs = (resvRes as { data: { items?: Reservation[] } | null }).data;
    this._reservations = rs?.items ?? [];
  }

  private async _setPresence(to: 'Ready' | 'NotReady'): Promise<void> {
    if (!this.client) return;
    this._error = null;
    const res = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}/status' as never, {
      params: { path: { org_id: this.orgId, id: this.agentId } }, body: { to },
    } as never);
    const { error } = res as { error: unknown };
    if (error) this._error = (error as { reason?: string })?.reason ?? `could not go ${to}`;
    await this._refresh();
  }

  private async _resolve(id: string, action: 'accept' | 'reject' | 'complete'): Promise<void> {
    if (!this.client || this._busyId) return;
    this._busyId = id;
    this._error = null;
    try {
      const res = await this.client.POST(`/v1/orgs/{org_id}/reservations/{id}/${action}` as never, {
        params: { path: { org_id: this.orgId, id } },
      } as never);
      const { error } = res as { error: unknown };
      if (error) this._error = (error as { reason?: string })?.reason ?? `${action} failed`;
      await this._refresh();
    } finally {
      this._busyId = null;
    }
  }

  private _renderReservation(r: Reservation) {
    const busy = this._busyId === r.id;
    return html`
      <div class="res-row">
        <span class="pill" style="background:var(--muted);color:var(--muted-foreground)">${r.state}</span>
        <div class="res-meta">
          <div class="res-route">route ${r.route_request_id.slice(0, 8)}</div>
          <div class="res-sub">attempt #${r.attempt}${r.reason ? ` · ${r.reason}` : ''}</div>
        </div>
        <div class="actions">
          ${r.state === 'offered'
            ? html`
                <button type="button" class="uk-button uk-button-primary uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(r.id, 'accept')}>Accept</button>
                <button type="button" class="uk-button uk-button-default uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(r.id, 'reject')}>Reject</button>`
            : nothing}
          ${r.state === 'accepted'
            ? html`<button type="button" class="uk-button uk-button-primary uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(r.id, 'complete')}>Complete</button>`
            : nothing}
          ${busy ? html`<span class="spinner"></span>` : nothing}
        </div>
      </div>
      ${r.state === 'accepted'
        ? html`<div class="audio-note">📞 On call — audio bridges over LiveKit once the real-media adapter + media environment are wired (v0.4 W3).</div>`
        : nothing}
    `;
  }

  override render() {
    const pill = STATUS_PILL[this._status ?? 'Offline'] ?? PILL_MUTED;
    return html`
      <h1 class="page-title">Agent Console</h1>
      <p class="page-subtitle">Go Ready, take a routed call, and handle it — accept, reject, or complete an offer.</p>

      <div class="card">
        <h2 class="card-title"><uk-icon icon="user" width="15" height="15"></uk-icon> Agent</h2>
        <div class="field">
          <label class="field-label">Agent ID</label>
          <input .value=${this.agentId} placeholder="agent UUID" @input=${(e: Event) => { this.agentId = (e.target as HTMLInputElement).value.trim(); }} />
        </div>
        ${when(this.agentId && this._status, () => html`
          <div class="row" style="margin-top:12px">
            <span style="font-size:13px;color:var(--muted-foreground)">Status</span>
            <span class="pill" style=${`background:${pill.bg};color:${pill.text}`}>${this._status}</span>
          </div>
          <div class="presence-actions">
            <button type="button" class="uk-button uk-button-primary uk-button-small" @click=${() => void this._setPresence('Ready')}>Go Ready</button>
            <button type="button" class="uk-button uk-button-default uk-button-small" @click=${() => void this._setPresence('NotReady')}>Go NotReady</button>
          </div>
        `)}
        ${this._pollTimer !== null ? html`<div class="live-chip"><span class="spinner"></span> live — polling every 2s</div>` : nothing}
      </div>

      <div class="card">
        <h2 class="card-title"><uk-icon icon="phone-call" width="15" height="15"></uk-icon> Offers & calls</h2>
        ${when(this._error, () => html`<div class="alert--danger"><uk-icon icon="alert-triangle" width="15" height="15"></uk-icon>${this._error}</div>`)}
        ${this._reservations.length === 0
          ? html`<div class="empty">No ringing offers. Go Ready and route a call (Route Tester) to see one here.</div>`
          : this._reservations.map((r) => this._renderReservation(r))}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-agent-console': OrAgentConsole;
  }
}
