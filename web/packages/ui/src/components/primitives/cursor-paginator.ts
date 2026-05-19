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
      gap: 12px;
      padding: 14px 18px;
      font-size: 13px;
      color: var(--muted-foreground);
      border-top: 1px solid var(--border);
      background: var(--card);
    }

    /* Nav buttons — proper uk-button-default look (border + shadow + hover) */
    .nav-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 14px;
      border-radius: 8px;
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--foreground);
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      box-shadow: var(--shadow-xs);
      transition: background .12s, border-color .12s, color .12s, transform .1s, box-shadow .12s;
    }

    .nav-btn:hover:not(:disabled) {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
      color: var(--primary);
      box-shadow: var(--shadow-sm);
    }

    .nav-btn:hover:not(:disabled) uk-icon {
      color: var(--primary);
    }

    .nav-btn:active:not(:disabled) {
      transform: translateY(1px);
      box-shadow: none;
    }

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

    /* Page indicator — coral chip so the current page is unmistakable */
    .page-indicator {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 6px 14px;
      border-radius: 9999px;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
      font-size: 13px;
      font-weight: 600;
      white-space: nowrap;
    }

    .page-indicator-num {
      font-variant-numeric: tabular-nums;
      font-weight: 700;
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
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      box-shadow: var(--shadow-xs);
      color: var(--foreground);
      font-size: 13px;
      font-weight: 600;
      padding: 7px 30px 7px 14px;
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

    return html`
      <button
        class="nav-btn"
        ?disabled=${prevDisabled}
        @click=${this._handlePrev}
        aria-label="Previous page"
      >
        <uk-icon icon="chevron-left" height="14" width="14"></uk-icon>
        Previous
      </button>

      <span class="page-indicator">
        Page <span class="page-indicator-num">${this._pageNum}</span>
      </span>

      <button
        class="nav-btn"
        ?disabled=${nextDisabled}
        @click=${this._handleNext}
        aria-label="Next page"
      >
        Next
        <uk-icon icon="chevron-right" height="14" width="14"></uk-icon>
      </button>

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
