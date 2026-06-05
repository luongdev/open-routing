// v0.3 W6 — <or-ops-view>: a live operations dashboard for the routing engine.
// Polls /routing/stats every 2s for queue depth, outstanding offers, the oldest
// caller's SLA age, and capacity occupancy, alongside a live feed of recent route
// requests. Read-only; mirrors route-tester's poll-while-visible pattern.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

interface RoutingStats {
  waiting_match: number;
  offering: number;
  waiting_offer: number;
  oldest_waiting_seconds: number;
  held_slots: number;
}

type RouteStatus =
  | 'pending' | 'running' | 'waiting' | 'waiting_match' | 'offering' | 'completed' | 'failed' | 'cancelled';

interface RouteRequest {
  id: string;
  channel: string;
  entry_code: string;
  status: RouteStatus;
  created_at: string;
  updated_at?: string;
}

const ROUTE_PILL: Record<RouteStatus, { bg: string; text: string }> = {
  pending: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
  running: { bg: 'color-mix(in oklch, oklch(0.55 0.2 250) 14%, transparent)', text: 'oklch(0.4 0.2 250)' },
  waiting_match: { bg: 'color-mix(in oklch, oklch(0.75 0.18 80) 18%, transparent)', text: 'oklch(0.5 0.18 80)' },
  offering: { bg: 'color-mix(in oklch, oklch(0.7 0.16 300) 18%, transparent)', text: 'oklch(0.46 0.16 300)' },
  waiting: { bg: 'color-mix(in oklch, oklch(0.6 0.16 220) 16%, transparent)', text: 'oklch(0.42 0.16 220)' },
  completed: { bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent)', text: 'oklch(0.45 0.18 145)' },
  failed: { bg: 'color-mix(in oklch, var(--destructive) 12%, transparent)', text: 'var(--destructive)' },
  cancelled: { bg: 'var(--muted)', text: 'var(--muted-foreground)' },
};

function humanAge(seconds: number): string {
  if (seconds <= 0) return '—';
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}m ${s}s`;
}

/**
 * <or-ops-view> — live routing-engine operations snapshot.
 *
 * Attributes: org-id. Properties: client.
 */
@customElement('or-ops-view')
export class OrOpsView extends LitElement {
  static override styles = css`
    :host { display: block; padding: 24px; background: var(--background); min-height: 100%; }
    .page-header { margin-bottom: 20px; display: flex; align-items: baseline; gap: 12px; }
    .page-title { font-size: 24px; font-weight: 700; margin: 0; color: var(--foreground); }
    .live-chip { font-size: 11px; color: var(--muted-foreground); display: inline-flex; align-items: center; gap: 5px; }
    .spinner {
      width: 9px; height: 9px; border-radius: 50%;
      border: 2px solid color-mix(in oklch, oklch(0.65 0.18 145) 40%, transparent);
      border-top-color: oklch(0.65 0.18 145); animation: spin 0.8s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }

    .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 14px; margin-bottom: 22px; }
    .stat {
      background: var(--card); border: 1px solid var(--border); border-radius: 12px;
      padding: 16px 18px; box-shadow: var(--shadow-sm);
    }
    .stat-label {
      font-size: 12px; font-weight: 600; color: var(--muted-foreground);
      display: flex; align-items: center; gap: 6px; margin-bottom: 8px;
    }
    .stat-value { font-size: 30px; font-weight: 700; color: var(--foreground); line-height: 1; }
    .stat-value.warn { color: oklch(0.5 0.18 80); }
    .stat-sub { font-size: 11px; color: var(--muted-foreground); margin-top: 4px; }

    .card {
      background: var(--card); border: 1px solid var(--border); border-radius: 12px;
      padding: 18px 20px; box-shadow: var(--shadow-sm);
    }
    .card-title { font-size: 15px; font-weight: 600; color: var(--foreground); margin: 0 0 14px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { text-align: left; font-size: 11px; font-weight: 600; color: var(--muted-foreground); padding: 0 0 8px; text-transform: uppercase; letter-spacing: 0.03em; }
    td { padding: 9px 0; border-top: 1px solid var(--border); color: var(--foreground); }
    td.mono { font-family: var(--uk-font-monospace, monospace); font-size: 12px; color: var(--muted-foreground); }
    .pill {
      display: inline-flex; align-items: center; padding: 3px 10px; border-radius: 9999px;
      font-size: 12px; font-weight: 600; white-space: nowrap;
    }
    .empty { color: var(--muted-foreground); font-size: 13px; padding: 16px 0; text-align: center; }
    .alert--danger {
      display: flex; align-items: center; gap: 9px; padding: 10px 13px; border-radius: 8px;
      font-size: 13px; margin-bottom: 14px;
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent);
      color: var(--destructive);
    }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object, attribute: false }) accessor client!: ApiClient;

  @state() private accessor _stats: RoutingStats | null = null;
  @state() private accessor _routes: RouteRequest[] = [];
  @state() private accessor _error: string | null = null;

  private _pollTimer: ReturnType<typeof setInterval> | null = null;
  private _started = false;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // orgId + client arrive as reactive properties after the router renders, so kick
  // off the poll the first time both are present (not in connectedCallback).
  override updated(changed: Map<string, unknown>): void {
    if ((changed.has('orgId') || changed.has('client')) && !this._started && this.orgId && this.client) {
      this._started = true;
      void this._refresh();
      this._pollTimer = setInterval(() => void this._refresh(), 2000);
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._pollTimer !== null) {
      clearInterval(this._pollTimer);
      this._pollTimer = null;
    }
  }

  private async _refresh(): Promise<void> {
    if (!this.orgId || !this.client) return;
    const [statsRes, routesRes] = await Promise.all([
      this.client.GET('/v1/orgs/{org_id}/routing/stats' as never, {
        params: { path: { org_id: this.orgId } },
      } as never),
      this.client.GET('/v1/orgs/{org_id}/route-requests' as never, {
        params: { path: { org_id: this.orgId }, query: { limit: 15 } },
      } as never),
    ]);
    const stats = (statsRes as { data: RoutingStats | null }).data;
    if (stats) {
      this._stats = stats;
      this._error = null;
    } else {
      this._error = 'Could not load routing stats — is MATCHER_ENABLED on?';
    }
    const routes = (routesRes as { data: { items?: RouteRequest[] } | null }).data;
    this._routes = routes?.items ?? [];
  }

  private _stat(label: string, icon: string, value: number, sub?: string, warn = false) {
    return html`<div class="stat">
      <div class="stat-label"><uk-icon icon=${icon}></uk-icon>${label}</div>
      <div class="stat-value ${warn ? 'warn' : ''}">${value}</div>
      ${sub ? html`<div class="stat-sub">${sub}</div>` : nothing}
    </div>`;
  }

  override render() {
    const s = this._stats;
    const oldest = s?.oldest_waiting_seconds ?? 0;
    return html`
      <div class="page-header">
        <h1 class="page-title">Live Operations</h1>
        ${this._started
          ? html`<span class="live-chip"><span class="spinner"></span> live — polling every 2s</span>`
          : nothing}
      </div>

      ${this._error ? html`<div class="alert--danger"><uk-icon icon="shield-alert"></uk-icon>${this._error}</div>` : nothing}

      <div class="stats">
        ${this._stat('Queue depth', 'hourglass-split', s?.waiting_match ?? 0, 'waiting for an agent')}
        ${this._stat('Offering', 'phone-forwarded', s?.offering ?? 0, 'matcher building an offer')}
        ${this._stat('Outstanding offers', 'phone-call', s?.waiting_offer ?? 0, 'ringing an agent')}
        ${this._stat('Oldest wait', 'clock', oldest, humanAge(oldest) === '—' ? 'queue empty' : humanAge(oldest), oldest >= 60)}
        ${this._stat('Held slots', 'gauge', s?.held_slots ?? 0, 'capacity in use')}
      </div>

      <div class="card">
        <h2 class="card-title">Recent route requests</h2>
        ${this._routes.length === 0
          ? html`<div class="empty">No route requests yet.</div>`
          : html`<table>
              <thead><tr><th>Route</th><th>Channel</th><th>Entry</th><th>Status</th><th>Created</th></tr></thead>
              <tbody>
                ${this._routes.map((r) => {
                  const p = ROUTE_PILL[r.status] ?? ROUTE_PILL.pending;
                  return html`<tr>
                    <td class="mono">${r.id.slice(0, 8)}</td>
                    <td>${r.channel}</td>
                    <td>${r.entry_code}</td>
                    <td><span class="pill" style=${`background:${p.bg};color:${p.text}`}>${r.status}</span></td>
                    <td class="mono">${new Date(r.created_at).toLocaleTimeString()}</td>
                  </tr>`;
                })}
              </tbody>
            </table>`}
      </div>
    `;
  }
}
