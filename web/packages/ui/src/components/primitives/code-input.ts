// Phase 6 Plan 04: <or-code-input> — validates Phase 04.1 universal code field.
// Regex: ^[a-z][a-z0-9_]{0,63}$ (D04_1-03: starts with lowercase letter, max 64 chars).
// Wraps sl-input with setCustomValidity enforcement (WHATWG Constraint Validation API).
// Read-only state (D04_1-02): disabled sl-input + lock icon + override helper text.
// Per-component Shoelace imports (D6-08).

import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/input/input.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';

/**
 * CODE_PATTERN enforces Phase 04.1 D04_1-03:
 *   - Starts with a lowercase letter [a-z]
 *   - Followed by 0..63 characters of [a-z0-9_]
 *   - Total length 1..64 characters
 *
 * Note: The plan spec says ^[a-z][a-z0-9_]{0,63}$ which disallows hyphens.
 * Some other references mention ^[a-z0-9_-]+$ — D04_1-03 is the authoritative
 * source; using ^[a-z][a-z0-9_]{0,63}$ per plan spec.
 */
export const CODE_PATTERN = /^[a-z][a-z0-9_]{0,63}$/;

export const CODE_ERROR_MSG =
  'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores (max 64 chars)';

/**
 * <or-code-input> — validated code field input component.
 *
 * Used on all Create forms where the `code` field is the universal entity identifier.
 * On detail/edit forms, set readonly=true to render as disabled with lock icon and
 * immutability message (D04_1-02: code is immutable post-create).
 *
 * Methods:
 *   - validate(): boolean — run validation, set setCustomValidity, return true/false
 *
 * Events:
 *   - 'or-code-input' CustomEvent<{ value: string; valid: boolean }> on every change
 *
 * Usage (create form):
 *   <or-code-input label="Code" required></or-code-input>
 *
 * Usage (detail/edit — immutable):
 *   <or-code-input value="agent-001" readonly></or-code-input>
 */
@customElement('or-code-input')
export class OrCodeInput extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    .input-wrapper {
      position: relative;
    }

    sl-input {
      width: 100%;
    }

    /* Valid state — override Shoelace's default for consistency */
    sl-input[data-valid="true"]::part(base) {
      border-color: var(--sl-color-success-500, #027a48);
    }

    /* Invalid state */
    sl-input[data-valid="false"]::part(base) {
      border-color: var(--sl-color-danger-500, #d92d20);
    }

    .helper-text {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 4px;
    }

    .helper-text.error {
      color: var(--sl-color-danger-500, #d92d20);
    }

    .readonly-notice {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
      margin-top: 4px;
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .readonly-notice sl-icon {
      font-size: 12px;
      color: var(--or-color-text-muted, #737373);
    }
  `;

  /** Current value. Bound two-way via .value and @or-code-input events. */
  @property({ type: String }) value = '';

  /** Label displayed above the input. */
  @property({ type: String }) label = 'Code';

  /** Helper text shown below the input (overridden when readonly=true). */
  @property({ type: String }) helperText = '';

  /** When true, input is disabled with lock icon (D04_1-02: immutable after create). */
  @property({ type: Boolean }) readonly = false;

  /** When true, validation fails on empty value. */
  @property({ type: Boolean }) required = false;

  /** Internal validation state — null means untouched (not yet validated). */
  private _validationState: boolean | null = null;

  /**
   * Validate the current value against CODE_PATTERN.
   * Also calls setCustomValidity on the sl-input's internal native input.
   * @returns true if valid, false if invalid
   */
  validate(): boolean {
    if (this.readonly) return true;

    // Empty value is only an error when required=true; otherwise it's valid-until-populated.
    if (!this.value && !this.required) {
      this._validationState = null; // Reset to untouched state — not invalid, not valid
      this.requestUpdate();
      return true;
    }

    const valid = CODE_PATTERN.test(this.value);
    this._validationState = valid;

    // Set custom validity on the sl-input to integrate with browser form validation
    const slInput = this.shadowRoot?.querySelector('sl-input') as (HTMLElement & {
      setCustomValidity?: (msg: string) => void;
    }) | null;

    if (slInput?.setCustomValidity) {
      slInput.setCustomValidity(valid ? '' : CODE_ERROR_MSG);
    }

    this.requestUpdate();
    return valid;
  }

  private _handleInput(e: Event): void {
    const target = e.target as HTMLInputElement;
    this.value = target.value;

    // Re-validate on change if we've already validated once (live feedback)
    if (this._validationState !== null) {
      this.validate();
    }

    this.dispatchEvent(
      new CustomEvent('or-code-input', {
        detail: { value: this.value, valid: CODE_PATTERN.test(this.value) },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleBlur(): void {
    // Validate on blur so user sees feedback after leaving the field
    this.validate();
  }

  override render() {
    if (this.readonly) {
      return html`
        <div class="input-wrapper">
          <sl-input
            label=${this.label}
            value=${this.value}
            disabled
            aria-label=${this.label}
          >
            <sl-icon slot="suffix" name="lock"></sl-icon>
          </sl-input>
          <div class="readonly-notice">
            <sl-icon name="info-circle"></sl-icon>
            Code cannot be changed after create.
          </div>
        </div>
      `;
    }

    const showError = this._validationState === false;
    const showSuccess = this._validationState === true;

    return html`
      <div class="input-wrapper">
        <sl-input
          label=${this.label}
          value=${this.value}
          ?required=${this.required}
          data-valid=${this._validationState === null ? '' : String(!showError)}
          placeholder="e.g. agent_voice_en"
          aria-label=${this.label}
          aria-invalid=${showError ? 'true' : 'false'}
          aria-describedby="code-helper"
          @input=${this._handleInput}
          @blur=${this._handleBlur}
        >
          ${showSuccess ? html`<sl-icon slot="suffix" name="check-circle" style="color:var(--sl-color-success-500)"></sl-icon>` : ''}
          ${showError ? html`<sl-icon slot="suffix" name="exclamation-circle" style="color:var(--sl-color-danger-500)"></sl-icon>` : ''}
        </sl-input>

        <div
          id="code-helper"
          class="helper-text ${showError ? 'error' : ''}"
        >
          ${showError
            ? CODE_ERROR_MSG
            : (this.helperText || 'Lowercase letters, digits, and underscores only. Cannot be changed after create.')}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-code-input': OrCodeInput;
  }
}
