import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';

export type Tab = { id: string; label: string; disabled?: boolean };

@customElement('or-tabs')
export class OrTabs extends LitElement {
  @property({ type: Array }) tabs: Tab[] = [];
  @property({ type: String, attribute: 'active-tab' }) activeTab = '';

  override createRenderRoot() { return this; }

  private onTab(id: string): void {
    if (this.tabs.find(t => t.id === id)?.disabled) return;
    if (id === this.activeTab) return;
    this.activeTab = id;
    this.dispatchEvent(new CustomEvent('or-tab-change', {
      detail: { id }, bubbles: true, composed: true,
    }));
  }

  override render() {
    return html`
      <ul class="uk-tab" role="tablist">
        ${this.tabs.map(t => html`
          <li role="presentation" class=${t.id === this.activeTab ? 'uk-active' : ''}>
            <a
              id="tab-${t.id}"
              role="tab"
              tabindex=${t.disabled ? '-1' : '0'}
              aria-selected=${t.id === this.activeTab ? 'true' : 'false'}
              aria-disabled=${t.disabled ? 'true' : 'false'}
              aria-controls="panel-${t.id}"
              @click=${(e: Event) => { e.preventDefault(); this.onTab(t.id); }}
              @keydown=${(e: KeyboardEvent) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  this.onTab(t.id);
                }
              }}
            >${t.label}</a>
          </li>
        `)}
      </ul>
      <div class="uk-tab-panels">
        ${this.tabs.map(t => html`
          <div
            id="panel-${t.id}"
            role="tabpanel"
            ?hidden=${t.id !== this.activeTab}
            aria-labelledby="tab-${t.id}"
          ><slot name=${`tab-${t.id}`}></slot></div>
        `)}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-tabs': OrTabs;
  }
}
