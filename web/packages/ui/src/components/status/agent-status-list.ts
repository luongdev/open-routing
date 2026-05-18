import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import type { ApiClient } from '../../api/client.js';
import { STATUS_PILL_STYLES, type AgentStatus, type AgentStatusResponse } from './status-panel.js';

import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/dropdown/dropdown.js';
import '@shoelace-style/shoelace/dist/components/menu/menu.js';
import '@shoelace-style/shoelace/dist/components/menu-item/menu-item.js';
import '@shoelace-style/shoelace/dist/components/divider/divider.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';

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

@customElement('or-agent-status-list')
export class OrAgentStatusList extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
    }

    .page-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 20px;
    }

    h1 {
      font-size: 20px;
      font-weight: 600;
      margin: 0;
      flex: 1;
    }

    .search-input {
      width: 260px;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
      color: var(--or-color-text-body, #404040);
    }

    thead {
      position: sticky;
      top: 0;
      z-index: 10;
      background: var(--or-color-sidebar-bg, #f5f5f5);
    }

    th {
      padding: 10px 12px;
      text-align: left;
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--or-color-text-muted, #737373);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      white-space: nowrap;
    }

    tbody tr {
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    tbody tr:last-child { border-bottom: none; }

    td {
      padding: 10px 12px;
      vertical-align: middle;
    }

    .agent-name {
      font-weight: 500;
      color: var(--or-color-text-strong, #171717);
    }

    .agent-code {
      font-family: var(--or-font-mono, monospace);
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 2px;
    }

    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 3px 10px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 500;
      white-space: nowrap;
    }


    .actions-cell {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .loading-cell {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      color: var(--or-color-text-muted, #737373);
      font-size: 12px;
    }

    .empty-state {
      text-align: center;
      padding: 40px;
      color: var(--or-color-text-muted, #737373);
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

  private _pollHandle: ReturnType<typeof setInterval> | null = null;

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
        // Discard poll response if a newer version already written (patch race guard)
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

  private _renderStatusPill(statusResp: AgentStatusResponse) {
    const s = STATUS_PILL_STYLES[statusResp.status] ?? STATUS_PILL_STYLES.Offline;
    const label = statusResp.status === 'NotReady' ? 'Not Ready'
      : statusResp.status === 'WrapUp' ? 'Wrap Up'
      : statusResp.status;
    const breakName = statusResp.break_reason_name
      ?? (statusResp.break_reason_id
        ? this._breakReasons.find(r => r.id === statusResp.break_reason_id)?.name
        : undefined);
    const pillLabel = statusResp.status === 'Break' && breakName ? breakName : label;
    return html`
      <span class="status-pill" style="background:${s.bg};color:${s.text};">
        <sl-icon name="${s.icon}" style="font-size:10px"></sl-icon>
        ${pillLabel}
      </span>
    `;
  }

  private _renderActions(agent: AgentRow) {
    const status = this._statuses.get(agent.id);
    const busy = this._transitioning.has(agent.id);
    const initialLoad = this._statusLoading.has(agent.id) && !status;
    const patchErr = this._patchErrors.get(agent.id);

    if (initialLoad) {
      return html`<span class="loading-cell"><sl-spinner style="font-size:13px"></sl-spinner></span>`;
    }

    const cur = status?.status;

    return html`
      <div class="actions-cell">
        ${patchErr ? html`<span style="color:var(--sl-color-danger-600);font-size:12px;max-width:180px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${patchErr}</span>` : nothing}

        ${cur === 'NotReady' || cur === 'Break' ? html`
          <sl-button size="small" variant="primary" ?disabled=${busy}
            @click=${() => void this._patch(agent.id, 'Ready')}>
            <sl-icon slot="prefix" name="check-circle"></sl-icon>
            Set Ready
          </sl-button>
        ` : nothing}

        ${cur === 'Ready' ? html`
          <sl-button size="small" variant="default" ?disabled=${busy}
            @click=${() => void this._patch(agent.id, 'NotReady')}>
            Set Not Ready
          </sl-button>
          <sl-dropdown @sl-show=${() => void this._fetchBreakReasons()}>
            <sl-button slot="trigger" size="small" variant="default" ?disabled=${busy}>
              Go on Break
            </sl-button>
            <sl-menu style="--sl-font-size-medium:13px">
              ${this._breakReasonsLoading ? html`
                <sl-menu-item disabled>
                  <sl-spinner slot="prefix" style="font-size:12px"></sl-spinner>
                  Loading…
                </sl-menu-item>
              ` : this._breakReasonsError ? html`
                <sl-menu-item disabled>${this._breakReasonsError}</sl-menu-item>
              ` : this._breakReasons.map(r => html`
                <sl-menu-item @click=${() => void this._patch(agent.id, 'Break', { break_reason_id: r.id })}>
                  <sl-icon slot="prefix" name="pause-circle"
                    style="color:var(--sl-color-warning-600,#b54708)"></sl-icon>
                  ${r.name}
                </sl-menu-item>
              `)}
            </sl-menu>
          </sl-dropdown>
        ` : nothing}

        ${cur === 'Break' ? html`
          <sl-button size="small" variant="default" ?disabled=${busy}
            @click=${() => void this._patch(agent.id, 'NotReady')}>
            Set Not Ready
          </sl-button>
        ` : nothing}

        ${cur === 'Offline' ? html`
          <sl-button size="small" variant="default" ?disabled=${busy}
            @click=${() => void this._patch(agent.id, 'Ready', { force: true })}>
            Force Ready
          </sl-button>
        ` : nothing}

        <sl-dropdown>
          <sl-icon-button slot="trigger" name="three-dots-vertical" label="More"></sl-icon-button>
          <sl-menu>
            <sl-menu-item @click=${() => this._navigate(`/orgs/${this.orgId}/agents/${agent.id}/status`)}>
              <sl-icon slot="prefix" name="activity"></sl-icon>
              Full status panel
            </sl-menu-item>
            <sl-divider></sl-divider>
            <sl-menu-item @click=${() => this._navigate(`/orgs/${this.orgId}/agents/${agent.id}`)}>
              <sl-icon slot="prefix" name="person"></sl-icon>
              Agent detail
            </sl-menu-item>
          </sl-menu>
        </sl-dropdown>
      </div>
    `;
  }

  override render() {
    const q = this._search.toLowerCase();
    const filtered = q
      ? this._agents.filter(a => a.name.toLowerCase().includes(q) || a.code.toLowerCase().includes(q))
      : this._agents;

    return html`
      <div class="page-header">
        <h1>Agent Status</h1>
        <sl-input
          class="search-input"
          placeholder="Search by name or code…"
          clearable
          @sl-input=${(e: Event) => { this._search = (e.target as HTMLInputElement).value; }}
        >
          <sl-icon name="search" slot="prefix"></sl-icon>
        </sl-input>
        <sl-icon-button
          name="arrow-clockwise"
          label="Refresh all"
          ?disabled=${this._agentsLoading}
          @click=${() => void this._refreshStatuses()}
        ></sl-icon-button>
      </div>

      ${this._loadError ? html`
        <sl-alert variant="danger" open style="margin-bottom:16px">
          <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
          ${this._loadError}
          <sl-button size="small" slot="footer" @click=${() => void this._loadAgents()}>Retry</sl-button>
        </sl-alert>
      ` : nothing}

      ${this._hasMore ? html`
        <sl-alert variant="warning" open style="margin-bottom:16px">
          <sl-icon slot="icon" name="exclamation-triangle"></sl-icon>
          Showing first 100 agents only. Search filters this list — agents beyond the first 100 are not reachable here.
        </sl-alert>
      ` : nothing}

      ${this._agentsLoading
        ? html`
          <div style="display:flex;align-items:center;gap:12px;color:var(--or-color-text-muted)">
            <sl-spinner></sl-spinner> Loading agents…
          </div>
        `
        : html`
          <table>
            <thead>
              <tr>
                <th>Name / Code</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${filtered.length === 0
                ? html`
                  <tr><td colspan="3" class="empty-state">No agents found.</td></tr>
                `
                : filtered.map(agent => {
                  const status = this._statuses.get(agent.id);
                  const isLoadingStatus = this._statusLoading.has(agent.id) && !status;
                  return html`
                    <tr>
                      <td>
                        <div class="agent-name">${agent.name}</div>
                        <div class="agent-code">${agent.code}</div>
                      </td>
                      <td>
                        ${isLoadingStatus
                          ? html`<sl-spinner style="font-size:13px"></sl-spinner>`
                          : status ? this._renderStatusPill(status) : nothing}
                      </td>
                      <td>${this._renderActions(agent)}</td>
                    </tr>
                  `;
                })}
            </tbody>
          </table>
        `}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-agent-status-list': OrAgentStatusList;
  }
}
