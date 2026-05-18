// Phase 6 Plan 03: <or-org-picker> — organization ID picker component.
// Shown at the root route '/'. Validates UUIDv7 on-submit (D6-V-09, D6-13).
// Persists last-used org_id to localStorage (D6-10).
// Dispatches 'open-routing:org-selected' with composed:true for Phase 7 Shadow DOM crossing.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/spinner/spinner.js';

/** UUIDv7 regex per D6-13. Client-side UX nicety; server is authoritative. */
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** localStorage key for last-used org ID (D6-10). */
const LS_KEY = 'or-last-org-id';

/**
 * <or-org-picker> — root route '/' component for the admin SPA.
 *
 * Shows a centered card with organization ID input. Validates UUIDv7 ON SUBMIT,
 * not on keystroke (D6-V-09). Persists and pre-fills last-used org_id from localStorage.
 *
 * Dispatches 'open-routing:org-selected' CustomEvent({ detail: { orgId }, bubbles, composed })
 * on successful validation. The shell listens to this event and navigates to /orgs/:orgId/agents.
 *
 * D6-20 note: This component does NOT set any theme tokens — that is the shell's responsibility.
 */
@customElement('or-org-picker')
export class OrOrgPicker extends LitElement {
  static override styles = css`
    :host {
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
    }
    .card {
      width: 480px;
      padding: 32px;
      border-radius: var(--or-radius-lg, 8px);
      border: 1px solid var(--or-color-card-border, #e5e5e5);
      background: var(--or-color-card-bg, #ffffff);
      box-shadow: var(--or-shadow-lg, 0 4px 24px rgba(0,0,0,0.08));
    }
    .wordmark {
      font-size: 24px;
      font-weight: 600;
      color: var(--or-color-text-strong, #171717);
      margin: 0 0 4px;
    }
    .subtitle {
      font-size: 14px;
      color: var(--or-color-text-muted, #737373);
      margin: 0 0 24px;
    }
    .field-label {
      display: block;
      font-size: 14px;
      font-weight: 500;
      color: var(--or-color-text-body, #404040);
      margin-bottom: 8px;
    }
    .helper-text {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 6px;
    }
    .validation-error {
      font-size: 12px;
      color: var(--sl-color-danger-500, #d92d20);
      margin-top: 6px;
    }
    .input-wrapper {
      margin-bottom: 16px;
    }
    .continue-btn {
      display: block;
      width: 100%;
      min-height: 44px;
      padding: 0 16px;
      font-size: 14px;
      font-weight: 500;
      background: var(--sl-color-primary-500, #2b8a93);
      color: var(--or-color-text-on-primary, #ffffff);
      border: none;
      border-radius: var(--sl-border-radius-medium, 4px);
      cursor: pointer;
      margin-top: 8px;
    }
    .continue-btn:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
    .last-used {
      margin-top: 16px;
      font-size: 13px;
      color: var(--or-color-text-muted, #737373);
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .last-used-uuid {
      font-family: monospace;
      font-size: 12px;
      background: var(--or-color-code-bg, #f5f5f5);
      padding: 2px 6px;
      border-radius: 3px;
    }
    .use-btn {
      background: none;
      border: none;
      padding: 0;
      font-size: 13px;
      color: var(--sl-color-primary-500, #2b8a93);
      cursor: pointer;
      text-decoration: underline;
    }
    .native-input {
      display: block;
      width: 100%;
      min-height: 40px;
      padding: 0 12px;
      font-size: 14px;
      border: 1px solid var(--or-color-card-border, #e5e5e5);
      border-radius: var(--sl-border-radius-medium, 4px);
      color: var(--or-color-text-body, #404040);
      background: var(--or-color-card-bg, #ffffff);
      box-sizing: border-box;
    }
    .native-input:focus {
      outline: 2px solid var(--or-color-focus-ring, #2b8a93);
      outline-offset: 2px;
    }
    .native-input.has-error {
      border-color: var(--sl-color-danger-500, #d92d20);
    }
  `;

  /** Pre-fill value — exposed as HTML attribute 'last-used-org-id'. */
  @property({ type: String, attribute: 'last-used-org-id' }) accessor lastUsedOrgId = '';

  @state() private accessor _value = '';
  @state() private accessor _error = '';
  @state() private accessor _submitting = false;
  @state() private accessor _lastUsedFromStorage = '';

  override firstUpdated(): void {
    // Pre-fill from localStorage first, fall back to lastUsedOrgId property (D6-10).
    // Wrapped in try/catch: localStorage can throw SecurityError in cross-origin embeds.
    let stored = '';
    try {
      stored = localStorage.getItem(LS_KEY) ?? '';
    } catch {
      // localStorage unavailable — use lastUsedOrgId property fallback
    }
    if (stored) {
      this._value = stored;
      this._lastUsedFromStorage = stored;
    } else if (this.lastUsedOrgId) {
      this._value = this.lastUsedOrgId;
    }
  }

  override updated(changed: Map<string, unknown>): void {
    // If lastUsedOrgId is set AFTER firstUpdated (e.g., set before appendChild in tests),
    // we need to apply it. But only if there's no localStorage value and _value is still empty.
    if (changed.has('lastUsedOrgId') && this.lastUsedOrgId && !this._value) {
      // Guard with try/catch: localStorage may throw SecurityError in cross-origin embeds.
      let stored = '';
      try {
        stored = localStorage.getItem(LS_KEY) ?? '';
      } catch {
        // localStorage unavailable — use lastUsedOrgId property fallback
      }
      if (!stored) {
        this._value = this.lastUsedOrgId;
      }
    }
  }

  /** Handle submit button click. Validates UUIDv7 ON SUBMIT per D6-V-09. */
  private _handleSubmit(): void {
    const trimmed = this._value.trim();

    // Validate on-submit, NOT on keystroke (D6-V-09)
    if (!UUIDV7_PATTERN.test(trimmed)) {
      this._error = 'Not a valid UUIDv7. Format: 8-4-4-4-12 hex characters with version digit 7.';
      return;
    }

    // Persist to localStorage (D6-10). Guarded: may throw in cross-origin embeds.
    try {
      localStorage.setItem(LS_KEY, trimmed);
    } catch {
      // Ignore — localStorage unavailable in some embed contexts
    }

    // Dispatch event with composed:true so it crosses Shadow DOM for Phase 7
    this.dispatchEvent(
      new CustomEvent('open-routing:org-selected', {
        detail: { orgId: trimmed },
        bubbles: true,
        composed: true,
      })
    );

    this._error = '';
    this._submitting = false;
  }

  /** Return the truncated form of a UUID for display. */
  private _truncateUuid(uuid: string): string {
    if (uuid.length <= 20) return uuid;
    return `${uuid.slice(0, 8)}…${uuid.slice(-4)}`;
  }

  override render() {
    const showLastUsed =
      this._lastUsedFromStorage &&
      this._lastUsedFromStorage !== this._value;

    return html`
      <div class="card">
        <p class="wordmark">Open Routing</p>
        <p class="subtitle">Standalone admin console</p>

        <label class="field-label" for="org-id-input">Organization ID</label>
        <div class="input-wrapper">
          <input
            id="org-id-input"
            class="native-input ${this._error ? 'has-error' : ''}"
            type="text"
            .value=${this._value}
            placeholder="01901b2c-7f3a-7abc-8d4e-..."
            aria-label="Organization ID"
            aria-describedby="${this._error ? 'org-id-error' : 'org-id-helper'}"
            @input=${(e: Event) => {
              this._value = (e.target as HTMLInputElement).value;
              // Clear error on input so user knows they can try again
              if (this._error) this._error = '';
            }}
          />
          ${this._error
            ? html`<span id="org-id-error" class="validation-error">${this._error}</span>`
            : html`<span id="org-id-helper" class="helper-text">
                UUIDv7 format. Example: 01901b2c-7f3a-7abc-8d4e-&hellip;
              </span>`
          }
        </div>

        <button
          type="submit"
          class="continue-btn"
          ?disabled=${this._submitting}
          @click=${this._handleSubmit}
        >
          ${this._submitting ? html`<sl-spinner style="font-size:1em"></sl-spinner>` : 'Continue'}
        </button>

        ${showLastUsed
          ? html`
            <div class="last-used">
              Last used:
              <code class="last-used-uuid">${this._truncateUuid(this._lastUsedFromStorage)}</code>
              <button
                class="use-btn"
                @click=${() => { this._value = this._lastUsedFromStorage; this._error = ''; }}
              >
                Use
              </button>
            </div>
          `
          : null
        }
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-org-picker': OrOrgPicker;
  }
}
