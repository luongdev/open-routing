import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';

export type SelectOption = { value: string; label: string; disabled?: boolean };

@customElement('or-select')
export class OrSelect extends LitElement {
  @property({ type: String }) accessor value = '';
  @property({ type: String }) accessor name = '';
  @property({ type: Boolean }) accessor disabled = false;
  @property({ type: Boolean }) accessor required = false;
  @property({ type: String }) accessor label = '';
  @property({ type: String, attribute: 'helper-text' }) accessor helperText = '';
  @property({ type: String, attribute: 'error-text' }) accessor errorText = '';
  @property({ type: String }) accessor placeholder = '';
  @property({ type: String }) accessor size: 'sm' | 'md' | 'lg' = 'md';
  @property({ type: Array }) accessor options: SelectOption[] = [];

  override createRenderRoot() { return this; }

  private onChange(e: Event): void {
    this.value = (e.target as HTMLSelectElement).value;
    this.dispatchEvent(new CustomEvent('or-change', {
      detail: { value: this.value }, bubbles: true, composed: true,
    }));
  }

  private get classes(): string {
    return ['uk-select',
      this.size !== 'md' && `uk-form-${this.size}`,
      this.errorText && 'uk-form-danger',
    ].filter(Boolean).join(' ');
  }

  override render() {
    return html`
      <div class="uk-form-controls">
        ${this.label ? html`<label class="uk-form-label">${this.label}</label>` : nothing}
        <select
          class=${this.classes}
          .value=${this.value}
          ?disabled=${this.disabled}
          ?required=${this.required}
          name=${this.name}
          @change=${this.onChange}
        >
          ${this.placeholder ? html`<option value="" disabled selected hidden>${this.placeholder}</option>` : nothing}
          ${this.options.map(o => html`<option value=${o.value} ?disabled=${o.disabled ?? false}>${o.label}</option>`)}
          <slot></slot>
        </select>
        ${this.errorText
          ? html`<div class="uk-form-help uk-text-danger">${this.errorText}</div>`
          : this.helperText
          ? html`<div class="uk-form-help">${this.helperText}</div>`
          : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-select': OrSelect;
  }
}
