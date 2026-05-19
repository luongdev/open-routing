import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

import '../primitives/data-table.js';
import '../primitives/cursor-paginator.js';
import type { OrDataTableColumn } from '../primitives/data-table.js';

type Queue = components['schemas']['Queue'];

@customElement('or-queue-list')
export class OrQueueList extends LitElement {
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

    .page-header-left {}

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

    .filter-row {
      display: flex;
      gap: 10px;
      align-items: center;
      margin-bottom: 12px;
      flex-wrap: wrap;
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

    .table-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      overflow: hidden;
    }

    .status-pill {
      display: inline-flex;
      padding: 2px 10px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 500;
    }

    .status-pill--success {
      background: color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent);
      color: oklch(0.45 0.18 145);
    }

    .status-pill--muted {
      background: var(--muted);
      color: var(--muted-foreground);
    }

    .channel-tag {
      display: inline-flex;
      padding: 1px 8px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 500;
      background: var(--muted);
      color: var(--muted-foreground);
      margin-right: 4px;
    }

    .channel-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    .name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 220px;
    }

    .name-cell-text {
      min-width: 0;
      overflow: hidden;
    }

    .queue-name {
      font-weight: 500;
      color: var(--foreground);
      font-size: 14px;
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .priority-cell {
      text-align: right;
      font-variant-numeric: tabular-nums;
      font-size: 13px;
    }

    .acw-cell {
      font-variant-numeric: tabular-nums;
      font-size: 13px;
      color: var(--muted-foreground);
    }

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

    .empty-state {
      padding: 40px 16px;
      text-align: center;
      color: var(--muted-foreground);
    }

    .empty-state p {
      margin: 0 0 12px;
    }

    .include-disabled-label {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 14px;
      color: var(--muted-foreground);
      cursor: pointer;
      white-space: nowrap;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  @state() private accessor _search = '';
  @state() private accessor _cursor: string | null = null;
  @state() private accessor _cursorStack: string[] = [];
  @state() private accessor _includeDisabled = false;
  @state() private accessor _limit = 25;
  @state() private accessor _nextCursor: string | null = null;
  @state() private accessor _hasMore = false;

  private _searchDebounce?: ReturnType<typeof setTimeout>;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  _columns: OrDataTableColumn[] = [
    {
      key: 'code',
      label: 'Code',
      render: (row) =>
        html`<code style="font-family:var(--uk-font-monospace,monospace);font-size:12px;color:var(--muted-foreground)">${String(row['code'] ?? '')}</code>`,
    },
    {
      key: 'name',
      label: 'Name',
      render: (row) => html`
        <div class="name-cell">
          <div class="name-cell-text">
            <span class="queue-name">${String(row['name'] ?? '')}</span>
          </div>
        </div>
      `,
    },
    {
      key: 'channel_types',
      label: 'Channels',
      width: '160px',
      render: (row) => {
        const types = (row['channel_types'] as string[]) ?? [];
        return html`
          <div class="channel-tags">
            ${types.map((t) => html`<span class="channel-tag">${t}</span>`)}
          </div>
        `;
      },
    },
    {
      key: 'priority',
      label: 'Priority',
      render: (row) =>
        html`<span class="priority-cell" style="display:block;text-align:right">${String(row['priority'] ?? '')}</span>`,
    },
    {
      key: 'acw_sec',
      label: 'ACW',
      render: (row) =>
        html`<span class="acw-cell">${String(row['acw_sec'] ?? '')}s</span>`,
    },
    {
      key: 'enabled',
      label: 'Status',
      width: '100px',
      render: (row) =>
        row['enabled']
          ? html`<span class="status-pill status-pill--success">Active</span>`
          : html`<span class="status-pill status-pill--muted">Disabled</span>`,
    },
    {
      key: 'updated_at',
      label: 'Updated',
      width: '130px',
      render: (row) => {
        const iso = String(row['updated_at'] ?? '');
        return html`<span title="${iso}" style="font-size:13px;color:var(--muted-foreground)">${this._relativeTime(iso)}</span>`;
      },
    },
  ];

  private async _handleRowAction(e: CustomEvent): Promise<void> {
    const { row, action } = e.detail as { row: { id: string; name: string; version: number }; action: string };
    if (!row?.id) return;
    if (action === 'edit') {
      this._navigate(`/orgs/${this.orgId}/queues/${row.id}`);
      return;
    }
    if (action === 'disable' || action === 'enable') {
      const body = { enabled: action === 'enable', version: row.version };
      await this.client.PATCH('/v1/orgs/{org_id}/queues/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
        body,
      } as never);
      void this._listTask.run();
      return;
    }
    if (action === 'delete') {
      const ok = window.confirm(`Delete queue "${row.name}"? This cannot be undone.`);
      if (!ok) return;
      await this.client.DELETE('/v1/orgs/{org_id}/queues/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
      } as never);
      void this._listTask.run();
    }
  }

  private _listTask = new Task(this, {
    task: async ([orgId, search, cursor, includeDisabled, limit]) => {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/queues' as never, {
        params: {
          path: { org_id: orgId as string },
          query: {
            name: (search as string) || undefined,
            cursor: (cursor as string | null) ?? undefined,
            include_disabled: includeDisabled as boolean,
            limit: limit as number,
          },
        },
      } as never);
      if (error) throw error;
      const d = data as { has_more?: boolean; next_cursor?: string | null };
      this._hasMore = d?.has_more ?? false;
      this._nextCursor = d?.next_cursor ?? null;
      return data;
    },
    args: () =>
      [this.orgId, this._search, this._cursor, this._includeDisabled, this._limit] as const,
  });

  private _relativeTime(iso: string): string {
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

  _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleSearch(e: Event): void {
    const val = (e.target as HTMLInputElement).value;
    clearTimeout(this._searchDebounce);
    this._searchDebounce = setTimeout(() => {
      this._search = val;
      this._cursor = null;
      this._cursorStack = [];
    }, 300);
  }

  private _handleIncludeDisabledChange(e: Event): void {
    this._includeDisabled = (e.target as HTMLInputElement).checked;
    this._cursor = null;
    this._cursorStack = [];
  }

  private _handleRefresh(): void {
    void this._listTask.run();
  }

  private _handleRowClick(e: CustomEvent): void {
    const row = e.detail?.row as Queue | undefined;
    if (row?.id) {
      this._navigate(`/orgs/${this.orgId}/queues/${row.id}`);
    }
  }

  private _handlePageChanged(e: CustomEvent): void {
    const { direction, limit } = e.detail as {
      cursor: string | null;
      direction: 'next' | 'prev';
      limit: number;
    };

    this._limit = limit;

    if (direction === 'next') {
      if (this._cursor !== null) {
        this._cursorStack = [...this._cursorStack, this._cursor];
      }
      this._cursor = this._nextCursor;
    } else {
      const newStack = [...this._cursorStack];
      const prevCursor = newStack.pop() ?? null;
      this._cursorStack = newStack;
      this._cursor = prevCursor;
    }
  }

  private _renderEmptyState() {
    if (this._search) {
      return html`
        <div class="empty-state">
          <p>No queues found matching '${this._search}'.</p>
          <button
            class="uk-button uk-button-default uk-button-small"
            @click=${() => {
              this._search = '';
              this._cursor = null;
              this._cursorStack = [];
            }}
          >Clear search</button>
        </div>
      `;
    }
    return html`
      <div class="empty-state">
        <p>No queues yet. Click <strong>+ Create queue</strong> to add your first.</p>
        <button
          class="uk-button uk-button-primary uk-button-small"
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues/new`)}
        >+ Create queue</button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <div class="page-header-left">
          <h1 class="page-title">Queues</h1>
          <p class="page-subtitle">Manage routing queues that distribute work to agents.</p>
        </div>
        <button
          class="uk-button uk-button-primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/queues/new`)}
        >+ Create queue</button>
      </div>

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search queues…"
            .value=${this._search}
            @input=${this._handleSearch}
            aria-label="Search queues"
          />
          ${this._search ? html`
            <button class="search-clear" @click=${() => { this._search = ''; this._cursor = null; this._cursorStack = []; }}>
              <uk-icon icon="x" height="14" width="14"></uk-icon>
            </button>
          ` : nothing}
        </div>
        <label class="include-disabled-label">
          <input
            type="checkbox"
            class="uk-checkbox"
            .checked=${this._includeDisabled}
            @change=${this._handleIncludeDisabledChange}
          />
          Include disabled
        </label>
        <button class="icon-btn" title="Refresh" @click=${this._handleRefresh}>
          <uk-icon icon="refresh-cw" height="18" width="18"></uk-icon>
        </button>
      </div>

      <div class="table-card">
        ${this._listTask.render({
          pending: () => html`
            <or-data-table
              .columns=${this._columns}
              .rows=${[]}
              .loading=${true}
            ></or-data-table>
          `,
          complete: (data) => {
            const rows = ((data as { items?: unknown[] })?.items ?? []) as Record<string, unknown>[];
            return html`
              ${when(
                rows.length === 0,
                () => this._renderEmptyState(),
                () => html`
                  <or-data-table
                    .columns=${this._columns}
                    .rows=${rows}
                    @or-row-click=${this._handleRowClick}
                    @or-row-action=${this._handleRowAction}
                  ></or-data-table>
                `
              )}
            `;
          },
          error: (err) => html`
            <div class="uk-alert uk-alert-danger" style="margin:16px;border-radius:8px">
              <strong>Failed to load queues.</strong>
              ${(err as { reason?: string })?.reason ?? String(err)}
              <button
                class="uk-button uk-button-default uk-button-small"
                style="margin-top:8px;display:block"
                @click=${this._handleRefresh}
              >Retry</button>
            </div>
          `,
        })}
      </div>

      ${when(
        this._hasMore || this._cursorStack.length > 0,
        () => html`
          <or-cursor-paginator
            .hasMore=${this._hasMore}
            .cursorStack=${this._cursorStack}
            .limit=${this._limit}
            @or-page-changed=${this._handlePageChanged}
          ></or-cursor-paginator>
        `
      )}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-list': OrQueueList;
  }
}
