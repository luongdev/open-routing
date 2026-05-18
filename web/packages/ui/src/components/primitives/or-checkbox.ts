import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('or-checkbox')
export class OrCheckbox extends LitElement {
  @property({ type: Boolean, reflect: true }) checked = false;
  @property({ type: Boolean }) disabled = false;
  @property({ type: Boolean }) indeterminate = false;
  @property({ type: String }) name = '';
  @property({ type: String }) value = '';
  @property({ type: String }) label = '';
  @property({ type: String, attribute: 'helper-text' }) helperText = '';

  override createRenderRoot() { return this; }

  override updated() {
    // indeterminate is not an HTML attribute — must be set as a DOM property after render
    const input = this.querySelector('input');
    if (input) input.indeterminate = this.indeterminate;
  }

  private _onChange(e: Event): void {
    this.checked = (e.target as HTMLInputElement).checked;
    this.dispatchEvent(new CustomEvent('or-change', {
      detail: { checked: this.checked }, bubbles: true, composed: true,
    }));
  }

  override render() {
    const id = `or-checkbox-${this.name || crypto.randomUUID().slice(0, 8)}`;
    return html`
      <label class="uk-flex uk-flex-middle" style="gap:8px; cursor:pointer;" for=${id}>
        <input
          id=${id}
          type="checkbox"
          class="uk-checkbox"
          .checked=${this.checked}
          ?disabled=${this.disabled}
          name=${this.name}
          value=${this.value}
          @change=${this._onChange}
        />
        ${this.label ? html`<span>${this.label}</span>` : nothing}
      </label>
      ${this.helperText ? html`<div class="uk-form-help">${this.helperText}</div>` : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-checkbox': OrCheckbox;
  }
}
