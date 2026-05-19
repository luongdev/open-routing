import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

import '../primitives/data-table.js';
import '../primitives/cursor-paginator.js';
import type { OrDataTableColumn } from '../primitives/data-table.js';

type BreakReason = components['schemas']['BreakReason'];

@customElement('or-break-reason-list')
export class OrBreakReasonList extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
    }

    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 24px;
    }

    .page-header-left {}

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

    .name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .break-name {
      font-weight: 500;
      color: var(--foreground);
      font-size: 14px;
      display: block;
    }

    .break-code {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--muted-foreground);
      display: block;
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

    .routable-pill {
      display: inline-flex;
      padding: 2px 10px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 500;
      background: color-mix(in oklch, oklch(0.65 0.18 210) 15%, transparent);
      color: oklch(0.40 0.18 210);
    }

    .routable-no {
      background: var(--muted);
      color: var(--muted-foreground);
    }

    .order-cell {
      text-align: right;
      display: block;
      font-size: 13px;
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

  // D6-V-10: routable + display_order columns; routable label is non-string for tooltip compat
  _columns: OrDataTableColumn[] = [
    {
      key: 'name',
      label: 'Name',
      render: (row) => {
        const name = String(row['name'] ?? '');
        const code = String(row['code'] ?? '');
        return html`
          <div class="name-cell">
            <div class="name-cell-text">
              <span class="break-name">${name}</span>
              <span class="break-code">${code}</span>
            </div>
          </div>
        `;
      },
    },
    {
      key: 'routable',
      label: html`<span title="Routable: agent can still receive interactions while on break">Routable</span>`,
      render: (row) =>
        row['routable']
          ? html`<span class="status-pill routable-pill">
              <uk-icon icon="check" height="12" width="12" style="margin-right:4px"></uk-icon>Yes
            </span>`
          : html`<span class="status-pill status-pill--muted routable-no">No</span>`,
    },
    {
      key: 'display_order',
      label: 'Order',
      render: (row) =>
        html`<span class="order-cell">${String(row['display_order'] ?? '')}</span>`,
    },
    {
      key: 'enabled',
      label: 'Status',
      render: (row) =>
        row['enabled']
          ? html`<span class="status-pill status-pill--success">Active</span>`
          : html`<span class="status-pill status-pill--muted">Disabled</span>`,
    },
    {
      key: 'updated_at',
      label: 'Updated',
      render: (row) => {
        const iso = String(row['updated_at'] ?? '');
        return html`<span title="${iso}" style="font-size:13px;color:var(--muted-foreground)">${this._relativeTime(iso)}</span>`;
      },
    },
    {
      key: '_actions',
      label: '',
      render: (row) => html`
        <div class="row-actions">
          <button
            type="button"
            class="icon-btn"
            title="Edit"
            @click=${(e: Event) => { e.stopPropagation(); this._navigate(`/orgs/${this.orgId}/break-reasons/${String(row['id'])}`); }}
          >
            <uk-icon icon="pencil" height="16" width="16"></uk-icon>
          </button>
        </div>
      `,
    },
  ];

  private _listTask = new Task(this, {
    task: async ([orgId, search, cursor, includeDisabled, limit]) => {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/break-reasons', {
        params: {
          path: { org_id: orgId as string },
          query: {
            name: (search as string) || undefined,
            cursor: (cursor as string | null) ?? undefined,
            include_disabled: includeDisabled as boolean,
            limit: limit as number,
          },
        },
      });
      if (error) throw error;
      this._hasMore = data?.has_more ?? false;
      this._nextCursor = (data as { next_cursor?: string | null })?.next_cursor ?? null;
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

  private _navigate(path: string): void {
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
    const row = e.detail?.row as BreakReason | undefined;
    if (row?.id) {
      this._navigate(`/orgs/${this.orgId}/break-reasons/${row.id}`);
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
          <p>No break reasons found matching '${this._search}'.</p>
          <button
            type="button"
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
        <p><strong>No break reasons yet</strong></p>
        <p>Define why agents go on break. Each reason shows in the break picker.</p>
        <button
          type="button"
          class="uk-button uk-button-primary uk-button-small"
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons/new`)}
        >+ Create break reason</button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <div class="page-header-left">
          <h1 class="page-title">Break Reasons</h1>
          <p class="page-subtitle">Manage break categories agents can select when off-queue.</p>
        </div>
        <button
          type="button"
          class="uk-button uk-button-primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons/new`)}
        >+ Create break reason</button>
      </div>

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search break reasons…"
            .value=${this._search}
            @input=${this._handleSearch}
            aria-label="Search break reasons"
          />
          ${this._search ? html`
            <button type="button" class="search-clear" @click=${() => { this._search = ''; this._cursor = null; this._cursorStack = []; }}>
              <uk-icon icon="x" height="14" width="14"></uk-icon>
            </button>
          ` : ''}
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
        <button type="button" class="icon-btn" title="Refresh" @click=${this._handleRefresh}>
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
            const rows = (data?.items ?? []) as Record<string, unknown>[];
            return html`
              ${when(
                rows.length === 0,
                () => this._renderEmptyState(),
                () => html`
                  <or-data-table
                    .columns=${this._columns}
                    .rows=${rows}
                    @or-row-click=${this._handleRowClick}
                  ></or-data-table>
                  <or-cursor-paginator
                    .hasMore=${this._hasMore}
                    .cursorStack=${this._cursorStack}
                    .limit=${this._limit}
                    @or-page-changed=${this._handlePageChanged}
                  ></or-cursor-paginator>
                `
              )}
            `;
          },
          error: (err) => html`
            <div class="uk-alert uk-alert-danger" style="margin:16px;border-radius:8px">
              <strong>Failed to load break reasons.</strong>
              ${(err as { reason?: string })?.reason ?? String(err)}
              <button
                type="button"
                class="uk-button uk-button-default uk-button-small"
                style="margin-top:8px;display:block"
                @click=${this._handleRefresh}
              >Retry</button>
            </div>
          `,
        })}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-break-reason-list': OrBreakReasonList;
  }
}
