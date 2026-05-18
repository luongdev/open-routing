import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { ifDefined } from 'lit/directives/if-defined.js';
import { ICON_NAMES, type IconName } from './icon-names.js';

@customElement('or-icon')
export class OrIcon extends LitElement {
  @property({ type: String }) name: IconName | '' = '';
  @property({ type: Number }) size = 16;
  @property({ type: String }) label = '';

  override createRenderRoot() { return this; }

  override render() {
    if (this.name && !ICON_NAMES.includes(this.name as IconName)) {
      console.warn(`<or-icon> unknown name: ${this.name}`);
      return nothing;
    }
    return html`
      <uk-icon
        icon=${this.name}
        height=${this.size}
        width=${this.size}
        aria-hidden=${this.label ? 'false' : 'true'}
        aria-label=${ifDefined(this.label || undefined)}
        role=${ifDefined(this.label ? 'img' : undefined)}
      ></uk-icon>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-icon': OrIcon;
  }
}
