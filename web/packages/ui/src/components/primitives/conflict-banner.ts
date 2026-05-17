// Phase 6 Plan 04: <or-conflict-banner> — inline 409 conflict display component.
// Supports two modes per D6-03/D6-04:
//   - 'crud' mode: CRUD 409 version_conflict — shows server-vs-user field diff.
//   - 'status' mode: Status PATCH 409 invalid_transition — shows from/to copy + Refresh CTA.
// Security (T-06-04-01): Lit html`` template auto-escapes all interpolated values.
// aria-live="assertive" on the host for screen reader announcement on first appearance.
// Per-component Shoelace imports (D6-08).

import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/details/details.js';

/**
 * <or-conflict-banner> — inline 409 conflict resolution UI.
 *
 * CRUD mode (409 version_conflict, D6-03):
 *   Displays per-field diff of server's current value vs user's pending edits.
 *   Server data comes from the `current` field of the 409 response body (D-37).
 *   Two action buttons:
 *     "Review and re-submit" → dispatches event with action='review'
 *     "Discard my changes"   → dispatches event with action='discard'
 *
 * Status mode (409 invalid_transition, D6-04):
 *   Displays transition-not-allowed copy with from/to state names.
 *   Server data comes from `from`/`to` fields of the 409 response body (D-91).
 *   One action button:
 *     "Refresh transitions" → dispatches event with action='review'
 *
 * Events:
 *   'open-routing:conflict-acknowledged' CustomEvent<{ action: 'review' | 'discard' }>
 *     bubbles: true, composed: true (crosses Shadow DOM for Phase 7)
 *
 * Usage:
 *   <!-- CRUD 409: -->
 *   <or-conflict-banner
 *     mode="crud"
 *     .serverValue=${{ name: 'Alice N', email: 'alice@example.com' }}
 *     .userValue=${{ name: 'Alice', email: 'alice@example.com' }}
 *   ></or-conflict-banner>
 *
 *   <!-- Status 409: -->
 *   <or-conflict-banner
 *     mode="status"
 *     .serverValue=${{ from: 'Engaged', to: 'NotReady' }}
 *   ></or-conflict-banner>
 */
@customElement('or-conflict-banner')
export class OrConflictBanner extends LitElement {
  // IMPORTANT: aria-live="assertive" must be set on the host element so that
  // screen readers announce the banner immediately when it appears.
  // Set via connectedCallback on the host, not via static initializer,
  // because Lit's shadow-root rendering won't apply ARIA to the host itself.

  static override styles = css`
    :host {
      display: block;
      background: var(--or-color-conflict-bg, #fef3c7);
      border: 2px solid var(--or-color-conflict-border, #f59e0b);
      border-radius: var(--or-radius-md, 4px);
      padding: 16px;
      margin-bottom: 24px;
    }

    .conflict-banner {
      width: 100%;
    }

    .banner-header {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      margin-bottom: 12px;
    }

    .banner-icon {
      flex-shrink: 0;
      color: var(--or-color-conflict-border, #f59e0b);
      font-size: 20px;
      line-height: 1;
    }

    .banner-heading {
      font-size: 15px;
      font-weight: 600;
      color: var(--or-color-text-strong, #171717);
      margin: 0 0 4px;
    }

    .banner-body {
      font-size: 14px;
      color: var(--or-color-text-body, #404040);
      margin: 0;
    }

    /* Diff table (CRUD mode) */
    .diff-section {
      margin: 12px 0;
    }

    .diff-field {
      margin-bottom: 8px;
      border-radius: 4px;
      overflow: hidden;
      border: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .diff-field-name {
      padding: 4px 10px;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      background: var(--or-color-sidebar-bg, #f5f5f5);
      color: var(--or-color-text-muted, #737373);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
    }

    .diff-row {
      display: flex;
      gap: 0;
    }

    .diff-label {
      flex: 0 0 100px;
      padding: 6px 10px;
      font-size: 11px;
      font-weight: 500;
      color: var(--or-color-text-muted, #737373);
      border-right: 1px solid var(--or-color-divider, #e5e5e5);
      background: var(--or-color-card-bg, #ffffff);
    }

    .diff-value {
      flex: 1;
      padding: 6px 10px;
      font-size: 13px;
      font-family: var(--or-font-mono, ui-monospace, 'Cascadia Code', 'Fira Code', monospace);
      word-break: break-all;
    }

    .diff-value.removed {
      background: var(--or-color-diff-removed, #fecaca);
      color: #7f1d1d;
    }

    .diff-value.added {
      background: var(--or-color-diff-added, #bbf7d0);
      color: #14532d;
    }

    /* Action buttons */
    .action-row {
      display: flex;
      gap: 8px;
      margin-top: 16px;
      flex-wrap: wrap;
    }

    /* Status mode specific */
    .status-body {
      font-size: 14px;
      color: var(--or-color-text-body, #404040);
      margin: 0 0 12px;
      line-height: 1.5;
    }

    .state-badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: 3px;
      background: var(--or-color-sidebar-bg, #f5f5f5);
      font-family: var(--or-font-mono, ui-monospace, 'Cascadia Code', 'Fira Code', monospace);
      font-size: 12px;
      font-weight: 500;
    }
  `;

  /** Conflict display mode: 'crud' for version_conflict, 'status' for invalid_transition. */
  @property({ type: String }) mode: 'crud' | 'status' = 'crud';

  /**
   * For CRUD mode: the server's current entity data (from 409 `current` field per D-37).
   * For status mode: { from: string; to: string } (from 409 body per D-91).
   */
  @property({ type: Object }) serverValue: Record<string, unknown> = {};

  /** For CRUD mode: the user's pending edit data (what they were about to submit). */
  @property({ type: Object }) userValue: Record<string, unknown> = {};

  /** Whether to show the diff section in CRUD mode (default: true). */
  @property({ type: Boolean, attribute: 'show-diff' }) showDiff = true;

  override connectedCallback(): void {
    super.connectedCallback();
    // Set aria-live on the host so screen readers announce the banner immediately.
    // Must be "assertive" per UI-SPEC §8 accessibility contract.
    this.setAttribute('aria-live', 'assertive');
    this.setAttribute('role', 'alert');
  }

  private _dispatch(action: 'review' | 'discard'): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:conflict-acknowledged', {
        detail: { action },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _renderCrudDiff() {
    // Compute the changed fields: keys present in serverValue where values differ from userValue
    // Use JSON.stringify for deep comparison — entity fields can include
    // nested objects (e.g. skills arrays). Primitive values (strings, numbers,
    // booleans) stringify deterministically; key order within objects may vary
    // but that edge case only matters for deeply nested sub-objects which are
    // beyond v0.1 scope. For v0.1 entity fields (strings/numbers/booleans),
    // this is safe and avoids false diffs from reference inequality.
    const changedKeys = Object.keys(this.serverValue).filter(
      (key) => JSON.stringify(this.serverValue[key]) !== JSON.stringify(this.userValue[key])
    );

    if (changedKeys.length === 0) {
      return html`<p class="banner-body">No field-level differences detected.</p>`;
    }

    return html`
      <div class="diff-section">
        ${changedKeys.map(key => html`
          <div class="diff-field">
            <div class="diff-field-name">${key}</div>
            <div class="diff-row">
              <div class="diff-label">Server now</div>
              <div class="diff-value removed">${String(this.serverValue[key] ?? '')}</div>
            </div>
            <div class="diff-row">
              <div class="diff-label">Your edit</div>
              <div class="diff-value added">${String(this.userValue[key] ?? '')}</div>
            </div>
          </div>
        `)}
      </div>
    `;
  }

  private _renderCrudMode() {
    return html`
      <div class="conflict-banner">
        <div class="banner-header">
          <sl-icon class="banner-icon" name="exclamation-triangle"></sl-icon>
          <div>
            <p class="banner-heading">This was changed elsewhere</p>
            <p class="banner-body">
              Your edits are below — review the diff and re-submit, or discard
              your changes and load the server's version.
            </p>
          </div>
        </div>

        ${when(this.showDiff, () => this._renderCrudDiff())}

        <div class="action-row">
          <sl-button
            variant="primary"
            size="small"
            @click=${() => this._dispatch('review')}
          >
            Review and re-submit
          </sl-button>
          <sl-button
            variant="text"
            size="small"
            style="color: var(--sl-color-danger-500, #d92d20)"
            @click=${() => this._dispatch('discard')}
          >
            Discard my changes
          </sl-button>
        </div>
      </div>
    `;
  }

  private _renderStatusMode() {
    const from = String(this.serverValue['from'] ?? '');
    const to = String(this.serverValue['to'] ?? '');

    return html`
      <div class="conflict-banner">
        <div class="banner-header">
          <sl-icon class="banner-icon" name="exclamation-triangle"></sl-icon>
          <div>
            <p class="banner-heading">Transition isn't allowed</p>
          </div>
        </div>

        <p class="status-body">
          Status is now <span class="state-badge">${from}</span>.
          Your request to go to <span class="state-badge">${to}</span>
          isn't allowed from <span class="state-badge">${from}</span>.
          Wait for wrap-up to end, or use Force transition (admin).
        </p>

        <div class="action-row">
          <sl-button
            variant="primary"
            size="small"
            @click=${() => this._dispatch('review')}
          >
            Refresh transitions
          </sl-button>
        </div>
      </div>
    `;
  }

  override render() {
    return when(
      this.mode === 'status',
      () => this._renderStatusMode(),
      () => this._renderCrudMode()
    );
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-conflict-banner': OrConflictBanner;
  }
}
