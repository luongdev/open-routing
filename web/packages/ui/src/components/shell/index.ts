// Wave 1 scaffold: shell component stub — full implementation in Wave 2 (06-02).
// Exporting a no-op custom element registers <or-catalog-shell> so index.html renders.
import { LitElement, html } from 'lit';
import { customElement } from 'lit/decorators.js';

@customElement('or-catalog-shell')
export class OrCatalogShell extends LitElement {
  override render() {
    return html`<slot></slot>`;
  }
}
