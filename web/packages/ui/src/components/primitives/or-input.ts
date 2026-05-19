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
  /** Lucide icon name rendered inside the input, before the text. */
  @property({ type: String, attribute: 'prefix-icon' }) prefixIcon = '';
  /** Lucide icon name rendered inside the input, after the text. */
  @property({ type: String, attribute: 'suffix-icon' }) suffixIcon = '';

  private readonly _fallbackId = Math.random().toString(36).slice(2, 10);

  // Light DOM — Frankenstyle uk-input and uk-form-* classes resolve from global stylesheet
  override createRenderRoot() { return this; }

  private get inputClasses(): string {
    const base = 'uk-input';
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
    const hasPrefix = !!this.prefixIcon;
    const hasSuffix = !!this.suffixIcon;
    // Input must reserve room for the overlaid icons via padding so the text
    // doesn't slide under them. Using inline style keeps this self-contained.
    const inputStyle = `${hasPrefix ? 'padding-left:34px;' : ''}${hasSuffix ? 'padding-right:34px;' : ''}`;

    return html`
      <div class="uk-form-controls">
        ${this.label ? html`<label class="uk-form-label" for=${inputId}>${this.label}</label>` : nothing}
        <div style="position:relative;">
          ${hasPrefix ? html`
            <uk-icon
              icon=${this.prefixIcon}
              height="14"
              width="14"
              style="position:absolute; left:11px; top:50%; transform:translateY(-50%); color:var(--muted-foreground); pointer-events:none;"
              aria-hidden="true"
            ></uk-icon>
          ` : nothing}
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
            style=${inputStyle}
            aria-invalid=${this.errorText ? 'true' : 'false'}
            @input=${this.onInput}
            @blur=${this.onBlur}
          />
          ${hasSuffix ? html`
            <uk-icon
              icon=${this.suffixIcon}
              height="14"
              width="14"
              style="position:absolute; right:11px; top:50%; transform:translateY(-50%); color:var(--muted-foreground); pointer-events:none;"
              aria-hidden="true"
            ></uk-icon>
          ` : nothing}
        </div>
        ${this.errorText
          ? html`<div class="uk-form-help" style="color:var(--destructive)">${this.errorText}</div>`
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
