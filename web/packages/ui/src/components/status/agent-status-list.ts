import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import { type AgentStatus, type AgentStatusResponse } from './status-panel.js';

interface AgentRow {
  id: string;
  name: string;
  code: string;
  email: string;
  enabled: boolean;
}

interface BreakReason {
  id: string;
  name: string;
  routable: boolean;
}

// Status-level colors using oklch design tokens (mirroring STATUS_PILL_STYLES but
// using the Ember variable system rather than sl-* tokens).
const STATUS_STYLES: Record<AgentStatus, { bg: string; text: string; dot: string }> = {
  Ready: {
    bg: 'color-mix(in oklch, oklch(0.65 0.18 145) 14%, transparent)',
    text: 'oklch(0.45 0.18 145)',
    dot: 'oklch(0.60 0.18 145)',
  },
  NotReady: {
    bg: 'color-mix(in oklch, oklch(0.55 0.20 25) 12%, transparent)',
    text: 'oklch(0.45 0.20 25)',
    dot: 'oklch(0.55 0.20 25)',
  },
  Break: {
    bg: 'color-mix(in oklch, oklch(0.72 0.18 75) 14%, transparent)',
    text: 'oklch(0.52 0.18 70)',
    dot: 'oklch(0.68 0.18 75)',
  },
  Engaged: {
    bg: 'color-mix(in oklch, oklch(0.55 0.18 250) 14%, transparent)',
    text: 'oklch(0.42 0.18 250)',
    dot: 'oklch(0.55 0.18 250)',
  },
  WrapUp: {
    bg: 'color-mix(in oklch, oklch(0.72 0.18 75) 10%, transparent)',
    text: 'oklch(0.52 0.18 70)',
    dot: 'oklch(0.68 0.16 75)',
  },
  Offline: {
    bg: 'var(--muted)',
    text: 'var(--muted-foreground)',
    dot: 'var(--muted-foreground)',
  },
};

// Stat card config — covers all 6 AgentStatus values so none are silently dropped
// from the summary row (Codex review concern: NotReady was missing).
const STATE_BUCKETS: Array<{ key: AgentStatus; label: string }> = [
  { key: 'Ready', label: 'Available' },
  { key: 'NotReady', label: 'Not Ready' },
  { key: 'Engaged', label: 'Engaged' },
  { key: 'WrapUp', label: 'Wrap Up' },
  { key: 'Break', label: 'Break' },
  { key: 'Offline', label: 'Offline' },
];

@customElement('or-agent-status-list')
export class OrAgentStatusList extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ────────────────────────────────────────────── */
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 24px;
      gap: 16px;
    }

    .page-title {
      font-size: 24px;
      font-weight: 700;
      margin: 0 0 4px;
      color: var(--foreground);
    }

    .page-subtitle {
      color: var(--muted-foreground);
      font-size: 14px;
      margin: 0;
    }

    .header-actions {
      display: flex;
      align-items: center;
      gap: 8px;
      flex-shrink: 0;
    }

    /* ── Stats row ──────────────────────────────────────────────── */
    .stats-row {
      display: grid;
      grid-template-columns: repeat(7, 1fr);
      gap: 12px;
      margin-bottom: 20px;
    }

    @media (max-width: 1100px) {
      .stats-row { grid-template-columns: repeat(4, 1fr); }
    }

    @media (max-width: 700px) {
      .stats-row { grid-template-columns: repeat(3, 1fr); }
    }

    @media (max-width: 500px) {
      .stats-row { grid-template-columns: repeat(2, 1fr); }
    }

    .stat-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px 16px;
      box-shadow: var(--shadow-xs);
      cursor: pointer;
      transition: box-shadow .12s, border-color .12s;
    }

    .stat-card:hover {
      box-shadow: var(--shadow-sm);
      border-color: color-mix(in oklch, var(--primary) 40%, var(--border));
    }

    .stat-card--active {
      border-color: color-mix(in oklch, var(--primary) 60%, var(--border));
      box-shadow: var(--shadow-sm);
    }

    .stat-card-top {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 6px;
    }

    .stat-label {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .04em;
      color: var(--muted-foreground);
    }

    .stat-pill {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 2px 8px;
      border-radius: 9999px;
      font-size: 10px;
      font-weight: 600;
    }

    .stat-dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      flex-shrink: 0;
    }

    .stat-value {
      font-size: 28px;
      font-weight: 700;
      color: var(--foreground);
      line-height: 1;
    }

    /* ── Filter row ─────────────────────────────────────────────── */
    .filter-row {
      display: flex;
      gap: 12px;
      align-items: center;
      margin-bottom: 16px;
      flex-wrap: wrap;
    }

    .search-wrap {
      position: relative;
      flex: 1;
      max-width: 320px;
    }

    .search-wrap uk-icon {
      position: absolute;
      left: 12px;
      top: 50%;
      transform: translateY(-50%);
      color: var(--muted-foreground);
      pointer-events: none;
    }

    .search-wrap input {
      padding-left: 36px;
      width: 100%;
      box-sizing: border-box;
    }

    .search-clear {
      position: absolute;
      right: 8px;
      top: 50%;
      transform: translateY(-50%);
      background: none;
      border: none;
      cursor: pointer;
      color: var(--muted-foreground);
      padding: 2px;
      display: flex;
      align-items: center;
    }

    .filter-pills {
      display: flex;
      gap: 6px;
      flex-wrap: wrap;
    }

    .filter-pill {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 4px 12px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 500;
      border: 1px solid var(--border);
      background: var(--background);
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background .1s, border-color .1s, color .1s;
    }

    .filter-pill:hover {
      background: var(--muted);
      color: var(--foreground);
    }

    .filter-pill--active {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
      color: var(--primary);
    }

    /* ── Notices ────────────────────────────────────────────────── */
    .notice {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px 14px;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 16px;
    }

    .notice--danger {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 35%, transparent);
      color: var(--destructive);
    }

    .notice--warning {
      background: color-mix(in oklch, oklch(0.72 0.18 75) 12%, transparent);
      border: 1px solid color-mix(in oklch, oklch(0.65 0.18 75) 35%, transparent);
      color: oklch(0.45 0.18 70);
    }

    /* ── Agent grid ─────────────────────────────────────────────── */
    .table-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      overflow: hidden;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
    }

    thead {
      position: sticky;
      top: 0;
      z-index: 10;
      background: var(--card);
    }

    th {
      padding: 10px 14px;
      text-align: left;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .05em;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
      white-space: nowrap;
    }

    tbody tr {
      border-bottom: 1px solid var(--border);
      transition: background .1s;
      cursor: pointer;
    }

    tbody tr:last-child { border-bottom: none; }

    tbody tr:hover { background: var(--muted); }

    td {
      padding: 10px 14px;
      vertical-align: middle;
    }

    /* ── Name cell ──────────────────────────────────────────────── */
    .name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .avatar {
      width: 34px;
      height: 34px;
      border-radius: 50%;
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 600;
      flex-shrink: 0;
    }

    .agent-name {
      font-weight: 500;
      color: var(--foreground);
      font-size: 14px;
      display: block;
    }

    .agent-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
      display: block;
      margin-top: 1px;
    }

    /* ── Status pill ────────────────────────────────────────────── */
    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 4px 12px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 500;
      white-space: nowrap;
    }

    .status-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      flex-shrink: 0;
    }

    /* ── Time-in-state ──────────────────────────────────────────── */
    .time-chip {
      font-size: 12px;
      color: var(--muted-foreground);
      white-space: nowrap;
    }

    /* ── Actions cell ───────────────────────────────────────────── */
    .actions-cell {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 6px;
    }

    .icon-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 5px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: color .12s, background .12s;
    }

    .icon-btn:hover {
      color: var(--foreground);
      background: color-mix(in oklch, var(--foreground) 10%, transparent);
    }

    /* ── Loading shimmer ────────────────────────────────────────── */
    .spinner {
      display: inline-block;
      width: 16px;
      height: 16px;
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
      padding: 40px 16px;
      color: var(--muted-foreground);
      font-size: 14px;
      justify-content: center;
    }

    /* ── Empty state ────────────────────────────────────────────── */
    .empty-state {
      padding: 40px 16px;
      text-align: center;
      color: var(--muted-foreground);
    }

    .empty-state p { margin: 0 0 12px; }

    /* ── Patch error inline ─────────────────────────────────────── */
    .patch-error {
      font-size: 11px;
      color: var(--destructive);
      max-width: 160px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;

  @state() private _agents: AgentRow[] = [];
  @state() private _agentsLoading = true;
  @state() private _loadError: string | null = null;
  @state() private _hasMore = false;
  @state() private _statuses = new Map<string, AgentStatusResponse>();
  @state() private _statusLoading = new Set<string>();
  @state() private _transitioning = new Set<string>();
  @state() private _patchErrors = new Map<string, string>();
  @state() private _breakReasons: BreakReason[] = [];
  @state() private _breakReasonsLoading = false;
  @state() private _breakReasonsError: string | null = null;
  @state() private _search = '';
  @state() private _stateFilter: AgentStatus | null = null;

  private _pollHandle: ReturnType<typeof setInterval> | null = null;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    void this._loadAgents();
    this._pollHandle = setInterval(() => void this._refreshStatuses(), 10_000);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this._pollHandle) {
      clearInterval(this._pollHandle);
      this._pollHandle = null;
    }
  }

  override updated(changedProps: Map<string, unknown>): void {
    super.updated(changedProps);
    if (changedProps.has('orgId')) {
      this._breakReasons = [];
      this._breakReasonsError = null;
    }
  }

  private async _loadAgents(): Promise<void> {
    if (!this.orgId || !this.client) return;
    this._agentsLoading = true;
    this._loadError = null;
    try {
      const result = await this.client.GET('/v1/orgs/{org_id}/agents' as never, {
        params: { path: { org_id: this.orgId }, query: { limit: 100 } },
      } as never);
      const { data, error } = result as {
        data: { items?: AgentRow[]; has_more?: boolean } | null;
        error: unknown;
      };
      if (error || !data) {
        this._loadError = 'Failed to load agents — check network connection.';
        return;
      }
      this._agents = data.items ?? [];
      this._hasMore = data.has_more ?? false;
      await Promise.allSettled([this._refreshStatuses(), this._fetchBreakReasons()]);
    } catch {
      this._loadError = 'Failed to load agents — check network connection.';
    } finally {
      this._agentsLoading = false;
    }
  }

  private async _refreshStatuses(): Promise<void> {
    await Promise.allSettled(this._agents.map(a => this._fetchStatus(a.id)));
  }

  private async _fetchStatus(agentId: string): Promise<void> {
    if (!this.orgId || !this.client) return;
    this._statusLoading = new Set([...this._statusLoading, agentId]);
    try {
      const result = await this.client.GET('/v1/orgs/{org_id}/agents/{id}/status' as never, {
        params: { path: { org_id: this.orgId, id: agentId } },
      } as never);
      const { data, error } = result as { data: AgentStatusResponse | null; error: unknown };
      if (error) return;
      if (data) {
        const existing = this._statuses.get(agentId);
        // Discard poll response if a newer version was already written (patch race guard).
        if (!existing || data.state_version >= existing.state_version) {
          const next = new Map(this._statuses);
          next.set(agentId, data);
          this._statuses = next;
        }
      }
    } finally {
      const next = new Set(this._statusLoading);
      next.delete(agentId);
      this._statusLoading = next;
    }
  }

  private async _fetchBreakReasons(): Promise<void> {
    if (this._breakReasonsLoading || this._breakReasons.length > 0 || !this.orgId || !this.client) return;
    this._breakReasonsLoading = true;
    this._breakReasonsError = null;
    try {
      const result = await this.client.GET('/v1/orgs/{org_id}/break-reasons' as never, {
        params: { path: { org_id: this.orgId }, query: { include_disabled: false, limit: 100 } },
      } as never);
      const { data, error } = result as { data: { items?: BreakReason[] } | null; error: unknown };
      if (error != null) {
        this._breakReasonsError = 'Failed to load break reasons';
        return;
      }
      this._breakReasons = data?.items ?? [];
    } catch {
      this._breakReasonsError = 'Failed to load break reasons';
    } finally {
      this._breakReasonsLoading = false;
    }
  }

  private async _patch(agentId: string, to: AgentStatus, extra?: { force?: boolean; break_reason_id?: string }): Promise<void> {
    if (this._transitioning.has(agentId) || !this.client) return;
    this._transitioning = new Set([...this._transitioning, agentId]);
    const errNext = new Map(this._patchErrors);
    errNext.delete(agentId);
    this._patchErrors = errNext;
    try {
      const result = await this.client.PATCH('/v1/orgs/{org_id}/agents/{id}/status' as never, {
        params: { path: { org_id: this.orgId, id: agentId } },
        body: { to, ...extra },
      } as never);
      const { error } = result as { error: { reason?: string } | null | undefined };
      if (error != null) {
        const next = new Map(this._patchErrors);
        next.set(agentId, error?.reason ?? 'Status change failed');
        this._patchErrors = next;
        return;
      }
      await this._fetchStatus(agentId);
    } finally {
      const next = new Set(this._transitioning);
      next.delete(agentId);
      this._transitioning = next;
    }
  }

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', { detail: { path }, bubbles: true, composed: true })
    );
  }

  private _initials(name: string): string {
    return name.split(/\s+/).slice(0, 2).map(s => s[0]?.toUpperCase() ?? '').join('') || '?';
  }

  private _relativeTime(iso: string): string {
    try {
      const ms = Date.now() - new Date(iso).getTime();
      if (!Number.isFinite(ms)) return iso;
      const seconds = Math.floor(ms / 1000);
      if (seconds < 60) return 'just now';
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return `${minutes}m ago`;
      const hours = Math.floor(minutes / 60);
      if (hours < 24) return `${hours}h ago`;
      return `${Math.floor(hours / 24)}d ago`;
    } catch {
      return iso;
    }
  }

  private _statusLabel(status: AgentStatusResponse): string {
    const base = status.status === 'NotReady' ? 'Not Ready'
      : status.status === 'WrapUp' ? 'Wrap Up'
      : status.status;
    if (status.status === 'Break') {
      const breakName = status.break_reason_name
        ?? this._breakReasons.find(r => r.id === status.break_reason_id)?.name;
      return breakName ?? base;
    }
    return base;
  }

  // ── Stat counts ─────────────────────────────────────────────────

  private _countByStatus(key: AgentStatus): number {
    let n = 0;
    for (const [, s] of this._statuses) {
      if (s.status === key) n++;
    }
    return n;
  }

  // ── Render ───────────────────────────────────────────────────────

  private _renderStatusPill(statusResp: AgentStatusResponse) {
    const s = STATUS_STYLES[statusResp.status] ?? STATUS_STYLES.Offline;
    const label = this._statusLabel(statusResp);
    return html`
      <span class="status-pill" style="background:${s.bg};color:${s.text};">
        <span class="status-dot" style="background:${s.dot};"></span>
        ${label}
      </span>
    `;
  }

  private _renderQuickActions(agent: AgentRow) {
    const status = this._statuses.get(agent.id);
    const busy = this._transitioning.has(agent.id);
    const cur = status?.status;
    const patchErr = this._patchErrors.get(agent.id);

    return html`
      <div class="actions-cell">
        ${patchErr ? html`<span class="patch-error" title="${patchErr}">${patchErr}</span>` : nothing}

        ${cur === 'NotReady' || cur === 'Break' ? html`
          <button
            class="uk-button uk-button-primary uk-button-small"
            ?disabled=${busy}
            @click=${(e: Event) => { e.stopPropagation(); void this._patch(agent.id, 'Ready'); }}
          >Set Ready</button>
        ` : nothing}

        ${cur === 'Ready' ? html`
          <button
            class="uk-button uk-button-default uk-button-small"
            ?disabled=${busy}
            @click=${(e: Event) => { e.stopPropagation(); void this._patch(agent.id, 'NotReady'); }}
          >Not Ready</button>
        ` : nothing}

        ${cur === 'Offline' ? html`
          <button
            class="uk-button uk-button-default uk-button-small"
            ?disabled=${busy}
            @click=${(e: Event) => { e.stopPropagation(); void this._patch(agent.id, 'Ready', { force: true }); }}
          >Force Ready</button>
        ` : nothing}

        <button
          class="icon-btn"
          title="Full status panel"
          @click=${(e: Event) => { e.stopPropagation(); this._navigate(`/orgs/${this.orgId}/agents/${agent.id}/status`); }}
        >
          <uk-icon icon="external-link" height="15" width="15"></uk-icon>
        </button>
      </div>
    `;
  }

  private _renderStatsRow() {
    const total = this._agents.length;
    return html`
      <div class="stats-row">
        ${STATE_BUCKETS.map(({ key, label }) => {
          const count = this._countByStatus(key);
          const s = STATUS_STYLES[key];
          const active = this._stateFilter === key;
          return html`
            <div
              class="stat-card${active ? ' stat-card--active' : ''}"
              role="button"
              tabindex="0"
              @click=${() => { this._stateFilter = active ? null : key; }}
              @keydown=${(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this._stateFilter = active ? null : key; } }}
            >
              <div class="stat-card-top">
                <span class="stat-label">${label}</span>
                <span class="stat-pill" style="background:${s.bg};color:${s.text};">
                  <span class="stat-dot" style="background:${s.dot};"></span>
                </span>
              </div>
              <div class="stat-value">${this._agentsLoading ? '—' : count}</div>
            </div>
          `;
        })}
        <div class="stat-card" style="cursor:default">
          <div class="stat-card-top">
            <span class="stat-label">${this._hasMore ? 'Shown' : 'Total'}</span>
          </div>
          <div class="stat-value">${total}</div>
        </div>
      </div>
    `;
  }

  private _renderTable(rows: AgentRow[]) {
    if (this._agentsLoading) {
      return html`
        <div class="loading-wrap">
          <span class="spinner"></span>
          Loading agents…
        </div>
      `;
    }

    if (rows.length === 0) {
      return html`
        <div class="empty-state">
          <p>${this._search || this._stateFilter ? 'No agents match the current filter.' : 'No agents in this org.'}</p>
          ${this._search || this._stateFilter ? html`
            <button
              class="uk-button uk-button-default uk-button-small"
              @click=${() => { this._search = ''; this._stateFilter = null; }}
            >Clear filters</button>
          ` : nothing}
        </div>
      `;
    }

    return html`
      <table>
        <thead>
          <tr>
            <th>Agent</th>
            <th>Status</th>
            <th>Last change</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          ${rows.map(agent => {
            const status = this._statuses.get(agent.id);
            const isLoadingStatus = this._statusLoading.has(agent.id) && !status;
            return html`
              <tr @click=${() => this._navigate(`/orgs/${this.orgId}/agents/${agent.id}/status`)}>
                <td>
                  <div class="name-cell">
                    <div class="avatar">${this._initials(agent.name)}</div>
                    <div>
                      <span class="agent-name">${agent.name}</span>
                      <span class="agent-code">${agent.code}</span>
                    </div>
                  </div>
                </td>
                <td>
                  ${isLoadingStatus
                    ? html`<span class="spinner"></span>`
                    : status
                      ? this._renderStatusPill(status)
                      : html`<span style="color:var(--muted-foreground);font-size:12px">—</span>`}
                </td>
                <td>
                  ${status
                    ? html`<span class="time-chip" title="${status.updated_at}">${this._relativeTime(status.updated_at)}</span>`
                    : nothing}
                </td>
                <td>${this._renderQuickActions(agent)}</td>
              </tr>
            `;
          })}
        </tbody>
      </table>
    `;
  }

  override render() {
    const q = this._search.toLowerCase();
    let filtered = q
      ? this._agents.filter(a => a.name.toLowerCase().includes(q) || a.code.toLowerCase().includes(q))
      : [...this._agents];

    if (this._stateFilter) {
      filtered = filtered.filter(a => this._statuses.get(a.id)?.status === this._stateFilter);
    }

    return html`
      <div class="page-header">
        <div>
          <h1 class="page-title">Agent Status</h1>
          <p class="page-subtitle">Real-time view of all agents and their current state</p>
        </div>
        <div class="header-actions">
          <button
            class="uk-button uk-button-default uk-button-small"
            ?disabled=${this._agentsLoading}
            @click=${() => void this._refreshStatuses()}
          >
            <uk-icon icon="refresh-cw" height="14" width="14"></uk-icon>
            Refresh
          </button>
        </div>
      </div>

      ${this._renderStatsRow()}

      ${this._loadError ? html`
        <div class="notice notice--danger">
          <uk-icon icon="alert-triangle" width="16" height="16"></uk-icon>
          ${this._loadError}
          <button
            class="uk-button uk-button-default uk-button-small"
            style="margin-left:auto"
            @click=${() => void this._loadAgents()}
          >Retry</button>
        </div>
      ` : nothing}

      ${this._hasMore ? html`
        <div class="notice notice--warning">
          <uk-icon icon="info" width="16" height="16"></uk-icon>
          Showing first 100 agents. Search to narrow results — agents beyond 100 are not shown.
        </div>
      ` : nothing}

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search by name or code…"
            .value=${this._search}
            @input=${(e: Event) => { this._search = (e.target as HTMLInputElement).value; }}
            aria-label="Search agents"
          />
          ${this._search ? html`
            <button class="search-clear" @click=${() => { this._search = ''; }}>
              <uk-icon icon="x" height="14" width="14"></uk-icon>
            </button>
          ` : nothing}
        </div>

        <div class="filter-pills">
          <button
            class="filter-pill${this._stateFilter === null ? ' filter-pill--active' : ''}"
            @click=${() => { this._stateFilter = null; }}
          >All</button>
          ${STATE_BUCKETS.map(({ key, label }) => html`
            <button
              class="filter-pill${this._stateFilter === key ? ' filter-pill--active' : ''}"
              @click=${() => { this._stateFilter = this._stateFilter === key ? null : key; }}
            >${label}</button>
          `)}
        </div>
      </div>

      <div class="table-card">
        ${this._renderTable(filtered)}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-agent-status-list': OrAgentStatusList;
  }
}
