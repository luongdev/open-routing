import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('or-table')
export class OrTable extends LitElement {
  @property({ type: Boolean }) striped = false;
  @property({ type: Boolean }) hover = false;
  @property({ type: Boolean }) divider = false;
  @property({ type: Boolean }) small = false;
  @property({ type: Boolean }) responsive = false;

  override createRenderRoot() { return this; }

  private get classes(): string {
    return [
      'uk-table',
      this.striped && 'uk-table-striped',
      this.hover && 'uk-table-hover',
      this.divider && 'uk-table-divider',
      this.small && 'uk-table-small',
    ].filter(Boolean).join(' ');
  }

  override render() {
    const inner = html`
      <table class=${this.classes}>
        <thead><slot name="head"></slot></thead>
        <tbody><slot></slot></tbody>
      </table>
    `;
    return this.responsive
      ? html`<div class="uk-overflow-auto">${inner}</div>`
      : inner;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-table': OrTable;
  }
}
