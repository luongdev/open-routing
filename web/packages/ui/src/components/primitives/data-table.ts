// Phase 6 Plan 04: <or-data-table> — reusable table primitive for all 6 entity list pages.
// Renders a <table> with sticky <thead> and dispatches 'or-row-click' on row click.
// Row context menu [⋮] uses sl-dropdown with Edit/Disable/Enable/Delete actions.
// CSS uses design token variables from or-light/or-dark/or-brand themes (D6-19, D6-20).
// Per-component Shoelace imports for tree-shaking (D6-08).

import { LitElement, html, css, type TemplateResult } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { repeat } from 'lit/directives/repeat.js';

// Shoelace per-component imports (D6-08: tree-shaking for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/dropdown/dropdown.js';
import '@shoelace-style/shoelace/dist/components/menu/menu.js';
import '@shoelace-style/shoelace/dist/components/menu-item/menu-item.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';

/** Column definition for or-data-table. */
export interface OrDataTableColumn {
  key: string;
  /** Column header label. Accepts plain string or a Lit TemplateResult for rich headers (e.g. sl-tooltip). */
  label: string | TemplateResult;
  width?: string;
  align?: 'left' | 'right' | 'center';
  /** Optional custom renderer — receives the full row object. */
  render?: (row: Record<string, unknown>) => TemplateResult;
}

/**
 * <or-data-table> — generic data table for all catalog entity list pages.
 *
 * Displays tabular data with sticky header, row hover, and row click events.
 * Provides a [⋮] context menu per row for Edit/Disable/Enable/Delete actions.
 *
 * Events:
 *   - 'or-row-click'   CustomEvent<{ row: object }> — row was clicked (not menu)
 *   - 'or-row-action'  CustomEvent<{ row: object, action: string }> — context menu action
 *
 * Usage:
 *   <or-data-table .columns=${cols} .rows=${rows}></or-data-table>
 */
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
      color: var(--or-color-text-body, #404040);
    }

    thead {
      position: sticky;
      top: 0;
      z-index: var(--or-z-sticky, 10);
      background: var(--or-color-sidebar-bg, #f5f5f5);
    }

    th {
      padding: 10px 12px;
      text-align: left;
      font-weight: 600;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--or-color-text-muted, #737373);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      white-space: nowrap;
    }

    th[data-align="right"] { text-align: right; }
    th[data-align="center"] { text-align: center; }

    tbody tr {
      cursor: pointer;
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      transition: background 0.1s ease;
    }

    tbody tr:hover {
      background: var(--or-color-row-hover, #fafafa);
    }

    tbody tr:last-child {
      border-bottom: none;
    }

    td {
      padding: 10px 12px;
      vertical-align: middle;
    }

    td[data-align="right"] { text-align: right; }
    td[data-align="center"] { text-align: center; }

    /* Code cells use monospace font (D6-V design token) */
    td.code-cell {
      font-family: var(--or-font-mono, ui-monospace, 'Cascadia Code', 'Fira Code', monospace);
      color: var(--or-color-code-fg, #1f6e77);
      font-size: 13px;
    }

    /* Context menu cell — does NOT trigger row click */
    td.menu-cell {
      width: 40px;
      padding: 0 4px;
      text-align: center;
    }

    td.menu-cell sl-dropdown {
      display: inline-block;
    }

    /* Loading state */
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
      background: var(--or-color-skeleton-base, #e5e5e5);
      animation: shimmer 1.5s ease-in-out infinite;
      flex: 1;
    }

    @keyframes shimmer {
      0%   { opacity: 1; }
      50%  { opacity: 0.4; }
      100% { opacity: 1; }
    }

    /* Empty state */
    .empty-state {
      padding: 40px 16px;
      text-align: center;
      color: var(--or-color-text-muted, #737373);
      font-size: 14px;
    }

    /* Menu trigger button */
    .menu-trigger {
      background: none;
      border: none;
      padding: 4px 8px;
      cursor: pointer;
      color: var(--or-color-text-muted, #737373);
      border-radius: 4px;
      font-size: 16px;
      line-height: 1;
    }

    .menu-trigger:hover {
      background: var(--or-color-row-hover, #fafafa);
      color: var(--or-color-text-body, #404040);
    }
  `;

  /** Column definitions. Order preserved. */
  @property({ type: Array }) accessor columns: OrDataTableColumn[] = [];

  /** Row data. Each row object's keys correspond to column keys. */
  @property({ type: Array }) accessor rows: Record<string, unknown>[] = [];

  /** Show loading skeleton rows instead of actual data. */
  @property({ type: Boolean }) accessor loading = false;

  /** Number of skeleton rows to show while loading. */
  @property({ type: Number }) accessor skeletonRows = 5;

  private _dispatchRowClick(row: Record<string, unknown>): void {
    this.dispatchEvent(
      new CustomEvent('or-row-click', {
        detail: { row },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _dispatchRowAction(row: Record<string, unknown>, action: string, e: Event): void {
    // Stop propagation so the row click does not also fire
    e.stopPropagation();
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
    if (col.render) {
      return col.render(row);
    }
    const value = row[col.key];
    return html`${value !== undefined && value !== null ? String(value) : ''}`;
  }

  private _renderContextMenu(row: Record<string, unknown>): TemplateResult {
    return html`
      <td class="menu-cell" @click=${(e: Event) => e.stopPropagation()}>
        <sl-dropdown>
          <button slot="trigger" class="menu-trigger" aria-label="Row actions">⋮</button>
          <sl-menu>
            <sl-menu-item @click=${(e: Event) => this._dispatchRowAction(row, 'edit', e)}>
              <sl-icon slot="prefix" name="pencil"></sl-icon>
              Edit
            </sl-menu-item>
            <sl-menu-item @click=${(e: Event) => this._dispatchRowAction(row, 'disable', e)}>
              <sl-icon slot="prefix" name="slash-circle"></sl-icon>
              Disable
            </sl-menu-item>
            <sl-menu-item @click=${(e: Event) => this._dispatchRowAction(row, 'enable', e)}>
              <sl-icon slot="prefix" name="check-circle"></sl-icon>
              Enable
            </sl-menu-item>
            <sl-menu-item variant="danger" @click=${(e: Event) => this._dispatchRowAction(row, 'delete', e)}>
              <sl-icon slot="prefix" name="trash"></sl-icon>
              Delete
            </sl-menu-item>
          </sl-menu>
        </sl-dropdown>
      </td>
    `;
  }

  override render() {
    if (this.loading) {
      return this._renderLoadingSkeleton();
    }

    return html`
      <table role="grid" aria-label="Data table">
        <thead>
          <tr>
            ${this.columns.map(col => html`
              <th
                scope="col"
                data-align=${col.align ?? 'left'}
                style=${col.width ? `width: ${col.width}` : ''}
              >
                ${col.label}
              </th>
            `)}
            <!-- Context menu header (no label) -->
            <th scope="col" style="width: 40px;" aria-label="Actions"></th>
          </tr>
        </thead>
        <tbody>
          ${when(
            this.rows.length === 0,
            () => html`
              <tr>
                <td colspan=${this.columns.length + 1} class="empty-state">
                  No data available
                </td>
              </tr>
            `,
            () => repeat(
              this.rows,
              (row, i) => (row['id'] as string | undefined) ?? i,
              (row) => html`
                <tr @click=${() => this._dispatchRowClick(row)}>
                  ${this.columns.map(col => html`
                    <td
                      data-align=${col.align ?? 'left'}
                      class=${col.key === 'code' ? 'code-cell' : ''}
                    >
                      ${this._renderCellValue(col, row)}
                    </td>
                  `)}
                  ${this._renderContextMenu(row)}
                </tr>
              `
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
