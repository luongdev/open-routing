import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';

export type ButtonVariant = 'default' | 'primary' | 'secondary' | 'ghost' | 'destructive';
export type ButtonSize = 'sm' | 'md' | 'lg';

@customElement('or-button')
export class OrButton extends LitElement {
  @property({ type: String }) variant: ButtonVariant = 'default';
  @property({ type: String }) size: ButtonSize = 'md';
  @property({ type: Boolean }) disabled = false;
  @property({ type: Boolean }) loading = false;
  @property({ type: String }) type: 'button' | 'submit' | 'reset' = 'button';

  // Light DOM — Frankenstyle uk-button classes resolve from global stylesheet
  override createRenderRoot() {
    return this;
  }

  private get classNames(): string {
    const base = 'uk-button';
    // Frankenstyle uses uk-button-{variant} pattern; 'destructive' maps to 'danger'
    const variantMap: Record<ButtonVariant, string> = {
      default: 'uk-button-default',
      primary: 'uk-button-primary',
      secondary: 'uk-button-secondary',
      ghost: 'uk-button-ghost',
      destructive: 'uk-button-danger',
    };
    const variantClass = variantMap[this.variant];
    // Frankenstyle size classes: small → uk-button-small, large → uk-button-large
    const sizeMap: Record<ButtonSize, string> = {
      sm: 'uk-button-small',
      md: '',
      lg: 'uk-button-large',
    };
    const sizeClass = sizeMap[this.size];
    return [base, variantClass, sizeClass].filter(Boolean).join(' ');
  }

  private onClick(e: Event): void {
    if (this.loading) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  }

  override render() {
    return html`
      <button
        type=${this.type}
        class=${this.classNames}
        ?disabled=${this.disabled || this.loading}
        @click=${this.onClick}
        aria-busy=${this.loading ? 'true' : 'false'}
      >
        ${this.loading
          ? html`<svg class="uk-spinner" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" stroke-dasharray="60" stroke-linecap="round"><animateTransform attributeName="transform" type="rotate" from="0 12 12" to="360 12 12" dur="1s" repeatCount="indefinite"/></circle></svg>`
          : nothing}
        <slot></slot>
      </button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-button': OrButton;
  }
}
