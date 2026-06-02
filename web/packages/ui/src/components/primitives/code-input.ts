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
  @property({ type: Boolean, attribute: 'edit-button' }) editButton = false;
  @property({ type: String, attribute: 'edit-button-label' }) editButtonLabel = 'Edit code';
  @property({ type: Boolean, attribute: 'save-button' }) saveButton = false;
  @property({ type: String, attribute: 'save-button-label' }) saveButtonLabel = 'Save code';
  @property({ type: Boolean, attribute: 'cancel-button' }) cancelButton = false;
  @property({ type: String, attribute: 'cancel-button-label' }) cancelButtonLabel = 'Cancel';
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

  private _handleEditClick(): void {
    this.dispatchEvent(
      new CustomEvent('or-code-edit', {
        detail: {},
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleSaveClick(): void {
    if (!this.validate()) return;
    this.dispatchEvent(
      new CustomEvent('or-code-save', {
        detail: { value: this.value },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleCancelClick(): void {
    this.dispatchEvent(
      new CustomEvent('or-code-cancel', {
        detail: {},
        bubbles: true,
        composed: true,
      })
    );
  }

  override render() {
    const showError = this._validationState === false;
    const readonlyHelper = this.helperText || 'Code cannot be changed after create.';
    const helper = showError
      ? CODE_ERROR_MSG
      : (this.helperText || 'Lowercase letters, digits, and underscores only. Cannot be changed after create.');
    const helperColor = showError ? 'var(--destructive)' : 'var(--muted-foreground)';
    const editableInput = html`
      <or-input
        style="display:block; width:100%;"
        .value=${this.value}
        ?required=${this.required}
        placeholder="e.g. agent_voice_en"
        aria-label=${this.label}
        @or-input=${this._handleInput}
        @or-change=${this._handleChange}
      ></or-input>
    `;
    const editAction = this.editButton
      ? html`
          <button
            type="button"
            class="uk-button uk-button-default uk-button-small"
            style="display:inline-flex; align-items:center; gap:5px; white-space:nowrap;"
            aria-label=${this.editButtonLabel}
            @click=${() => this._handleEditClick()}
          >
            <uk-icon icon="pencil" height="14" width="14"></uk-icon>
            <span>${this.editButtonLabel}</span>
          </button>
        `
      : nothing;
    const commitActions = this.saveButton || this.cancelButton
      ? html`
          <div style="display:flex; gap:6px; align-items:center;">
            ${this.saveButton
              ? html`
                  <button
                    type="button"
                    class="uk-button uk-button-primary uk-button-small"
                    style="display:inline-flex; align-items:center; gap:5px; white-space:nowrap;"
                    aria-label=${this.saveButtonLabel}
                    @click=${() => this._handleSaveClick()}
                  >
                    <uk-icon icon="check" height="14" width="14"></uk-icon>
                    <span>${this.saveButtonLabel}</span>
                  </button>
                `
              : nothing}
            ${this.cancelButton
              ? html`
                  <button
                    type="button"
                    class="uk-button uk-button-default uk-button-small"
                    style="display:inline-flex; align-items:center; gap:5px; white-space:nowrap;"
                    aria-label=${this.cancelButtonLabel}
                    @click=${() => this._handleCancelClick()}
                  >
                    <uk-icon icon="x" height="14" width="14"></uk-icon>
                    <span>${this.cancelButtonLabel}</span>
                  </button>
                `
              : nothing}
          </div>
        `
      : nothing;

    if (this.readonly) {
      return html`
        <label class="uk-form-label" style="display:block; margin-bottom:4px;">${this.label}</label>
        <div style="display:grid; grid-template-columns:minmax(0, 1fr) auto; gap:8px; align-items:center;">
          <or-input
            style="display:block; width:100%;"
            .value=${this.value}
            .readonly=${true}
            .disabled=${true}
            suffix-icon="lock"
            aria-label=${this.label}
          ></or-input>
          ${editAction}
        </div>
        <div style="font-size:12px; color:var(--muted-foreground); margin-top:4px;">
          ${readonlyHelper}
        </div>
      `;
    }

    return html`
      <label class="uk-form-label" style="display:block; margin-bottom:4px;">${this.label}</label>
      <div style="display:grid; grid-template-columns:${this.saveButton || this.cancelButton ? 'minmax(0, 1fr) auto' : '1fr'}; gap:8px; align-items:center;">
        ${editableInput}
        ${commitActions}
      </div>
      <div style="font-size:12px; color:${helperColor}; margin-top:4px;">
        ${helper}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-code-input': OrCodeInput;
  }
}
