// Phase 6 Plan 10 Task 1: <or-queue-picker> — async select primitive for queues.
// Fetches GET /v1/orgs/{orgId}/queues with name search (debounced 300ms).
// Emits 'or-queue-picker-change' CustomEvent<{queueId: string | null}> on selection.
// "(none)" option allows clearing to null for nullable default_queue_id fields.
// limit=25 + cursor pagination; T-06-10-04 DoS mitigation.
// ADMIN-04: only this.client.GET — never direct fetch().
// Wave 0.1: pure Lit + Ember tokens, no shoelace.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

interface QueueItem {
  id: string;
  code: string;
  name: string;
}

@customElement('or-queue-picker')
export class OrQueuePicker extends LitElement {
  static override styles = css`
    :host {
      display: block;
      position: relative;
      font-size: 14px;
    }

    /* ── Trigger button ─────────────────────────────────────────────── */
    .trigger {
      display: flex;
      align-items: center;
      gap: 6px;
      width: 100%;
      padding: 7px 10px;
      background: var(--background);
      border: 1px solid var(--border);
      border-radius: 8px;
      cursor: pointer;
      color: var(--foreground);
      font-size: 14px;
      text-align: left;
      transition: border-color .12s, box-shadow .12s;
    }

    .trigger:hover {
      border-color: var(--ring);
    }

    .trigger:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 30%, transparent);
    }

    .trigger[aria-expanded="true"] {
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 30%, transparent);
    }

    .trigger--disabled {
      opacity: 0.55;
      cursor: not-allowed;
      pointer-events: none;
    }

    .trigger-label {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--muted-foreground);
    }

    .trigger-label--selected {
      color: var(--foreground);
    }

    /* ── Dropdown panel ─────────────────────────────────────────────── */
    .dropdown {
      position: absolute;
      top: calc(100% + 4px);
      left: 0;
      right: 0;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: var(--shadow-md);
      z-index: 50;
      overflow: hidden;
      animation: pop-in .1s ease-out;
    }

    @keyframes pop-in {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    /* ── Search input ───────────────────────────────────────────────── */
    .search-wrap {
      padding: 8px;
      border-bottom: 1px solid var(--border);
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .search-wrap uk-icon {
      color: var(--muted-foreground);
      flex-shrink: 0;
    }

    .search-input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      font-size: 13px;
      color: var(--foreground);
    }

    .search-input::placeholder {
      color: var(--muted-foreground);
    }

    /* ── Option list ────────────────────────────────────────────────── */
    .option-list {
      max-height: 220px;
      overflow-y: auto;
      padding: 4px;
    }

    .option {
      display: flex;
      align-items: center;
      gap: 8px;
      width: 100%;
      padding: 7px 10px;
      border-radius: 6px;
      background: none;
      border: none;
      cursor: pointer;
      font-size: 13px;
      color: var(--foreground);
      text-align: left;
      transition: background .1s;
    }

    .option:hover,
    .option:focus-visible {
      background: var(--muted);
      outline: none;
    }

    .option--selected {
      color: var(--primary);
      font-weight: 500;
    }

    .option--none {
      color: var(--muted-foreground);
      font-style: italic;
    }

    .divider {
      height: 1px;
      background: var(--border);
      margin: 4px 2px;
    }

    /* ── Load-more ──────────────────────────────────────────────────── */
    .load-more-wrap {
      display: flex;
      justify-content: center;
      padding: 6px 8px 8px;
    }

    .load-more-btn {
      background: none;
      border: none;
      cursor: pointer;
      font-size: 12px;
      color: var(--primary);
      padding: 4px 8px;
      border-radius: 4px;
      transition: background .1s;
    }

    .load-more-btn:hover {
      background: color-mix(in oklch, var(--primary) 10%, transparent);
    }

    /* ── Spinner ────────────────────────────────────────────────────── */
    .spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid var(--border);
      border-top-color: var(--primary);
      border-radius: 50%;
      animation: spin .7s linear infinite;
      flex-shrink: 0;
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    /* ── Chip / tag ─────────────────────────────────────────────────── */
    .chip {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 2px 8px;
      border-radius: 999px;
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary);
      font-size: 12px;
      font-weight: 500;
      line-height: 1.6;
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: Object }) client!: ApiClient;
  @property({ type: String }) value: string | null = null;
  @property({ type: String }) placeholder = 'Select a queue';

  // --- Internal state ---
  @state() private _search = '';
  @state() private _queues: QueueItem[] = [];
  @state() private _hasMore = false;
  @state() private _nextCursor: string | null = null;
  @state() private _open = false;

  private _searchDebounce?: ReturnType<typeof setTimeout>;

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._handleOutsideClick);
    document.addEventListener('keydown', this._handleEsc);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._handleOutsideClick);
    document.removeEventListener('keydown', this._handleEsc);
  }

  private _handleOutsideClick = (e: MouseEvent): void => {
    if (!this._open) return;
    const path = e.composedPath();
    const inside = path.some((n) => n === this);
    if (!inside) this._open = false;
  };

  private _handleEsc = (e: KeyboardEvent): void => {
    if (e.key === 'Escape' && this._open) this._open = false;
  };

  // --- @lit/task for async queue fetch ---
  private _queueTask = new Task(this, {
    task: async ([orgId, search]) => {
      if (!orgId || !this.client) return { items: [], has_more: false, next_cursor: null };
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/queues' as never, {
        params: {
          path: { org_id: orgId as string },
          query: {
            name: (search as string) || undefined,
            limit: 25,
          },
        },
      } as never);
      if (error) return { items: [], has_more: false, next_cursor: null };
      const d = data as { items?: QueueItem[]; has_more?: boolean; next_cursor?: string | null };
      this._queues = d?.items ?? [];
      this._hasMore = d?.has_more ?? false;
      this._nextCursor = d?.next_cursor ?? null;
      return d;
    },
    args: () => [this.orgId, this._search] as const,
  });

  // --- Event handlers ---

  private _handleSearch(e: Event): void {
    const val = (e.target as HTMLInputElement).value ?? '';
    clearTimeout(this._searchDebounce);
    this._searchDebounce = setTimeout(() => {
      this._search = val;
    }, 300);
  }

  private _handleSelect(queueId: string | null): void {
    this.value = queueId;
    this._open = false;
    this.dispatchEvent(
      new CustomEvent('or-queue-picker-change', {
        detail: { queueId },
        bubbles: true,
        composed: true,
      })
    );
  }

  private async _handleLoadMore(): Promise<void> {
    if (!this._hasMore || !this._nextCursor) return;
    const { data, error } = await this.client.GET('/v1/orgs/{org_id}/queues' as never, {
      params: {
        path: { org_id: this.orgId },
        query: {
          name: this._search || undefined,
          cursor: this._nextCursor,
          limit: 25,
        },
      },
    } as never);
    if (!error && data) {
      const d = data as { items?: QueueItem[]; has_more?: boolean; next_cursor?: string | null };
      this._queues = [...this._queues, ...(d?.items ?? [])];
      this._hasMore = d?.has_more ?? false;
      this._nextCursor = d?.next_cursor ?? null;
    }
  }

  private _selectedLabel(): string {
    if (!this.value) return '';
    const q = this._queues.find((q) => q.id === this.value);
    return q ? `${q.code} — ${q.name}` : this.value;
  }

  // --- Render ---

  private _renderDropdown(loading: boolean) {
    return html`
      <div class="dropdown" role="listbox" aria-label="Queue options">
        <div class="search-wrap">
          <uk-icon icon="search" height="14" width="14"></uk-icon>
          <input
            class="search-input"
            type="search"
            placeholder="Search queues…"
            autocomplete="off"
            @input=${this._handleSearch}
            @click=${(e: Event) => e.stopPropagation()}
          />
        </div>
        <div class="option-list">
          ${loading
            ? html`<button class="option" disabled>
                <span class="spinner" data-testid="spinner"></span>
                Loading queues…
              </button>`
            : nothing}
          ${!loading ? html`
            <button
              class="option option--none"
              role="option"
              aria-selected=${this.value === null ? 'true' : 'false'}
              @click=${() => this._handleSelect(null)}
            >
              ${this.value === null
                ? html`<uk-icon icon="check" height="14" width="14"></uk-icon>`
                : nothing}
              (none)
            </button>
            <div class="divider"></div>
            ${this._queues.map((q) => html`
              <button
                class="option${this.value === q.id ? ' option--selected' : ''}"
                role="option"
                aria-selected=${this.value === q.id ? 'true' : 'false'}
                data-queue-id=${q.id}
                @click=${() => this._handleSelect(q.id)}
              >
                ${this.value === q.id
                  ? html`<uk-icon icon="check" height="14" width="14"></uk-icon>`
                  : nothing}
                ${q.code} — ${q.name}
              </button>
            `)}
            ${when(
              this._hasMore,
              () => html`
                <div class="load-more-wrap">
                  <button
                    class="load-more-btn"
                    @click=${(e: Event) => { e.stopPropagation(); void this._handleLoadMore(); }}
                  >
                    <uk-icon icon="chevron-down" height="14" width="14"></uk-icon>
                    Load more
                  </button>
                </div>
              `
            )}
          ` : nothing}
        </div>
      </div>
    `;
  }

  override render() {
    const label = this._selectedLabel();
    return html`
      ${this._queueTask.render({
        pending: () => html`
          <button
            class="trigger trigger--disabled"
            aria-haspopup="listbox"
            aria-expanded="false"
            disabled
          >
            <span class="spinner" data-testid="spinner"></span>
            <span class="trigger-label">Loading queues…</span>
            <uk-icon icon="chevron-down" height="16" width="16"></uk-icon>
          </button>
        `,
        complete: () => html`
          <button
            class="trigger"
            aria-haspopup="listbox"
            aria-expanded=${this._open ? 'true' : 'false'}
            @click=${(e: Event) => { e.stopPropagation(); this._open = !this._open; }}
          >
            <uk-icon icon="list" height="16" width="16"></uk-icon>
            <span class="trigger-label${label ? ' trigger-label--selected' : ''}">
              ${label || this.placeholder}
            </span>
            <uk-icon icon="chevron-down" height="16" width="16"></uk-icon>
          </button>
          ${this._open ? this._renderDropdown(false) : nothing}
        `,
        error: () => html`
          <button class="trigger trigger--disabled" disabled>
            <span class="trigger-label">Failed to load queues</span>
          </button>
        `,
      })}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-picker': OrQueuePicker;
  }
}
