// Phase 6 Plan 04: <or-data-table> — reusable table primitive for all 6 entity list pages.
// Wave 0.1 polish: pure Lit + Ember tokens, no shoelace. Custom kebab dropdown
// with lucide icons, divider, and proper hover/active states.
//
// Events:
//   - 'or-row-click'   CustomEvent<{ row }>            — row was clicked (not menu)
//   - 'or-row-action'  CustomEvent<{ row, action }>    — kebab menu action

import { LitElement, html, css, nothing, type TemplateResult } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { repeat } from 'lit/directives/repeat.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import './or-table.js';

export interface OrDataTableColumn {
  key: string;
  label: string | TemplateResult;
  width?: string;
  align?: 'left' | 'right' | 'center';
  render?: (row: Record<string, unknown>) => TemplateResult;
}

@customElement('or-data-table')
export class OrDataTable extends LitElement {
  static override styles = css`
    :host {
      display: block;
      width: 100%;
      overflow-x: auto;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
      color: var(--foreground);
    }

    thead {
      position: sticky;
      top: 0;
      z-index: 10;
      background: var(--card);
    }

    th {
      padding: 10px 14px;
      text-align: left;
      font-weight: 600;
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
      white-space: nowrap;
    }

    th[data-align="right"] { text-align: right; }
    th[data-align="center"] { text-align: center; }

    tbody tr {
      cursor: pointer;
      border-bottom: 1px solid var(--border);
      transition: background 0.1s ease;
    }

    tbody tr:hover {
      background: var(--muted);
    }

    tbody tr:last-child { border-bottom: none; }

    td {
      padding: 10px 14px;
      vertical-align: middle;
    }

    td[data-align="right"] { text-align: right; }
    td[data-align="center"] { text-align: center; }

    /* Kebab always visible — subtle muted color, brightens on row hover */
    tbody tr .row-actions { opacity: 0.65; transition: opacity .12s; }
    tbody tr:hover .row-actions { opacity: 1; }
    tbody tr.menu-open .row-actions { opacity: 1; }

    td.code-cell {
      font-family: var(--uk-font-monospace, ui-monospace, 'SF Mono', monospace);
      color: var(--muted-foreground);
      font-size: 13px;
    }

    /* ── Kebab cell ───────────────────────────────────────────────── */
    td.menu-cell {
      width: 44px;
      padding: 0 8px 0 4px;
      text-align: right;
      position: relative;
    }

    .kebab-btn {
      background: none;
      border: none;
      cursor: pointer;
      padding: 6px;
      border-radius: 6px;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transition: color .12s, background .12s;
    }

    .kebab-btn:hover,
    .kebab-btn:focus-visible {
      color: var(--foreground);
      background: color-mix(in oklch, var(--foreground) 10%, transparent);
      outline: none;
    }

    .kebab-btn[aria-expanded="true"] {
      color: var(--foreground);
      background: var(--muted);
    }

    /* ── Menu popup ────────────────────────────────────────────────── */
    .menu-popup {
      position: absolute;
      top: 100%;
      right: 8px;
      margin-top: 4px;
      min-width: 168px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: var(--shadow-md);
      padding: 4px;
      z-index: 50;
      animation: pop-in .1s ease-out;
    }

    @keyframes pop-in {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .menu-item {
      display: flex;
      align-items: center;
      gap: 9px;
      width: 100%;
      padding: 7px 10px;
      border-radius: 6px;
      background: none;
      border: none;
      cursor: pointer;
      font-size: 13px;
      color: var(--foreground);
      text-align: left;
      transition: background .1s, color .1s;
    }

    .menu-item uk-icon {
      color: var(--muted-foreground);
      flex-shrink: 0;
    }

    .menu-item:hover,
    .menu-item:focus-visible {
      background: var(--muted);
      outline: none;
    }

    .menu-item:hover uk-icon,
    .menu-item:focus-visible uk-icon {
      color: var(--foreground);
    }

    .menu-divider {
      height: 1px;
      background: var(--border);
      margin: 4px 2px;
    }

    .menu-item--danger {
      color: var(--destructive);
    }

    .menu-item--danger uk-icon {
      color: var(--destructive);
    }

    .menu-item--danger:hover {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      color: var(--destructive);
    }

    /* ── Loading skeleton ─────────────────────────────────────────── */
    .loading-container {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 16px;
    }

    .skeleton-row {
      display: flex;
      gap: 8px;
    }

    .skeleton-cell {
      height: 20px;
      border-radius: 4px;
      background: linear-gradient(
        90deg,
        var(--muted) 0%,
        color-mix(in oklch, var(--muted) 60%, var(--card)) 50%,
        var(--muted) 100%
      );
      background-size: 200% 100%;
      animation: shimmer 1.4s linear infinite;
      flex: 1;
    }

    @keyframes shimmer {
      0%   { background-position: 200% center; }
      100% { background-position: -200% center; }
    }

    .empty-state {
      padding: 40px 16px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 14px;
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: Array }) columns: OrDataTableColumn[] = [];
  @property({ type: Array }) rows: Record<string, unknown>[] = [];
  @property({ type: Boolean }) loading = false;
  @property({ type: Number }) skeletonRows = 5;

  @state() private _openMenuRowKey: string | number | null = null;

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
    if (this._openMenuRowKey === null) return;
    const path = e.composedPath();
    const insideMenu = path.some(
      (n) =>
        n instanceof Element &&
        (n.classList.contains('menu-cell') || n.classList.contains('menu-popup'))
    );
    if (!insideMenu) this._openMenuRowKey = null;
  };

  private _handleEsc = (e: KeyboardEvent): void => {
    if (e.key === 'Escape' && this._openMenuRowKey !== null) {
      this._openMenuRowKey = null;
    }
  };

  private _dispatchRowClick(row: Record<string, unknown>): void {
    this.dispatchEvent(
      new CustomEvent('or-row-click', { detail: { row }, bubbles: true, composed: true })
    );
  }

  private _dispatchRowAction(row: Record<string, unknown>, action: string, e: Event): void {
    e.stopPropagation();
    this._openMenuRowKey = null;
    this.dispatchEvent(
      new CustomEvent('or-row-action', {
        detail: { row, action },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _renderLoadingSkeleton() {
    const skelRows = Array.from({ length: this.skeletonRows });
    return html`
      <div data-testid="loading" class="loading-container">
        ${repeat(skelRows, (_, i) => i, () => html`
          <div class="skeleton-row">
            ${this.columns.map(() => html`<div class="skeleton-cell"></div>`)}
          </div>
        `)}
      </div>
    `;
  }

  private _renderCellValue(col: OrDataTableColumn, row: Record<string, unknown>): TemplateResult {
    if (col.render) return col.render(row);
    const value = row[col.key];
    return html`${value !== undefined && value !== null ? String(value) : ''}`;
  }

  private _renderKebab(row: Record<string, unknown>, rowKey: string | number) {
    const enabled = row['enabled'] !== false; // default true if undefined
    const isOpen = this._openMenuRowKey === rowKey;
    return html`
      <td class="menu-cell" @click=${(e: Event) => e.stopPropagation()}>
        <button
          class="kebab-btn row-actions"
          aria-haspopup="menu"
          aria-expanded=${isOpen ? 'true' : 'false'}
          aria-label="Row actions"
          @click=${(e: Event) => {
            e.stopPropagation();
            this._openMenuRowKey = isOpen ? null : rowKey;
          }}
        >
          <uk-icon icon="more-vertical" height="16" width="16"></uk-icon>
        </button>
        ${isOpen
          ? html`
              <div class="menu-popup" role="menu">
                <button
                  class="menu-item"
                  role="menuitem"
                  @click=${(e: Event) => this._dispatchRowAction(row, 'edit', e)}
                >
                  <uk-icon icon="pencil" height="14" width="14"></uk-icon>
                  Edit
                </button>
                ${enabled
                  ? html`
                      <button
                        class="menu-item"
                        role="menuitem"
                        @click=${(e: Event) => this._dispatchRowAction(row, 'disable', e)}
                      >
                        <uk-icon icon="ban" height="14" width="14"></uk-icon>
                        Disable
                      </button>
                    `
                  : html`
                      <button
                        class="menu-item"
                        role="menuitem"
                        @click=${(e: Event) => this._dispatchRowAction(row, 'enable', e)}
                      >
                        <uk-icon icon="check-circle" height="14" width="14"></uk-icon>
                        Enable
                      </button>
                    `}
                <div class="menu-divider"></div>
                <button
                  class="menu-item menu-item--danger"
                  role="menuitem"
                  @click=${(e: Event) => this._dispatchRowAction(row, 'delete', e)}
                >
                  <uk-icon icon="trash-2" height="14" width="14"></uk-icon>
                  Delete
                </button>
              </div>
            `
          : nothing}
      </td>
    `;
  }

  override render() {
    if (this.loading) return this._renderLoadingSkeleton();

    return html`
      <table role="grid" aria-label="Data table">
        <thead>
          <tr>
            ${this.columns.map((col) => html`
              <th
                scope="col"
                data-align=${col.align ?? 'left'}
                style=${col.width ? `width: ${col.width}` : ''}
              >
                ${col.label}
              </th>
            `)}
            <th scope="col" style="width: 44px;" aria-label="Actions"></th>
          </tr>
        </thead>
        <tbody>
          ${when(
            this.rows.length === 0,
            () => html`
              <tr>
                <td colspan=${this.columns.length + 1} class="empty-state">No data available</td>
              </tr>
            `,
            () => repeat(
              this.rows,
              (row, i) => (row['id'] as string | undefined) ?? i,
              (row, i) => {
                const rowKey = (row['id'] as string | undefined) ?? i;
                const isOpen = this._openMenuRowKey === rowKey;
                return html`
                  <tr
                    class=${isOpen ? 'menu-open' : ''}
                    @click=${() => this._dispatchRowClick(row)}
                  >
                    ${this.columns.map((col) => html`
                      <td
                        data-align=${col.align ?? 'left'}
                        class=${col.key === 'code' ? 'code-cell' : ''}
                      >
                        ${this._renderCellValue(col, row)}
                      </td>
                    `)}
                    ${this._renderKebab(row, rowKey)}
                  </tr>
                `;
              }
            )
          )}
        </tbody>
      </table>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-data-table': OrDataTable;
  }
}
