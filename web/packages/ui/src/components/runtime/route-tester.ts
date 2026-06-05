// v0.2 Wave 3 — <or-route-tester>: a hands-on page for driving a live route
// request through a published flow and resolving its reservations (accept /
// reject / complete) without curl. Picks a channel+entry binding, POSTs a
// route request, then polls status + reservations + trace until the run
// settles. Mirrors status-panel's poll-while-visible pattern.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

type RouteStatus = 'pending' | 'running' | 'waiting' | 'waiting_match' | 'offering' | 'completed' | 'failed' | 'cancelled';
type ReservationState = 'offered' | 'accepted' | 'rejected' | 'timeout' | 'cancelled' | 'completed';

interface RouteRequest {
  id: string;
  channel: string;
  entry_code: string;
  flow_version_id?: string;
  flow_code?: string | null;
  status: RouteStatus;
  failure_code?: string | null;
  trace_id?: string;
  created_at: string;
  updated_at?: string;
}

interface Reservation {
  id: string;
  route_request_id: string;
  agent_id: string;
  state: ReservationState;
  attempt: number;
  offered_at: string;
  expires_at: string;
  resolved_at?: string | null;
  reason?: string | null;
}

interface TraceStep {
  index: number;
  node_id: string;
  node_kind: string;
  status: 'ok' | 'error' | 'skipped' | 'suspended';
  port?: string | null;
  error?: string | null;
}

interface Trace {
  id: string;
  outcome?: string | null;
  steps: TraceStep[];
}

interface FlowEntryBinding {
  channel: string;
  entry_code: string;
  flow_code: string;
  active: boolean;
}

const ROUTE_PILL: Record<RouteStatus, { bg: string; text: string }> = {
  pending: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
  running: { bg: 'color-mix(in oklch, oklch(0.55 0.2 250) 14%, transparent)', text: 'oklch(0.4 0.2 250)' },
  waiting: { bg: 'color-mix(in oklch, oklch(0.6 0.16 220) 16%, transparent)', text: 'oklch(0.42 0.16 220)' },
  waiting_match: { bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 18%, transparent)', text: 'oklch(0.5 0.18 80)' },
  offering: { bg: 'color-mix(in oklch, oklch(0.7 0.16 300) 18%, transparent)', text: 'oklch(0.46 0.16 300)' },
  completed: { bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent)', text: 'oklch(0.45 0.18 145)' },
  failed: { bg: 'color-mix(in oklch, var(--destructive) 12%, transparent)', text: 'var(--destructive)' },
  cancelled: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
};

const FALLBACK_PILL = { bg: 'var(--muted)', text: 'var(--muted-foreground)' };

const RES_PILL: Record<ReservationState, { bg: string; text: string }> = {
  offered: { bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 18%, transparent)', text: 'oklch(0.5 0.18 80)' },
  accepted: { bg: 'color-mix(in oklch, oklch(0.55 0.2 250) 14%, transparent)', text: 'oklch(0.4 0.2 250)' },
  completed: { bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent)', text: 'oklch(0.45 0.18 145)' },
  rejected: { bg: 'color-mix(in oklch, var(--destructive) 12%, transparent)', text: 'var(--destructive)' },
  timeout: { bg: 'color-mix(in oklch, var(--destructive) 10%, transparent)', text: 'var(--destructive)' },
  cancelled: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
};

const STEP_DOT: Record<TraceStep['status'], string> = {
  ok: 'oklch(0.65 0.18 145)',
  error: 'var(--destructive)',
  skipped: 'var(--muted-foreground)',
  suspended: 'oklch(0.5 0.18 80)',
};

/** Terminal route statuses stop the poll loop. */
const TERMINAL: ReadonlySet<RouteStatus> = new Set(['completed', 'failed', 'cancelled']);

/**
 * <or-route-tester> — drive a route request live and resolve its reservations.
 *
 * Attributes: org-id. Properties: client.
 */
@customElement('or-route-tester')
export class OrRouteTester extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }
    .page-header { margin-bottom: 20px; }
    .page-title { font-size: 24px; font-weight: 700; margin: 0 0 2px; color: var(--foreground); }
    .page-subtitle { font-size: 13px; color: var(--muted-foreground); margin: 0; }

    .grid {
      display: grid;
      grid-template-columns: minmax(340px, 420px) 1fr;
      gap: 20px;
      align-items: start;
    }
    @media (max-width: 860px) { .grid { grid-template-columns: 1fr; } }

    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 20px 22px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 16px;
    }
    .card-title {
      font-size: 15px; font-weight: 600; color: var(--foreground);
      margin: 0 0 16px; display: flex; align-items: center; gap: 7px;
    }
    .card-title uk-icon { color: var(--muted-foreground); }

    .field { margin-bottom: 14px; }
    .field-label {
      display: block; font-size: 12px; font-weight: 600; color: var(--foreground);
      margin-bottom: 5px;
    }
    .field select, .field input, .field textarea {
      width: 100%; box-sizing: border-box;
      padding: 8px 10px; border: 1px solid var(--border); border-radius: 8px;
      background: var(--background); color: var(--foreground);
      font-size: 14px; font-family: inherit;
    }
    .field textarea {
      font-family: var(--uk-font-monospace, monospace); font-size: 13px;
      min-height: 120px; resize: vertical;
    }
    .field-hint { font-size: 11px; color: var(--muted-foreground); margin-top: 4px; }

    .pill {
      display: inline-flex; align-items: center; gap: 5px;
      padding: 3px 11px; border-radius: 9999px;
      font-size: 12px; font-weight: 600; white-space: nowrap;
    }

    .alert {
      display: flex; align-items: center; gap: 9px;
      padding: 10px 13px; border-radius: 8px; font-size: 13px; margin-bottom: 14px;
    }
    .alert--danger {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent);
      color: var(--destructive);
    }

    .res-row {
      display: flex; align-items: center; gap: 12px;
      padding: 12px 0; border-top: 1px solid var(--border);
    }
    .res-row:first-of-type { border-top: none; }
    .res-meta { flex: 1; min-width: 0; }
    .res-agent {
      font-family: var(--uk-font-monospace, monospace); font-size: 12px;
      color: var(--foreground); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .res-sub { font-size: 11px; color: var(--muted-foreground); margin-top: 2px; }
    .res-actions { display: flex; gap: 6px; flex-shrink: 0; }

    .trace-step {
      display: flex; align-items: baseline; gap: 10px;
      padding: 7px 0; border-bottom: 1px dashed var(--border); font-size: 13px;
    }
    .trace-step:last-child { border-bottom: none; }
    .step-dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; align-self: center; }
    .step-kind { font-weight: 600; color: var(--foreground); }
    .step-node { font-family: var(--uk-font-monospace, monospace); font-size: 11px; color: var(--muted-foreground); }
    .step-port {
      margin-left: auto; font-size: 11px; font-family: var(--uk-font-monospace, monospace);
      background: var(--muted); color: var(--muted-foreground); padding: 1px 7px; border-radius: 4px;
    }
    .step-error { color: var(--destructive); font-size: 11px; }

    .empty { color: var(--muted-foreground); font-size: 13px; padding: 16px 0; }

    .meta-grid {
      display: grid; grid-template-columns: auto 1fr; gap: 6px 14px;
      font-size: 13px; align-items: baseline;
    }
    .meta-key { color: var(--muted-foreground); }
    .meta-val { color: var(--foreground); font-family: var(--uk-font-monospace, monospace); font-size: 12px; word-break: break-all; }

    .spinner {
      display: inline-block; width: 14px; height: 14px;
      border: 2px solid var(--border); border-top-color: var(--primary);
      border-radius: 50%; animation: spin .6s linear infinite; vertical-align: middle;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .live-chip {
      font-size: 11px; color: var(--muted-foreground);
      display: inline-flex; align-items: center; gap: 5px;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object, attribute: false }) accessor client!: ApiClient;

  @state() private accessor _bindings: FlowEntryBinding[] = [];
  @state() private accessor _selectedBinding = '';
  @state() private accessor _channel = '';
  @state() private accessor _entryCode = '';
  @state() private accessor _inputJson = '{}';
  @state() private accessor _creating = false;
  @state() private accessor _createError: string | null = null;

  @state() private accessor _route: RouteRequest | null = null;
  @state() private accessor _reservations: Reservation[] = [];
  @state() private accessor _trace: Trace | null = null;
  @state() private accessor _actionError: string | null = null;
  @state() private accessor _busyResId: string | null = null;
  @state() private accessor _inputValue = '';
  @state() private accessor _submittingInput = false;

  private _pollTimer: ReturnType<typeof setInterval> | null = null;
  private _bindingsFetched = false;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // orgId + client arrive as reactive properties after connect (router render),
  // so fetch bindings the first time both are present rather than in
  // connectedCallback (where orgId is still empty).
  override updated(changed: Map<string, unknown>): void {
    if ((changed.has('orgId') || changed.has('client')) && !this._bindingsFetched && this.orgId && this.client) {
      this._bindingsFetched = true;
      void this._fetchBindings();
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._stopPolling();
  }

  private async _fetchBindings(): Promise<void> {
    if (!this.orgId || !this.client) return;
    const res = await this.client.GET('/v1/orgs/{org_id}/bindings' as never, {
      params: { path: { org_id: this.orgId } },
    } as never);
    const { data } = res as { data: { items?: FlowEntryBinding[] } | null };
    const items = (data?.items ?? []).filter((b) => b.active);
    this._bindings = items;
    if (items.length > 0 && !this._selectedBinding) {
      this._selectBinding(`${items[0]!.channel} ${items[0]!.entry_code}`);
    }
  }

  private _selectBinding(key: string): void {
    this._selectedBinding = key;
    const [channel, entry] = key.split(' ');
    this._channel = channel ?? '';
    this._entryCode = entry ?? '';
  }

  private async _createRoute(): Promise<void> {
    if (!this.orgId || !this.client || this._creating) return;
    this._createError = null;
    this._actionError = null;

    let interaction_input: Record<string, unknown> = {};
    if (this._inputJson.trim()) {
      try {
        const parsed: unknown = JSON.parse(this._inputJson);
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          this._createError = 'interaction_input must be a JSON object';
          return;
        }
        interaction_input = parsed as Record<string, unknown>;
      } catch (e) {
        this._createError = `Invalid JSON: ${(e as Error).message}`;
        return;
      }
    }
    if (!this._channel || !this._entryCode) {
      this._createError = 'Pick a channel + entry (publish a flow first if the list is empty)';
      return;
    }

    this._creating = true;
    this._stopPolling();
    this._reservations = [];
    this._trace = null;
    try {
      const res = await this.client.POST('/v1/orgs/{org_id}/route-requests' as never, {
        params: { path: { org_id: this.orgId } },
        body: { channel: this._channel, entry_code: this._entryCode, interaction_input },
      } as never);
      const { data, error } = res as { data: RouteRequest | null; error: unknown };
      if (error || !data) {
        this._createError = (error as { reason?: string })?.reason ?? 'Failed to create route request';
        return;
      }
      this._route = data;
      await this._refresh();
      if (!TERMINAL.has(data.status)) this._startPolling();
    } finally {
      this._creating = false;
    }
  }

  private _startPolling(): void {
    this._stopPolling();
    this._pollTimer = setInterval(() => void this._refresh(), 2000);
  }

  private _stopPolling(): void {
    if (this._pollTimer !== null) {
      clearInterval(this._pollTimer);
      this._pollTimer = null;
    }
  }

  private async _refresh(): Promise<void> {
    if (!this._route || !this.orgId || !this.client) return;
    const id = this._route.id;

    const [routeRes, resvRes] = await Promise.all([
      this.client.GET('/v1/orgs/{org_id}/route-requests/{id}' as never, {
        params: { path: { org_id: this.orgId, id } },
      } as never),
      this.client.GET('/v1/orgs/{org_id}/route-requests/{id}/reservations' as never, {
        params: { path: { org_id: this.orgId, id } },
      } as never),
    ]);

    const route = (routeRes as { data: RouteRequest | null }).data;
    if (route) this._route = route;
    const resv = (resvRes as { data: { items?: Reservation[] } | null }).data;
    this._reservations = resv?.items ?? [];

    if (route?.trace_id) {
      const traceRes = await this.client.GET('/v1/orgs/{org_id}/route-requests/{id}/trace' as never, {
        params: { path: { org_id: this.orgId, id } },
      } as never);
      const trace = (traceRes as { data: Trace | null }).data;
      if (trace) this._trace = trace;
    }

    if (route && TERMINAL.has(route.status)) this._stopPolling();
  }

  private async _resolve(resId: string, action: 'accept' | 'reject' | 'complete'): Promise<void> {
    if (!this.orgId || !this.client || this._busyResId) return;
    this._busyResId = resId;
    this._actionError = null;
    try {
      const res = await this.client.POST(`/v1/orgs/{org_id}/reservations/{id}/${action}` as never, {
        params: { path: { org_id: this.orgId, id: resId } },
      } as never);
      const { error } = res as { error: unknown };
      if (error) {
        this._actionError = (error as { reason?: string })?.reason ?? `${action} failed`;
      }
      await this._refresh();
      if (this._route && !TERMINAL.has(this._route.status) && this._pollTimer === null) {
        this._startPolling();
      }
    } finally {
      this._busyResId = null;
    }
  }

  private _renderCreateCard() {
    const hasBindings = this._bindings.length > 0;
    return html`
      <div class="card">
        <h2 class="card-title"><uk-icon icon="play" width="15" height="15"></uk-icon> New route request</h2>

        ${when(
          this._createError,
          () => html`<div class="alert alert--danger"><uk-icon icon="alert-triangle" width="15" height="15"></uk-icon>${this._createError}</div>`,
        )}

        <div class="field">
          <label class="field-label">Channel / entry (published bindings)</label>
          ${hasBindings
            ? html`<select
                .value=${this._selectedBinding}
                @change=${(e: Event) => this._selectBinding((e.target as HTMLSelectElement).value)}
              >
                ${this._bindings.map((b) => {
                  const key = `${b.channel} ${b.entry_code}`;
                  return html`<option value=${key} ?selected=${key === this._selectedBinding}>
                    ${b.channel} / ${b.entry_code} → ${b.flow_code}
                  </option>`;
                })}
              </select>`
            : html`<div class="field-hint">
                No active bindings — publish a flow first (Flows → open → Publish), then reload.
              </div>`}
        </div>

        ${when(
          !hasBindings,
          () => html`
            <div class="field">
              <label class="field-label">Channel</label>
              <input .value=${this._channel} @input=${(e: Event) => { this._channel = (e.target as HTMLInputElement).value; }} placeholder="voice" />
            </div>
            <div class="field">
              <label class="field-label">Entry code</label>
              <input .value=${this._entryCode} @input=${(e: Event) => { this._entryCode = (e.target as HTMLInputElement).value; }} placeholder="main" />
            </div>
          `,
        )}

        <div class="field">
          <label class="field-label">interaction_input (JSON)</label>
          <textarea
            .value=${this._inputJson}
            @input=${(e: Event) => { this._inputJson = (e.target as HTMLTextAreaElement).value; }}
            spellcheck="false"
          ></textarea>
          <div class="field-hint">Variables the flow can read (e.g. <code>customer.tier</code>).</div>
        </div>

        <button
          type="button"
          class="uk-button uk-button-primary"
          ?disabled=${this._creating}
          @click=${() => void this._createRoute()}
        >
          ${this._creating ? html`<span class="spinner"></span> Creating…` : 'Create route'}
        </button>
      </div>
    `;
  }

  private _renderRouteCard() {
    const r = this._route;
    if (!r) {
      return html`<div class="card"><div class="empty">No route yet — create one to see status, reservations, and the trace.</div></div>`;
    }
    const pill = ROUTE_PILL[r.status] ?? FALLBACK_PILL;
    const polling = this._pollTimer !== null;
    return html`
      <div class="card">
        <h2 class="card-title">
          <uk-icon icon="route" width="15" height="15"></uk-icon> Route request
          <span class="pill" style=${`margin-left:auto;background:${pill.bg};color:${pill.text}`}>${r.status}</span>
        </h2>
        <div class="meta-grid">
          <span class="meta-key">ID</span><span class="meta-val">${r.id}</span>
          <span class="meta-key">Channel / entry</span><span class="meta-val">${r.channel} / ${r.entry_code}</span>
          <span class="meta-key">Flow</span><span class="meta-val">${r.flow_code ?? '—'}</span>
          ${r.failure_code ? html`<span class="meta-key">Failure</span><span class="meta-val">${r.failure_code}</span>` : nothing}
        </div>
        ${polling ? html`<div class="live-chip" style="margin-top:12px"><span class="spinner"></span> live — polling every 2s</div>` : nothing}
      </div>
    `;
  }

  private _renderReservationsCard() {
    if (!this._route) return nothing;
    return html`
      <div class="card">
        <h2 class="card-title"><uk-icon icon="users" width="15" height="15"></uk-icon> Reservations</h2>
        ${when(
          this._actionError,
          () => html`<div class="alert alert--danger"><uk-icon icon="alert-triangle" width="15" height="15"></uk-icon>${this._actionError}</div>`,
        )}
        ${this._reservations.length === 0
          ? html`<div class="empty">No reservations offered yet.</div>`
          : this._reservations.map((res) => this._renderReservationRow(res))}
      </div>
    `;
  }

  private _renderReservationRow(res: Reservation) {
    const pill = RES_PILL[res.state] ?? FALLBACK_PILL;
    const busy = this._busyResId === res.id;
    return html`
      <div class="res-row">
        <span class="pill" style=${`background:${pill.bg};color:${pill.text}`}>${res.state}</span>
        <div class="res-meta">
          <div class="res-agent">${res.agent_id}</div>
          <div class="res-sub">attempt #${res.attempt}${res.reason ? ` · ${res.reason}` : ''}</div>
        </div>
        <div class="res-actions">
          ${res.state === 'offered'
            ? html`
                <button type="button" class="uk-button uk-button-primary uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(res.id, 'accept')}>Accept</button>
                <button type="button" class="uk-button uk-button-default uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(res.id, 'reject')}>Reject</button>
              `
            : nothing}
          ${res.state === 'accepted'
            ? html`<button type="button" class="uk-button uk-button-primary uk-button-small" ?disabled=${busy} @click=${() => void this._resolve(res.id, 'complete')}>Complete</button>`
            : nothing}
          ${busy ? html`<span class="spinner"></span>` : nothing}
        </div>
      </div>
    `;
  }

  private async _submitInput(): Promise<void> {
    if (!this._route || !this.orgId || !this.client || this._submittingInput) return;
    this._submittingInput = true;
    this._actionError = null;
    try {
      const res = await this.client.POST('/v1/orgs/{org_id}/route-requests/{id}/input' as never, {
        params: { path: { org_id: this.orgId, id: this._route.id } },
        body: { value: this._inputValue },
      } as never);
      const { error } = res as { error: unknown };
      if (error) {
        this._actionError = (error as { reason?: string })?.reason ?? 'submit input failed';
      } else {
        this._inputValue = '';
      }
      await this._refresh();
      if (this._route && !TERMINAL.has(this._route.status) && this._pollTimer === null) {
        this._startPolling();
      }
    } finally {
      this._submittingInput = false;
    }
  }

  // A route waiting with no offered reservation is parked at an interactive-input
  // (or wait) node — let the tester answer it.
  private _renderInputCard() {
    const r = this._route;
    if (!r || r.status !== 'waiting') return nothing;
    if (this._reservations.some((res) => res.state === 'offered')) return nothing; // reservation wait → handled above
    return html`
      <div class="card">
        <h2 class="card-title"><uk-icon icon="message-square" width="15" height="15"></uk-icon> Waiting for input</h2>
        <div class="field">
          <label class="field-label">Captured value</label>
          <input
            .value=${this._inputValue}
            placeholder="e.g. 1234 · approved · gold"
            @input=${(e: Event) => { this._inputValue = (e.target as HTMLInputElement).value; }}
            @keydown=${(e: KeyboardEvent) => { if (e.key === 'Enter') void this._submitInput(); }}
          />
          <div class="field-hint">Submitted to the node the route is parked at; it takes the captured branch.</div>
        </div>
        <button type="button" class="uk-button uk-button-primary" ?disabled=${this._submittingInput} @click=${() => void this._submitInput()}>
          ${this._submittingInput ? html`<span class="spinner"></span> Submitting…` : 'Submit input'}
        </button>
      </div>
    `;
  }

  private _renderTraceCard() {
    if (!this._route) return nothing;
    const t = this._trace;
    return html`
      <div class="card">
        <h2 class="card-title">
          <uk-icon icon="list" width="15" height="15"></uk-icon> Trace
          ${t?.outcome ? html`<span style="margin-left:auto;font-size:12px;color:var(--muted-foreground)">outcome: ${t.outcome}</span>` : nothing}
        </h2>
        ${!t || t.steps.length === 0
          ? html`<div class="empty">No trace steps yet.</div>`
          : t.steps.map(
              (s) => html`
                <div class="trace-step">
                  <span class="step-dot" style=${`background:${STEP_DOT[s.status]}`}></span>
                  <span class="step-kind">${s.node_kind}</span>
                  <span class="step-node">${s.node_id}</span>
                  ${s.error ? html`<span class="step-error">${s.error}</span>` : nothing}
                  ${s.port ? html`<span class="step-port">${s.port}</span>` : nothing}
                </div>
              `,
            )}
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <h1 class="page-title">Route Tester</h1>
        <p class="page-subtitle">Drive a published flow live and resolve its reservations — accept, reject, or complete an offer and watch the route settle.</p>
      </div>
      <div class="grid">
        <div>${this._renderCreateCard()}</div>
        <div>
          ${this._renderRouteCard()}
          ${this._renderInputCard()}
          ${this._renderReservationsCard()}
          ${this._renderTraceCard()}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-route-tester': OrRouteTester;
  }
}
