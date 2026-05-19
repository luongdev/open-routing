// Phase 6 Plan 04: <or-code-input> — validates Phase 04.1 universal code field.
// Regex: ^[a-z][a-z0-9_]{0,63}$ (D04_1-03: starts with lowercase letter, max 64 chars).
// Read-only state (D04_1-02): disabled input + lock indicator + override helper text.
// Plan 07-w0-11: Refactored to use or-input internally, light DOM.

import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import './or-input.js';
import type { OrInput } from './or-input.js';

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

// Characters that NFD decomposition cannot strip (no base+combining form).
const _NON_DECOMPOSABLE: Record<string, string> = {
  đ: 'd', ð: 'd', ø: 'o', ł: 'l', æ: 'ae', œ: 'oe', þ: 'th', ß: 'ss',
};
const _NON_DECOMPOSABLE_RE = new RegExp(
  `[${Object.keys(_NON_DECOMPOSABLE).join('')}]`,
  'gi',
);

export function nameToCode(name: string): string {
  return name
    .replace(_NON_DECOMPOSABLE_RE, (c) => _NON_DECOMPOSABLE[c.toLowerCase()] ?? c)
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9_]/g, '_')
    .replace(/_+/g, '_')
    .replace(/^[^a-z]+/, '')
    .replace(/_+$/, '')
    .substring(0, 64);
}

export const CODE_ERROR_MSG =
  'Code must start with a lowercase letter and contain only lowercase letters, digits, and underscores (max 64 chars)';

/**
 * <or-code-input> — validated code field input component.
 *
 * Usage (create form):
 *   <or-code-input label="Code" required></or-code-input>
 *
 * Usage (detail/edit — immutable):
 *   <or-code-input value="agent-001" readonly></or-code-input>
 */
@customElement('or-code-input')
export class OrCodeInput extends LitElement {
  // Light DOM — inherits Frankenstyle uk-* classes from global stylesheet (W0.0-11)
  override createRenderRoot() { return this; }

  @property({ type: String }) value = '';
  @property({ type: String }) label = 'Code';
  @property({ type: String }) helperText = '';
  // D04_1-02: immutable after create
  @property({ type: Boolean }) readonly = false;
  @property({ type: Boolean }) required = false;

  /** Internal validation state — null means untouched (not yet validated). */
  private _validationState: boolean | null = null;

  validate(): boolean {
    if (this.readonly) return true;

    if (!this.value && !this.required) {
      this._validationState = null;
      // Clear any previously set customValidity so browsers don't block submit with stale message
      const orInput = this.querySelector('or-input') as OrInput | null;
      orInput?.setCustomValidity('');
      this.requestUpdate();
      return true;
    }

    const valid = CODE_PATTERN.test(this.value);
    this._validationState = valid;

    const orInput = this.querySelector('or-input') as OrInput | null;
    orInput?.setCustomValidity(valid ? '' : CODE_ERROR_MSG);

    this.requestUpdate();
    return valid;
  }

  private _handleInput(e: CustomEvent<{ value: string }>): void {
    this.value = e.detail.value;

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

  private _handleChange(): void {
    // or-change fires on blur — validate to show feedback
    this.validate();
  }

  override render() {
    const showError = this._validationState === false;

    if (this.readonly) {
      return html`
        <or-input
          label=${this.label}
          .value=${this.value}
          .readonly=${true}
          .disabled=${true}
          suffix-icon="lock"
          aria-label=${this.label}
        ></or-input>
        <div style="font-size:12px; color:var(--muted-foreground); margin-top:4px;">
          Code cannot be changed after create.
        </div>
      `;
    }

    return html`
      <or-input
        label=${this.label}
        .value=${this.value}
        ?required=${this.required}
        placeholder="e.g. agent_voice_en"
        error-text=${showError ? CODE_ERROR_MSG : ''}
        helper-text=${showError ? '' : (this.helperText || 'Lowercase letters, digits, and underscores only. Cannot be changed after create.')}
        aria-label=${this.label}
        @or-input=${this._handleInput}
        @or-change=${this._handleChange}
      ></or-input>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-code-input': OrCodeInput;
  }
}
