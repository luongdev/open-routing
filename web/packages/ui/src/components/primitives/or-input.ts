import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('or-input')
export class OrInput extends LitElement {
  @property({ type: String }) type = 'text';
  @property({ type: String }) value = '';
  @property({ type: String }) placeholder = '';
  @property({ type: Boolean }) disabled = false;
  @property({ type: Boolean }) required = false;
  @property({ type: Boolean }) readonly = false;
  @property({ type: String }) name = '';
  @property({ type: String }) label = '';
  @property({ type: String, attribute: 'helper-text' }) helperText = '';
  @property({ type: String, attribute: 'error-text' }) errorText = '';
  @property({ type: String }) size: 'sm' | 'md' | 'lg' = 'md';

  // Stable fallback ID generated once per instance — avoids broken label/input association on re-renders
  private readonly _fallbackId = Math.random().toString(36).slice(2, 10);

  // Light DOM — Frankenstyle uk-input and uk-form-* classes resolve from global stylesheet
  override createRenderRoot() { return this; }

  private get inputClasses(): string {
    const base = 'uk-input';
    // Frankenstyle size classes: small/medium/large (not sm/md/lg)
    const sizeMap: Record<string, string> = {
      sm: 'uk-form-small',
      md: '',
      lg: 'uk-form-large',
    };
    const sizeClass = sizeMap[this.size] ?? '';
    const errorClass = this.errorText ? 'uk-form-danger' : '';
    return [base, sizeClass, errorClass].filter(Boolean).join(' ');
  }

  private onInput(e: Event): void {
    this.value = (e.target as HTMLInputElement).value;
    this.dispatchEvent(new CustomEvent('or-input', {
      detail: { value: this.value }, bubbles: true, composed: true,
    }));
  }

  private onBlur(): void {
    this.dispatchEvent(new CustomEvent('or-change', {
      detail: { value: this.value }, bubbles: true, composed: true,
    }));
  }

  setCustomValidity(msg: string): void {
    const native = this.querySelector('input');
    native?.setCustomValidity(msg);
  }

  override render() {
    const inputId = this.name ? `or-input-${this.name}` : `or-input-${this._fallbackId}`;
    return html`
      <div class="uk-form-controls">
        ${this.label ? html`<label class="uk-form-label" for=${inputId}>${this.label}</label>` : nothing}
        <div style="display:flex; align-items:center; gap:8px;">
          <slot name="prefix"></slot>
          <input
            id=${inputId}
            type=${this.type}
            class=${this.inputClasses}
            .value=${this.value}
            placeholder=${this.placeholder}
            ?disabled=${this.disabled}
            ?required=${this.required}
            ?readonly=${this.readonly}
            name=${this.name}
            aria-invalid=${this.errorText ? 'true' : 'false'}
            @input=${this.onInput}
            @blur=${this.onBlur}
          />
          <slot name="suffix"></slot>
        </div>
        ${this.errorText
          ? html`<div class="uk-form-help" style="color:var(--uk-danger, var(--sl-color-danger-500, #d92d20))">${this.errorText}</div>`
          : this.helperText
          ? html`<div class="uk-form-help">${this.helperText}</div>`
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-input': OrInput;
  }
}
