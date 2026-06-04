import { LitElement, html, css, nothing } from 'lit';
import { confirmDelete } from '../primitives/confirm.js';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

import '../primitives/data-table.js';
import '../primitives/cursor-paginator.js';
import type { OrDataTableColumn } from '../primitives/data-table.js';

type Channel = components['schemas']['Channel'];

@customElement('or-channel-list')
export class OrChannelList extends LitElement {
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

    .avatar {
      width: 32px;
      height: 32px;
      border-radius: 8px;
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 600;
      flex-shrink: 0;
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

    .channel-name {
      font-weight: 500;
      color: var(--foreground);
      font-size: 14px;
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .channel-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
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

    .type-pill {
      display: inline-flex;
      padding: 2px 10px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 500;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
      text-transform: capitalize;
    }

    .queue-id-cell {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 12px;
      color: var(--muted-foreground);
    }

    .row-actions {
      display: flex;
      gap: 4px;
      opacity: 0;
      transition: opacity .12s;
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

    .icon-btn--danger:hover {
      color: var(--destructive);
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
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

  private _initials(name: string): string {
    return name.split(/\s+/).slice(0, 2).map(s => s[0]?.toUpperCase() ?? '').join('') || '?';
  }

  private _columns: OrDataTableColumn[] = [
    {
      key: 'name',
      label: 'Name',
      render: (row) => {
        const name = String(row['name'] ?? '');
        const code = String(row['code'] ?? '');
        return html`
          <div class="name-cell">
            <div class="avatar">${this._initials(name)}</div>
            <div class="name-cell-text">
              <span class="channel-name">${name}</span>
              <span class="channel-code">${code}</span>
            </div>
          </div>
        `;
      },
    },
    {
      key: 'channel_type',
      label: 'Type',
      width: '160px',
      render: (row) => {
        const t = String(row['channel_type'] ?? '');
        return t ? html`<span class="type-pill">${t}</span>` : html`<span style="color:var(--muted-foreground);font-size:12px">—</span>`;
      },
    },
    {
      key: 'default_queue_id',
      label: 'Default Queue',
      render: (row) => {
        const queueId = row['default_queue_id'] as string | null | undefined;
        if (!queueId) {
          return html`<span style="color:var(--muted-foreground);font-size:12px">—</span>`;
        }
        const truncated = queueId.slice(0, 8);
        return html`<code class="queue-id-cell" title="${queueId}">${truncated}…</code>`;
      },
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
      this._navigate(`/orgs/${this.orgId}/channels/${row.id}`);
      return;
    }
    if (action === 'disable' || action === 'enable') {
      const body = { enabled: action === 'enable', version: row.version };
      await this.client.PATCH('/v1/orgs/{org_id}/channels/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
        body,
      } as never);
      void this._listTask.run();
      return;
    }
    if (action === 'delete') {
      const ok = await confirmDelete(row.name, 'channel');
      if (!ok) return;
      await this.client.DELETE('/v1/orgs/{org_id}/channels/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
      } as never);
      void this._listTask.run();
    }
  }

  private _listTask = new Task(this, {
    task: async ([orgId, search, cursor, includeDisabled, limit]) => {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/channels' as never, {
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
    const row = e.detail?.row as Channel | undefined;
    if (row?.id) {
      this._navigate(`/orgs/${this.orgId}/channels/${row.id}`);
    }
  }

  private _handlePageChanged(e: CustomEvent): void {
    const { cursor, direction, limit } = e.detail as {
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
      void cursor;
    }
  }

  private _renderEmptyState() {
    if (this._search) {
      return html`
        <div class="empty-state">
          <p>No channels found matching '${this._search}'.</p>
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
        <p>No channels yet. Click <strong>+ Create channel</strong> to add your first.</p>
        <button
          class="uk-button uk-button-primary uk-button-small"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels/new`)}
        >+ Create channel</button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <div class="page-header-left">
          <h1 class="page-title">Channels</h1>
          <p class="page-subtitle">Manage communication channels routed to agents.</p>
        </div>
        <button
          class="uk-button uk-button-primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/channels/new`)}
        >+ Create channel</button>
      </div>

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search channels…"
            .value=${this._search}
            @input=${this._handleSearch}
            aria-label="Search channels"
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
              <strong>Failed to load channels.</strong>
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

      <or-cursor-paginator
        .hasMore=${this._hasMore}
        .cursorStack=${this._cursorStack}
        .limit=${this._limit}
        @or-page-changed=${this._handlePageChanged}
      ></or-cursor-paginator>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-channel-list': OrChannelList;
  }
}
