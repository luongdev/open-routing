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
              role="tab"
              aria-selected=${t.id === this.activeTab ? 'true' : 'false'}
              aria-disabled=${t.disabled ? 'true' : 'false'}
              @click=${(e: Event) => { e.preventDefault(); this.onTab(t.id); }}
            >${t.label}</a>
          </li>
        `)}
      </ul>
      <div class="uk-tab-panels">
        ${this.tabs.map(t => html`
          <div
            role="tabpanel"
            ?hidden=${t.id !== this.activeTab}
            aria-labelledby=${t.id}
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
