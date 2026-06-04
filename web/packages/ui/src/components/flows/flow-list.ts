// v0.2 Layer 2 — API-backed flow list (FLOW). Mirrors the v0.1 entity-list
// pattern (queue-list.ts): org-id + client props, @lit/task fetch against the
// typed client, data-table render with pending/complete/error, search +
// include-disabled + cursor paginator, row actions (edit/disable/delete).
//
// The vNext playground `or-flow-list` stays a mock fixture; this is the real
// product element bound to GET /v1/orgs/{org_id}/flows.
//
// Status columns (review choice "b" — show the full table so layout/wiring bugs
// surface): "Status" reflects the draft lifecycle (Draft / Archived from
// `enabled`); "Published" derives from active bindings, which are a 501 stub
// until Layer 3, so it renders "—" with a pending hint rather than a fake state.

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

type Flow = components['schemas']['Flow'];

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

    .status-pill--draft {
      background: color-mix(in oklch, var(--primary) 14%, transparent);
      color: var(--primary);
    }

    .status-pill--muted {
      background: var(--muted);
      color: var(--muted-foreground);
    }

    .pending-dash {
      color: var(--muted-foreground);
      cursor: help;
    }

    .name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 220px;
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

    .version-cell {
      text-align: right;
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
  @state() private accessor _cursorStack: (string | null)[] = [];
  @state() private accessor _includeDisabled = false;
  @state() private accessor _limit = 25;
  @state() private accessor _nextCursor: string | null = null;
  @state() private accessor _hasMore = false;
  @state() private accessor _actionError: string | null = null;

  private _searchDebounce?: ReturnType<typeof setTimeout>;

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    clearTimeout(this._searchDebounce);
  }

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
          <div style="min-width:0;overflow:hidden">
            <span class="flow-name">${String(row['name'] ?? '')}</span>
          </div>
        </div>
      `,
    },
    {
      key: 'enabled',
      label: 'Status',
      width: '110px',
      render: (row) =>
        row['enabled']
          ? html`<span class="status-pill status-pill--draft">Draft</span>`
          : html`<span class="status-pill status-pill--muted">Archived</span>`,
    },
    {
      key: 'published',
      label: 'Published',
      width: '110px',
      render: () =>
        // Derived from active bindings (GET /bindings) — wired in Layer 3.
        html`<span class="pending-dash" title="Published state derives from route bindings — available in Layer 3.">—</span>`,
    },
    {
      key: 'version',
      label: 'Rev',
      width: '70px',
      render: (row) =>
        html`<span class="version-cell" style="display:block">v${String(row['version'] ?? '')}</span>`,
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

  private _listTask = new Task(this, {
    task: async ([client, orgId, search, cursor, includeDisabled, limit], { signal }) => {
      const { data, error } = await (client as ApiClient).GET('/v1/orgs/{org_id}/flows' as never, {
        params: {
          path: { org_id: orgId as string },
          query: {
            name: (search as string) || undefined,
            cursor: (cursor as string | null) ?? undefined,
            include_disabled: includeDisabled as boolean,
            limit: limit as number,
          },
        },
        signal,
      } as never);
      if (error) throw error;
      if (signal.aborted) return data;
      const d = data as { has_more?: boolean; next_cursor?: string | null };
      this._hasMore = d?.has_more ?? false;
      this._nextCursor = d?.next_cursor ?? null;
      return data;
    },
    // client is in the deps so a late-set client (separate update than orgId) reruns.
    args: () =>
      [this.client, this.orgId, this._search, this._cursor, this._includeDisabled, this._limit] as const,
  });

  private async _handleRowAction(e: CustomEvent): Promise<void> {
    const { row, action } = e.detail as { row: { id: string; name: string; version: number }; action: string };
    if (!row?.id) return;
    if (action === 'edit') {
      this._navigate(`/orgs/${this.orgId}/flows/${row.id}`);
      return;
    }
    if (action === 'disable' || action === 'enable') {
      const { error } = await this.client.PATCH('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
        body: { enabled: action === 'enable', version: row.version },
      } as never);
      if (error) {
        this._actionError = this._errorMessage(error);
        // 409 means our cached version is stale — refresh to pull the current
        // one so the next attempt can succeed, but keep the conflict visible.
        if (this._isConflict(error)) void this._listTask.run();
        return;
      }
      this._actionError = null;
      void this._listTask.run();
      return;
    }
    if (action === 'delete') {
      const ok = await confirmDelete(row.name, 'flow');
      if (!ok) return;
      const { error } = await this.client.DELETE('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: this.orgId, id: row.id } },
      } as never);
      if (error) {
        this._actionError = this._errorMessage(error);
        if (this._isConflict(error)) void this._listTask.run();
        return;
      }
      this._actionError = null;
      void this._listTask.run();
    }
  }

  private _errorMessage(error: unknown): string {
    const reason = (error as { reason?: string; message?: string })?.reason
      ?? (error as { message?: string })?.message;
    return reason ?? String(error);
  }

  private _isConflict(error: unknown): boolean {
    const reason = (error as { reason?: string })?.reason ?? '';
    return reason === 'version_conflict' || /conflict/i.test(reason);
  }

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
      new CustomEvent('open-routing:navigate', { detail: { path }, bubbles: true, composed: true })
    );
  }

  private _setSearch(val: string): void {
    clearTimeout(this._searchDebounce);
    this._search = val;
    this._cursor = null;
    this._cursorStack = [];
  }

  private _handleSearch(e: Event): void {
    const val = (e.target as HTMLInputElement).value;
    clearTimeout(this._searchDebounce);
    this._searchDebounce = setTimeout(() => this._setSearch(val), 300);
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
    const row = e.detail?.row as Flow | undefined;
    if (row?.id) {
      this._navigate(`/orgs/${this.orgId}/flows/${row.id}`);
    }
  }

  private _handlePageChanged(e: CustomEvent): void {
    const { direction, limit } = e.detail as { cursor: string | null; direction: 'next' | 'prev'; limit: number };

    // The paginator reports a page-size change as direction:'next', so detect it
    // by comparing against the prior limit and treat it as a fresh page 1.
    if (limit !== this._limit) {
      this._limit = limit;
      this._cursor = null;
      this._cursorStack = [];
      return;
    }

    if (direction === 'next') {
      this._cursorStack = [...this._cursorStack, this._cursor];
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
          <p>No flows found matching '${this._search}'.</p>
          <button
            class="uk-button uk-button-default uk-button-small"
            @click=${() => this._setSearch('')}
          >Clear search</button>
        </div>
      `;
    }
    return html`
      <div class="empty-state">
        <p>No flows yet. Click <strong>+ Create flow</strong> to author your first routing flow.</p>
        <button
          class="uk-button uk-button-primary uk-button-small"
          @click=${() => this._navigate(`/orgs/${this.orgId}/flows/new`)}
        >+ Create flow</button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <div>
          <h1 class="page-title">Flows</h1>
          <p class="page-subtitle">Author, simulate, and publish routing flows.</p>
        </div>
        <button
          class="uk-button uk-button-primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/flows/new`)}
        >+ Create flow</button>
      </div>

      <div class="filter-row">
        <div class="search-wrap">
          <uk-icon icon="search" height="16" width="16"></uk-icon>
          <input
            class="uk-input"
            type="search"
            placeholder="Search flows…"
            .value=${this._search}
            @input=${this._handleSearch}
            aria-label="Search flows"
          />
          ${this._search ? html`
            <button class="search-clear" aria-label="Clear search" @click=${() => this._setSearch('')}>
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
          Include archived
        </label>
        <button class="icon-btn" title="Refresh" aria-label="Refresh" @click=${this._handleRefresh}>
          <uk-icon icon="refresh-cw" height="18" width="18"></uk-icon>
        </button>
      </div>

      ${this._actionError
        ? html`
            <div class="uk-alert uk-alert-danger" style="margin:0 0 12px;border-radius:8px" role="alert">
              <button
                class="uk-alert-close"
                type="button"
                aria-label="Dismiss"
                @click=${() => { this._actionError = null; }}
              >
                <uk-icon icon="x" height="14" width="14"></uk-icon>
              </button>
              <strong>Action failed.</strong> ${this._actionError}
            </div>
          `
        : nothing}

      <div class="table-card">
        ${this._listTask.render({
          pending: () => html`
            <or-data-table .columns=${this._columns} .rows=${[]} .loading=${true}></or-data-table>
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
              <strong>Failed to load flows.</strong>
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
    'or-flow-list': OrFlowList;
  }
}
