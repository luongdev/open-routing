// Phase 6 Plan 04: <or-cursor-paginator> — cursor-based pagination control.
// Wave 0.1 polish: pure Lit + Ember tokens, no shoelace.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

@customElement('or-cursor-paginator')
export class OrCursorPaginator extends LitElement {
  static override styles = css`
    :host {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-top: 20px;
      padding: 4px 14px;
      font-size: 13px;
      color: var(--muted-foreground);
      background: transparent;
    }

    /* Each control = same square boxed button, same radius/border/shadow */
    .page-num,
    .nav-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 36px;
      padding: 0 10px;
      border-radius: 8px;
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--foreground);
      font-size: 13px;
      font-weight: 500;
      font-variant-numeric: tabular-nums;
      cursor: pointer;
      box-shadow: var(--shadow-xs);
      transition: background .12s, border-color .12s, color .12s, transform .1s, box-shadow .12s;
    }

    .nav-btn {
      padding: 0;
      width: 36px;
    }

    .page-num:hover:not(:disabled),
    .nav-btn:hover:not(:disabled) {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
      color: var(--primary);
      box-shadow: var(--shadow-sm);
    }

    .nav-btn:hover:not(:disabled) uk-icon {
      color: var(--primary);
    }

    .page-num:active:not(:disabled),
    .nav-btn:active:not(:disabled) {
      transform: translateY(1px);
      box-shadow: none;
    }

    .page-num:disabled,
    .nav-btn:disabled {
      opacity: 0.45;
      cursor: not-allowed;
      box-shadow: none;
    }

    .nav-btn uk-icon {
      color: var(--muted-foreground);
      transition: color .12s;
    }

    .nav-btn:disabled uk-icon {
      color: var(--muted-foreground);
    }

    /* Current page = filled coral with white digit */
    .page-num--current {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
      box-shadow: 0 2px 6px -1px oklch(0.62 0.22 28 / 0.35), inset 0 1px 0 0 oklch(1 0 0 / 0.15);
      cursor: default;
      font-weight: 700;
    }

    .page-num--current:hover {
      background: var(--primary);
      border-color: var(--primary);
      color: var(--primary-foreground);
      box-shadow: 0 2px 6px -1px oklch(0.62 0.22 28 / 0.35), inset 0 1px 0 0 oklch(1 0 0 / 0.15);
    }

    /* Small gap between page-num group and nav arrows */
    .nav-group {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      margin-left: 4px;
    }

    .spacer {
      flex: 1;
    }

    .limit-label {
      font-size: 13px;
      color: var(--muted-foreground);
      white-space: nowrap;
    }

    .limit-select-wrap {
      position: relative;
      display: inline-flex;
      align-items: center;
    }

    .limit-select {
      appearance: none;
      -webkit-appearance: none;
      height: 36px;
      box-sizing: border-box;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      box-shadow: var(--shadow-xs);
      color: var(--foreground);
      font-size: 13px;
      font-weight: 600;
      padding: 0 32px 0 14px;
      cursor: pointer;
      transition: border-color .12s, box-shadow .12s;
      font-variant-numeric: tabular-nums;
    }

    .limit-select:hover {
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
    }

    .limit-select:focus,
    .limit-select:focus-visible {
      outline: none;
      border-color: var(--ring);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--ring) 25%, transparent);
    }

    .limit-select-chevron {
      position: absolute;
      right: 10px;
      top: 50%;
      transform: translateY(-50%);
      pointer-events: none;
      color: var(--muted-foreground);
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: Boolean }) hasMore = false;
  @property({ type: Array }) cursorStack: string[] = [];
  @property({ type: Number }) limit = 25;

  @state() private _pageNum = 1;

  private _handlePrev(): void {
    if (this.cursorStack.length === 0) return;
    const poppedCursor = this.cursorStack[this.cursorStack.length - 1];
    this._pageNum = Math.max(1, this._pageNum - 1);
    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: { cursor: poppedCursor ?? null, direction: 'prev', limit: this.limit },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleNext(): void {
    if (!this.hasMore) return;
    this._pageNum += 1;
    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: { cursor: null, direction: 'next', limit: this.limit },
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
    this.dispatchEvent(
      new CustomEvent('or-page-changed', {
        detail: { cursor: null, direction: 'next', limit: newLimit },
        bubbles: true,
        composed: true,
      })
    );
  }

  override render() {
    const prevDisabled = this.cursorStack.length === 0;
    const nextDisabled = !this.hasMore;

    // Cursor-based pagination only knows: current page index + whether there's
    // a next page. We surface the current page as a single coral square and
    // expose history pages (1..N-1) as separate boxes that the back button
    // walks through one step at a time — clicking N-1 is the same as Prev.
    const pageButtons: ReturnType<typeof html>[] = [];
    for (let p = 1; p < this._pageNum; p++) {
      pageButtons.push(html`
        <button
          class="page-num"
          @click=${this._handlePrev}
          aria-label="Go to page ${p}"
          title="Go to page ${p}"
        >${p}</button>
      `);
    }
    pageButtons.push(html`
      <button
        class="page-num page-num--current"
        aria-current="page"
        aria-label="Current page, ${this._pageNum}"
      >${this._pageNum}</button>
    `);

    return html`
      ${pageButtons}

      <div class="nav-group">
        <button
          class="nav-btn"
          ?disabled=${prevDisabled}
          @click=${this._handlePrev}
          aria-label="Previous page"
          title="Previous page"
        >
          <uk-icon icon="arrow-left" height="16" width="16"></uk-icon>
        </button>
        <button
          class="nav-btn"
          ?disabled=${nextDisabled}
          @click=${this._handleNext}
          aria-label="Next page"
          title="Next page"
        >
          <uk-icon icon="arrow-right" height="16" width="16"></uk-icon>
        </button>
      </div>

      <div class="spacer"></div>

      <span class="limit-label">Rows per page:</span>
      <div class="limit-select-wrap">
        <select
          class="limit-select"
          .value=${String(this.limit)}
          @change=${this._handleLimitChange}
          aria-label="Rows per page"
        >
          <option value="10" ?selected=${this.limit === 10}>10</option>
          <option value="25" ?selected=${this.limit === 25}>25</option>
          <option value="50" ?selected=${this.limit === 50}>50</option>
          <option value="100" ?selected=${this.limit === 100}>100</option>
        </select>
        <uk-icon class="limit-select-chevron" icon="chevron-down" height="14" width="14"></uk-icon>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-cursor-paginator': OrCursorPaginator;
  }
}
