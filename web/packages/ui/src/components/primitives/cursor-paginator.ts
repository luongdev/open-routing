// Phase 6 Plan 04: <or-cursor-paginator> — cursor-based pagination control.
// Previous button disabled when cursorStack is empty (no previous pages).
// Next button disabled when hasMore is false.
// Limit picker (10/25/50/100) resets to page 1 when changed.
// Per-component Shoelace imports (D6-08).

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';

/**
 * <or-cursor-paginator> — cursor-based page navigation control.
 *
 * Works in conjunction with the list component which manages cursorStack.
 * Emits 'or-page-changed' events; the parent handles cursor state.
 *
 * Properties:
 *   - hasMore:     whether there's a next page available
 *   - cursorStack: array of previously-used cursors (for back navigation)
 *   - limit:       current page size (10/25/50/100)
 *
 * Events:
 *   - 'or-page-changed' CustomEvent<{ cursor: string|null; direction: 'next'|'prev'; limit: number }>
 *
 * Usage:
 *   <or-cursor-paginator
 *     .hasMore=${hasMore}
 *     .cursorStack=${cursorStack}
 *     .limit=${limit}
 *   ></or-cursor-paginator>
 */
@customElement('or-cursor-paginator')
export class OrCursorPaginator extends LitElement {
  static override styles = css`
    :host {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px 0;
      font-size: 14px;
      color: var(--or-color-text-muted, #737373);
    }

    .page-indicator {
      padding: 0 8px;
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
    }

    .limit-label {
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
      white-space: nowrap;
    }

    sl-select {
      width: 80px;
    }

    sl-button::part(base) {
      font-size: 13px;
    }
  `;

  /** Whether there's a next page (controls Next button disabled state). */
  @property({ type: Boolean }) accessor hasMore = false;

  /**
   * Stack of cursors for previous pages (managed by parent component).
   * Previous button is disabled when this is empty.
   */
  @property({ type: Array }) accessor cursorStack: string[] = [];

  /** Current page size. */
  @property({ type: Number }) accessor limit = 25;

  /** Internal page display counter (increments on next, decrements on prev). */
  @state() private accessor _pageNum = 1;

  private _handlePrev(): void {
    if (this.cursorStack.length === 0) return;

    // The parent manages cursorStack — we emit the event and they update the stack.
    // We pop from our perspective to signal "go to previous cursor".
    const poppedCursor = this.cursorStack[this.cursorStack.length - 1];
    this._pageNum = Math.max(1, this._pageNum - 1);

    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: {
          cursor: poppedCursor ?? null,
          direction: 'prev',
          limit: this.limit,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleNext(): void {
    if (!this.hasMore) return;

    this._pageNum += 1;

    // Cursor for next page is null here — the parent component knows the
    // next_cursor from the last API response and passes it via its own state.
    // Paginator emits the intent; parent resolves the actual cursor value.
    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: {
          cursor: null, // parent fills actual next_cursor from API response
          direction: 'next',
          limit: this.limit,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleLimitChange(e: Event): void {
    const target = e.target as HTMLSelectElement;
    const newLimit = parseInt(target.value, 10);
    if (isNaN(newLimit)) return;

    this._pageNum = 1;
    this.limit = newLimit;

    // Limit change resets pagination to the beginning (cursor: null)
    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: {
          cursor: null,
          direction: 'next', // effectively "go to first page"
          limit: newLimit,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  override render() {
    const prevDisabled = this.cursorStack.length === 0;
    const nextDisabled = !this.hasMore;

    return html`
      <sl-button
        size="small"
        variant="default"
        ?disabled=${prevDisabled}
        @click=${this._handlePrev}
        aria-label="Previous page"
      >
        Previous
      </sl-button>

      <span class="page-indicator">Page ${this._pageNum}</span>

      <sl-button
        size="small"
        variant="default"
        ?disabled=${nextDisabled}
        @click=${this._handleNext}
        aria-label="Next page"
      >
        Next
      </sl-button>

      <span class="limit-label">Rows per page:</span>
      <sl-select
        size="small"
        value=${String(this.limit)}
        @sl-change=${this._handleLimitChange}
        aria-label="Rows per page"
      >
        <sl-option value="10">10</sl-option>
        <sl-option value="25">25</sl-option>
        <sl-option value="50">50</sl-option>
        <sl-option value="100">100</sl-option>
      </sl-select>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-cursor-paginator': OrCursorPaginator;
  }
}
