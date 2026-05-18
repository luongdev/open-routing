// Phase 6 Plan 05 Task 1: <or-agent-list> — Agent entity list page.
// Uses @lit/task for async state machine; passes typed client as @property.
// Debounces name search 300ms per UI-SPEC §5.3 + D6-V-11.
// Per-component Shoelace imports for tree-shaking (D6-08).
// ADMIN-04: only this.client.GET — never direct fetch().

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import type { components } from '../../api/generated.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/checkbox/checkbox.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/alert/alert.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';

// Primitives
import '../primitives/data-table.js';
import '../primitives/cursor-paginator.js';
import type { OrDataTableColumn } from '../primitives/data-table.js';

type Agent = components['schemas']['Agent'];

/**
 * <or-agent-list> — Agent entity list page.
 *
 * Fetches GET /v1/orgs/{org_id}/agents via @lit/task.
 * Renders rows in <or-data-table> with cursor pagination.
 * Dispatches 'open-routing:navigate' on row click and "+ New agent" CTA.
 * Debounces name search by 300ms per D6-V-11.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from the shell at boot
 */
@customElement('or-agent-list')
export class OrAgentList extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
    }

    .page-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 20px;
    }

    .page-title {
      font-size: var(--or-text-display, 24px);
      font-weight: 700;
      color: var(--or-color-text-strong, #171717);
      margin: 0;
    }

    .filter-row {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 16px;
      flex-wrap: wrap;
    }

    .filter-row sl-input {
      min-width: 240px;
      flex: 1;
    }

    .filter-row sl-checkbox {
      white-space: nowrap;
    }

    .empty-state {
      padding: 40px 16px;
      text-align: center;
      color: var(--or-color-text-muted, #737373);
    }

    .empty-state p {
      margin: 0 0 12px;
    }

    .table-container {
      width: 100%;
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;
  @property({ type: Boolean, attribute: 'status-mode' }) accessor statusMode = false;

  // --- Internal state ---
  @state() private accessor _search = '';
  @state() private accessor _cursor: string | null = null;
  @state() private accessor _cursorStack: string[] = [];
  @state() private accessor _includeDisabled = false;
  @state() private accessor _limit = 25;
  @state() private accessor _nextCursor: string | null = null;
  @state() private accessor _hasMore = false;

  private _searchDebounce?: ReturnType<typeof setTimeout>;

  // --- Column definitions per UI-SPEC §5.3 ---
  private _columns: OrDataTableColumn[] = [
    {
      key: 'code',
      label: 'Code',
      render: (row) =>
        html`<code class="code-cell" style="font-family:var(--or-font-mono,monospace);color:var(--or-color-code-fg,#1f6e77)">${String(row['code'] ?? '')}</code>`,
    },
    { key: 'name', label: 'Name' },
    { key: 'email', label: 'Email' },
    {
      key: 'enabled',
      label: 'Enabled',
      render: (row) =>
        row['enabled']
          ? html`<sl-icon name="check-lg" style="color:var(--sl-color-success-500)"></sl-icon>`
          : html`<sl-icon name="x-lg" style="color:var(--or-color-text-muted)"></sl-icon>`,
    },
    {
      key: 'updated_at',
      label: 'Updated',
      render: (row) => {
        const iso = String(row['updated_at'] ?? '');
        return html`<sl-tooltip content="${iso}"><span>${this._relativeTime(iso)}</span></sl-tooltip>`;
      },
    },
  ];

  // --- Async task (§3 Lit Task pattern) ---
  private _listTask = new Task(this, {
    task: async ([orgId, search, cursor, includeDisabled, limit]) => {
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/agents', {
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

  // --- Helpers ---

  /** Simple relative time without external lib (e.g. "2h ago", "3d ago"). */
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

  // --- Event handlers ---

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
    const row = e.detail?.row as Agent | undefined;
    if (row?.id) {
      const dest = this.statusMode
        ? `/orgs/${this.orgId}/agents/${row.id}/status`
        : `/orgs/${this.orgId}/agents/${row.id}`;
      this._navigate(dest);
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
      // prev — pop from stack
      const newStack = [...this._cursorStack];
      const prevCursor = newStack.pop() ?? null;
      this._cursorStack = newStack;
      this._cursor = prevCursor;
      void cursor; // cursor from paginator is just a hint
    }
  }

  // --- Render ---

  private _renderEmptyState() {
    if (this._search) {
      return html`
        <div class="empty-state">
          <p>No agents found matching '${this._search}'.</p>
          <sl-button
            size="small"
            @click=${() => {
              this._search = '';
              this._cursor = null;
              this._cursorStack = [];
            }}
          >Clear search</sl-button>
        </div>
      `;
    }
    return html`
      <div class="empty-state">
        <p>No agents yet. Click <strong>+ Create agent</strong> to add your first.</p>
        <sl-button
          variant="primary"
          size="small"
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents/new`)}
        >+ Create agent</sl-button>
      </div>
    `;
  }

  override render() {
    return html`
      <div class="page-header">
        <h1 class="page-title">Agents</h1>
        <sl-button
          variant="primary"
          @click=${() => this._navigate(`/orgs/${this.orgId}/agents/new`)}
        >+ Create agent</sl-button>
      </div>

      <div class="filter-row">
        <sl-input
          placeholder="Search by name…"
          clearable
          @sl-input=${this._handleSearch}
          aria-label="Search agents"
        >
          <sl-icon name="search" slot="prefix"></sl-icon>
        </sl-input>
        <sl-checkbox
          ?checked=${this._includeDisabled}
          @sl-change=${this._handleIncludeDisabledChange}
        >Include disabled</sl-checkbox>
        <sl-icon-button
          name="arrow-clockwise"
          label="Refresh"
          @click=${this._handleRefresh}
        ></sl-icon-button>
      </div>

      <div class="table-container">
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
            <sl-alert variant="danger" open>
              <sl-icon slot="icon" name="exclamation-octagon"></sl-icon>
              <strong>Failed to load agents.</strong>
              ${(err as { reason?: string })?.reason ?? String(err)}
              <sl-button
                size="small"
                slot="footer"
                @click=${this._handleRefresh}
              >Retry</sl-button>
            </sl-alert>
          `,
        })}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-agent-list': OrAgentList;
  }
}
