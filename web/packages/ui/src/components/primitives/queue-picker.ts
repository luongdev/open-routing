// Phase 6 Plan 10 Task 1: <or-queue-picker> — async select primitive for queues.
// Fetches GET /v1/orgs/{orgId}/queues with name search (debounced 300ms).
// Emits 'or-queue-picker-change' CustomEvent<{queueId: string | null}> on selection.
// "(none)" option allows clearing to null for nullable default_queue_id fields.
// limit=25 + cursor pagination; T-06-10-04 DoS mitigation.
// Per-component Shoelace imports (D6-08).
// ADMIN-04: only this.client.GET — never direct fetch().

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/divider/divider.js';
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

interface QueueItem {
  id: string;
  code: string;
  name: string;
}

/**
 * <or-queue-picker> — async queue selector primitive.
 *
 * Fetches queues from /v1/orgs/{orgId}/queues with optional name search.
 * Supports cursor-based pagination (limit=25).
 * Emits 'or-queue-picker-change' with {queueId: string | null}.
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — current org UUID
 *   - client: ApiClient — passed from parent component
 *   - value: string | null — currently selected queue ID (two-way binding)
 *   - placeholder: string — select placeholder text
 */
@customElement('or-queue-picker')
export class OrQueuePicker extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .picker-search {
      padding: 4px 8px 8px;
    }

    .load-more {
      display: flex;
      justify-content: center;
      padding: 4px 8px;
    }
  `;

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;
  @property({ type: String }) accessor value: string | null = null;
  @property({ type: String }) accessor placeholder = 'Select a queue';

  // --- Internal state ---
  @state() private accessor _search = '';
  @state() private accessor _queues: QueueItem[] = [];
  @state() private accessor _hasMore = false;
  @state() private accessor _nextCursor: string | null = null;

  private _searchDebounce?: ReturnType<typeof setTimeout>;

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

  private _handleChange(e: Event): void {
    // Guard: only process sl-change from sl-select, not from sl-input inside the dropdown
    // (Gemini HIGH: nested sl-input sl-change events bubble up and contaminate value)
    const target = e.target as HTMLElement;
    if (target.tagName.toLowerCase() !== 'sl-select') return;

    const select = target as HTMLElement & { value: string };
    const rawValue = select.value;
    const queueId = rawValue === '' ? null : rawValue;
    this.value = queueId;
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

  // --- Render ---

  override render() {
    return this._queueTask.render({
      pending: () => html`
        <sl-select disabled placeholder="${this.placeholder}">
          <sl-spinner slot="prefix"></sl-spinner>
          <sl-option value="">Loading queues…</sl-option>
        </sl-select>
      `,
      complete: () => html`
        <sl-select
          .value=${this.value ?? ''}
          placeholder="${this.placeholder}"
          @sl-change=${this._handleChange}
        >
          <!-- Search input at top of dropdown -->
          <sl-input
            slot="label"
            class="picker-search"
            placeholder="Search queues…"
            size="small"
            clearable
            @sl-input=${this._handleSearch}
            @click=${(e: Event) => e.stopPropagation()}
          >
            <sl-icon name="search" slot="prefix"></sl-icon>
          </sl-input>

          <!-- (none) option for clearing nullable field -->
          <sl-option value="">(none)</sl-option>

          <sl-divider></sl-divider>

          ${this._queues.map(
            (q) => html`
              <sl-option value="${q.id}">${q.code} — ${q.name}</sl-option>
            `
          )}

          ${when(
            this._hasMore,
            () => html`
              <div class="load-more">
                <sl-button
                  size="small"
                  variant="text"
                  @click=${(e: Event) => { e.stopPropagation(); void this._handleLoadMore(); }}
                >Load more</sl-button>
              </div>
            `
          )}
        </sl-select>
      `,
      error: () => html`
        <sl-select disabled placeholder="Failed to load queues">
          <sl-option value="">(none)</sl-option>
        </sl-select>
      `,
    });
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-queue-picker': OrQueuePicker;
  }
}
