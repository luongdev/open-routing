import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { ICON_NAMES, type IconName } from './icon-names.js';

@customElement('or-icon')
export class OrIcon extends LitElement {
  @property({ type: String }) name: IconName | '' = '';
  @property({ type: Number }) size = 16;
  @property({ type: String }) label = '';

  // Light DOM — styles applied via host selector so they work without shadow root
  static override styles = css`
    :host { display: inline-flex; vertical-align: middle; line-height: 0; }
  `;

  override createRenderRoot() { return this; }

  override render() {
    if (this.name && !ICON_NAMES.includes(this.name as IconName)) {
      console.warn(`<or-icon> unknown name: ${this.name}`);
      return null;
    }
    return html`
      <uk-icon
        icon=${this.name}
        height=${this.size}
        width=${this.size}
        aria-hidden=${this.label ? 'false' : 'true'}
        aria-label=${this.label || undefined}
        role=${this.label ? 'img' : undefined}
      ></uk-icon>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-icon': OrIcon;
  }
}
