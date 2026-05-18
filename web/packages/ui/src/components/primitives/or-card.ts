import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('or-card')
export class OrCard extends LitElement {
  @property({ type: Boolean }) accessor hoverable = false;
  @property({ type: String }) accessor variant: 'default' | 'primary' | 'secondary' = 'default';
  @property({ type: String }) accessor padding: 'sm' | 'md' | 'lg' = 'md';

  override createRenderRoot() { return this; }

  private get classes(): string {
    return ['uk-card', `uk-card-${this.variant}`,
      this.padding === 'sm' ? 'uk-card-small' : this.padding === 'lg' ? 'uk-card-large' : '',
      this.hoverable ? 'uk-card-hover' : '',
    ].filter(Boolean).join(' ');
  }

  override render() {
    return html`
      <div class=${this.classes}>
        <div class="uk-card-header"><slot name="header"></slot></div>
        <div class="uk-card-body"><slot></slot></div>
        <div class="uk-card-footer"><slot name="footer"></slot></div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-card': OrCard;
  }
}
