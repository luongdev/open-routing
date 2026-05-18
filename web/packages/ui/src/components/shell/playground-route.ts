import { LitElement, html, css, type TemplateResult } from 'lit';
import { customElement } from 'lit/decorators.js';
import { iconSlot } from './slots/icon.js';
import { switchCheckboxSlot } from './slots/switch-checkbox.js';
import { buttonSlot } from './slots/button.js';
import { inputSlot } from './slots/input.js';
import { selectSlot } from './slots/select.js';
import { cardSlot } from './slots/card.js';
import { badgeSlot } from './slots/badge.js';
import { tabsSlot } from './slots/tabs.js';

// <uk-theme-switcher> does not exist in Frankenstyle v0.3.8 — implemented inline.
// Dispatches 'open-routing:theme-change' to the parent catalog-shell which owns
// the theme state. This avoids Shadow DOM piercing while keeping the playground
// self-contained as a dev tool.

interface SlotDef {
  id: string;
  label: string;
  plan: string;
  render?: () => TemplateResult;
}

const SLOT_CONTENT: Partial<Record<string, TemplateResult>> = {
  button: buttonSlot,
  input: inputSlot,
  select: selectSlot,
  card: cardSlot,
  icon: iconSlot,
  'switch-checkbox': switchCheckboxSlot,
};

const SLOTS: readonly SlotDef[] = [
  { id: 'button',          label: 'Button',            plan: '07-w0-10' },
  { id: 'input',           label: 'Input',             plan: '07-w0-11' },
  { id: 'select',          label: 'Select',            plan: '07-w0-12' },
  { id: 'card',            label: 'Card',              plan: '07-w0-13' },
  { id: 'badge',           label: 'Badge',             plan: '07-w0-14', render: badgeSlot },
  { id: 'dialog',          label: 'Dialog',            plan: '07-w0-15' },
  { id: 'tabs',            label: 'Tabs',              plan: '07-w0-16', render: tabsSlot },
  { id: 'icon',            label: 'Icon',              plan: '07-w0-17' },
  { id: 'switch-checkbox', label: 'Switch + Checkbox', plan: '07-w0-18' },
  { id: 'table',           label: 'Table',             plan: '07-w0-19' },
  { id: 'toast',           label: 'Toast',             plan: '07-w0-20' },
  { id: 'dropdown',        label: 'Dropdown',          plan: '07-w0-21' },
  { id: 'sidebar',         label: 'Sidebar',           plan: '07-w0-22' },
];

@customElement('or-playground-route')
export class OrPlaygroundRoute extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 32px;
      background: var(--background, var(--or-color-app-bg, #fff));
      color: var(--foreground, var(--or-color-text-body, #404040));
      min-height: 100vh;
    }
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 32px;
    }
    h1 {
      margin: 0;
      font-size: 24px;
      font-weight: 700;
    }
    .theme-toggle {
      display: flex;
      gap: 8px;
    }
    .theme-btn {
      padding: 6px 16px;
      border: 1px solid var(--border, var(--or-color-divider, #e5e5e5));
      border-radius: 6px;
      background: var(--card, var(--or-color-card-bg, #fff));
      color: var(--foreground, var(--or-color-text-body, #404040));
      cursor: pointer;
      font-size: 13px;
      font-weight: 500;
    }
    .theme-btn:hover {
      background: var(--muted, var(--or-color-row-hover, #fafafa));
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
      gap: 24px;
    }
    .slot {
      padding: 24px;
      border: 1px solid var(--border, var(--or-color-card-border, #e5e5e5));
      border-radius: 12px;
      background: var(--card, var(--or-color-card-bg, #fff));
    }
    .slot h3 {
      margin: 0 0 16px;
      font-size: 14px;
      font-weight: 600;
      color: var(--muted-foreground, var(--or-color-text-muted, #737373));
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .playground-slot {
      color: var(--muted-foreground, var(--or-color-text-muted, #737373));
      font-style: italic;
      padding: 24px;
      text-align: center;
      border: 2px dashed var(--border, var(--or-color-card-border, #e5e5e5));
      border-radius: 8px;
    }
  `;

  private _dispatchThemeChange(theme: 'ember-light' | 'ember-dark'): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:theme-change', {
        detail: { theme },
        bubbles: true,
        composed: true,
      })
    );
  }

  override render() {
    return html`
      <div class="header">
        <h1>UI Playground</h1>
        <div class="theme-toggle" role="group" aria-label="Theme switcher">
          <button
            class="theme-btn"
            @click=${() => this._dispatchThemeChange('ember-light')}
          >Ember Light</button>
          <button
            class="theme-btn"
            @click=${() => this._dispatchThemeChange('ember-dark')}
          >Ember Dark</button>
        </div>
      </div>
      <div class="grid">
        ${SLOTS.map(
          (s) => html`
            <section class="slot" data-component=${s.id}>
              <h3>${s.label}</h3>
              ${SLOT_CONTENT[s.id]
                ? SLOT_CONTENT[s.id]
                : s.render
                ? s.render()
                : html`<div class="playground-slot" data-component=${s.id}>${s.plan} pending</div>`}
            </section>
          `
        )}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-playground-route': OrPlaygroundRoute;
  }
}
