// Phase 6 Plan 04: <or-conflict-banner> — inline 409 conflict display component.
// Wave 0.1 polish: pure Lit + Ember tokens, no shoelace.
// Supports two modes per D6-03/D6-04:
//   - 'crud' mode: CRUD 409 version_conflict — shows server-vs-user field diff.
//   - 'status' mode: Status PATCH 409 invalid_transition — shows from/to copy + Refresh CTA.

import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

@customElement('or-conflict-banner')
export class OrConflictBanner extends LitElement {
  static override styles = css`
    :host {
      display: block;
      background: color-mix(in oklch, var(--warning) 14%, transparent);
      border: 1px solid color-mix(in oklch, var(--warning) 45%, transparent);
      border-radius: 10px;
      padding: 14px 16px;
      margin-bottom: 16px;
    }

    .conflict-banner {
      width: 100%;
    }

    .banner-header {
      display: flex;
      align-items: flex-start;
      gap: 10px;
      margin-bottom: 10px;
    }

    .banner-icon {
      flex-shrink: 0;
      color: var(--warning);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin-top: 1px;
    }

    .banner-heading {
      font-size: 14px;
      font-weight: 600;
      color: var(--foreground);
      margin: 0 0 2px;
    }

    .banner-body {
      font-size: 13px;
      color: var(--foreground);
      margin: 0;
      line-height: 1.45;
    }

    /* Diff table (CRUD mode) */
    .diff-section {
      margin: 10px 0;
    }

    .diff-field {
      margin-bottom: 8px;
      border-radius: 8px;
      overflow: hidden;
      border: 1px solid var(--border);
    }

    .diff-field-name {
      padding: 5px 10px;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      background: var(--muted);
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }

    .diff-row {
      display: flex;
      gap: 0;
      background: var(--card);
    }

    .diff-row + .diff-row {
      border-top: 1px solid var(--border);
    }

    .diff-label {
      flex: 0 0 100px;
      padding: 6px 10px;
      font-size: 11px;
      font-weight: 500;
      color: var(--muted-foreground);
      border-right: 1px solid var(--border);
    }

    .diff-value {
      flex: 1;
      padding: 6px 10px;
      font-size: 13px;
      font-family: var(--uk-font-monospace, ui-monospace, 'SF Mono', monospace);
      word-break: break-all;
    }

    .diff-value.removed {
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
      color: var(--destructive);
    }

    .diff-value.added {
      background: color-mix(in oklch, var(--success) 14%, transparent);
      color: var(--success);
    }

    /* Action buttons */
    .action-row {
      display: flex;
      gap: 8px;
      margin-top: 12px;
      flex-wrap: wrap;
      align-items: center;
    }

    .link-btn {
      background: none;
      border: none;
      cursor: pointer;
      font-size: 13px;
      font-weight: 500;
      color: var(--destructive);
      padding: 6px 8px;
      border-radius: 6px;
      transition: background .12s;
    }

    .link-btn:hover {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
    }

    /* Status mode specific */
    .status-body {
      font-size: 13px;
      color: var(--foreground);
      margin: 0 0 10px;
      line-height: 1.5;
    }

    .state-badge {
      display: inline-block;
      padding: 1px 8px;
      border-radius: 6px;
      background: var(--card);
      border: 1px solid var(--border);
      font-family: var(--uk-font-monospace, ui-monospace, 'SF Mono', monospace);
      font-size: 12px;
      font-weight: 500;
      color: var(--foreground);
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: String }) mode: 'crud' | 'status' = 'crud';
  @property({ type: Object }) serverValue: Record<string, unknown> = {};
  @property({ type: Object }) userValue: Record<string, unknown> = {};
  @property({ type: Boolean, attribute: 'show-diff' }) showDiff = true;

  override connectedCallback(): void {
    super.connectedCallback();
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
    const changedKeys = Object.keys(this.serverValue).filter(
      (key) => JSON.stringify(this.serverValue[key]) !== JSON.stringify(this.userValue[key])
    );

    if (changedKeys.length === 0) {
      return html`<p class="banner-body">No field-level differences detected.</p>`;
    }

    return html`
      <div class="diff-section">
        ${changedKeys.map((key) => html`
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
          <uk-icon class="banner-icon" icon="triangle-alert" height="18" width="18"></uk-icon>
          <div>
            <p class="banner-heading">This was changed elsewhere</p>
            <p class="banner-body">
              Your edits are below — review the diff and re-submit, or discard your
              changes and load the server's version.
            </p>
          </div>
        </div>

        ${when(this.showDiff, () => this._renderCrudDiff())}

        <div class="action-row">
          <button
            class="uk-button uk-button-primary uk-button-small"
            @click=${() => this._dispatch('review')}
          >
            Review and re-submit
          </button>
          <button class="link-btn" @click=${() => this._dispatch('discard')}>
            Discard my changes
          </button>
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
          <uk-icon class="banner-icon" icon="triangle-alert" height="18" width="18"></uk-icon>
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
          <button
            class="uk-button uk-button-primary uk-button-small"
            @click=${() => this._dispatch('review')}
          >
            Refresh transitions
          </button>
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
