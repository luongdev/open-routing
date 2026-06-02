// vNext preview — Flow list. Entry point for the workflow milestone (v0.2+).
// Not wired to a real API; reads MOCK_FLOWS from playground-mock-data.
// Visual contract: same Ember tokens + table-card frame as v0.1 entity lists.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import { MOCK_FLOWS, type FlowSummary, type FlowStatus } from '../shell/playground-mock-data.js';

import '../primitives/data-table.js';
import type { OrDataTableColumn } from '../primitives/data-table.js';

@customElement('or-flow-list')
export class OrFlowList extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 20px 24px;
    }

    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 16px;
    }

    .page-title {
      font-size: 22px;
      font-weight: 700;
      margin: 0 0 2px;
      color: var(--foreground);
    }

    .page-subtitle {
      color: var(--muted-foreground);
      font-size: 13px;
      margin: 0;
    }

    .vnext-tag {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 2px 8px;
      border-radius: 9999px;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      margin-left: 8px;
      vertical-align: middle;
    }

    .filter-row {
      display: flex;
      gap: 10px;
      align-items: center;
      margin-bottom: 12px;
    }

    .search-wrap {
      position: relative;
      flex: 1;
      max-width: 360px;
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

    .seg {
      display: inline-flex;
      gap: 0;
      border: 1px solid var(--border);
      border-radius: 8px;
      overflow: hidden;
      background: var(--card);
    }
    .seg button {
      background: transparent;
      border: none;
      padding: 7px 12px;
      font-size: 13px;
      font-weight: 500;
      color: var(--muted-foreground);
      cursor: pointer;
      border-right: 1px solid var(--border);
    }
    .seg button:last-child { border-right: none; }
    .seg button:hover { background: var(--muted); color: var(--foreground); }
    .seg button.active {
      background: var(--primary);
      color: var(--primary-foreground);
    }

    .table-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      overflow: hidden;
    }

    .flow-name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 260px;
    }
    .flow-icon {
      width: 32px; height: 32px; border-radius: 8px;
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
      display: inline-flex; align-items: center; justify-content: center;
      flex-shrink: 0;
    }
    .flow-name-text {
      min-width: 0;
      overflow: hidden;
    }
    .flow-name {
      font-weight: 500;
      color: var(--foreground);
      font-size: 14px;
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .flow-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .channel-chip-row {
      display: inline-flex;
      gap: 4px;
    }
    .channel-chip {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 2px 8px;
      border-radius: 4px;
      font-size: 11px;
      background: var(--muted);
      color: var(--muted-foreground);
      border: 1px solid var(--border);
      text-transform: capitalize;
    }

    .version-chip {
      display: inline-flex;
      align-items: center;
      padding: 2px 8px;
      border-radius: 4px;
      font-size: 11px;
      font-family: var(--uk-font-monospace, monospace);
      background: var(--muted);
      color: var(--muted-foreground);
      border: 1px solid var(--border);
    }

    /* .status-pill + variants live in ember-polish.css so they survive the
       Shadow-DOM boundary into data-table cells. Don't redefine here. */
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';

  @state() private _search = '';
  @state() private _statusFilter: 'all' | FlowStatus = 'all';

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  private _relativeTime(iso: string | null): string {
    if (!iso) return '—';
    try {
      const ms = Date.now() - new Date(iso).getTime();
      const seconds = Math.floor(ms / 1000);
      if (seconds < 60) return 'just now';
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return `${minutes}m ago`;
      const hours = Math.floor(minutes / 60);
      if (hours < 24) return `${hours}h ago`;
      const days = Math.floor(hours / 24);
      return `${days}d ago`;
    } catch {
      return iso;
    }
  }

  private _columns: OrDataTableColumn[] = [
    {
      key: 'name',
      label: 'Flow',
      render: (row) => {
        const name = String(row['name'] ?? '');
        const code = String(row['code'] ?? '');
        return html`
          <div class="flow-name-cell">
            <div class="flow-icon">
              <uk-icon icon="git-branch" height="16" width="16"></uk-icon>
            </div>
            <div class="flow-name-text">
              <span class="flow-name">${name}</span>
              <span class="flow-code">${code}</span>
            </div>
          </div>
        `;
      },
    },
    {
      key: 'channel_types',
      label: 'Channels',
      width: '160px',
      render: (row) => {
        const channels = (row['channel_types'] as string[]) ?? [];
        return html`
          <div class="channel-chip-row">
            ${channels.map(c => html`
              <span class="channel-chip">
                <uk-icon
                  icon=${c === 'voice' ? 'phone' : c === 'chat' ? 'message-circle' : 'mail'}
                  height="11" width="11"
                ></uk-icon>
                ${c}
              </span>
            `)}
          </div>
        `;
      },
    },
    {
      key: 'version',
      label: 'Version',
      width: '90px',
      render: (row) => html`<span class="version-chip">v${row['version']}</span>`,
    },
    {
      key: 'status',
      label: 'Status',
      width: '120px',
      render: (row) => {
        const s = String(row['status'] ?? '') as FlowStatus;
        return html`<span class="status-pill status-pill--${s}">${s}</span>`;
      },
    },
    {
      key: 'last_simulated_at',
      label: 'Last simulated',
      width: '140px',
      render: (row) => {
        const iso = (row['last_simulated_at'] as string | null) ?? null;
        return html`<span title="${iso ?? ''}" style="font-size:13px;color:var(--muted-foreground)">${this._relativeTime(iso)}</span>`;
      },
    },
    {
      key: 'last_published_at',
      label: 'Last published',
      width: '140px',
      render: (row) => {
        const iso = (row['last_published_at'] as string | null) ?? null;
        return html`<span title="${iso ?? ''}" style="font-size:13px;color:var(--muted-foreground)">${this._relativeTime(iso)}</span>`;
      },
    },
  ];

  private _filteredRows(): FlowSummary[] {
    const q = this._search.trim().toLowerCase();
    return MOCK_FLOWS.filter(f => {
      if (this._statusFilter !== 'all' && f.status !== this._statusFilter) return false;
      if (!q) return true;
      return f.name.toLowerCase().includes(q) || f.code.toLowerCase().includes(q);
    });
  }

  private _navigate(path: string): void {
    this.dispatchEvent(new CustomEvent('open-routing:navigate', {
      detail: { path }, bubbles: true, composed: true,
    }));
  }

  override render() {
    const rows = this._filteredRows() as unknown as Record<string, unknown>[];
    const counts = {
      all: MOCK_FLOWS.length,
      draft: MOCK_FLOWS.filter(f => f.status === 'draft').length,
      published: MOCK_FLOWS.filter(f => f.status === 'published').length,
      archived: MOCK_FLOWS.filter(f => f.status === 'archived').length,
    };

    return html`
      <div class="page-header">
        <div>
          <h1 class="page-title">
            Flows
            <span class="vnext-tag">vNext</span>
          </h1>
          <p class="page-subtitle">Author routing logic visually. Validate, simulate, publish.</p>
        </div>
        <button
          class="uk-button uk-button-primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/flows/new`)}
        >+ New Flow</button>
      </div>

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search flows…"
            .value=${this._search}
            @input=${(e: Event) => { this._search = (e.target as HTMLInputElement).value; }}
            aria-label="Search flows"
          />
        </div>
        <div class="seg" role="tablist">
          <button
            class=${this._statusFilter === 'all' ? 'active' : ''}
            @click=${() => { this._statusFilter = 'all'; }}
          >All (${counts.all})</button>
          <button
            class=${this._statusFilter === 'draft' ? 'active' : ''}
            @click=${() => { this._statusFilter = 'draft'; }}
          >Draft (${counts.draft})</button>
          <button
            class=${this._statusFilter === 'published' ? 'active' : ''}
            @click=${() => { this._statusFilter = 'published'; }}
          >Published (${counts.published})</button>
          <button
            class=${this._statusFilter === 'archived' ? 'active' : ''}
            @click=${() => { this._statusFilter = 'archived'; }}
          >Archived (${counts.archived})</button>
        </div>
      </div>

      <div class="table-card">
        <or-data-table
          .columns=${this._columns}
          .rows=${rows}
          @or-row-click=${(e: CustomEvent) => {
            const row = e.detail?.row as FlowSummary | undefined;
            if (row?.id) this._navigate(`/orgs/${this.orgId}/flows/${row.id}`);
          }}
        ></or-data-table>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-flow-list': OrFlowList;
  }
}
