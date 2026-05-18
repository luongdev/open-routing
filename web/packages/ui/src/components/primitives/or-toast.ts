import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import type { NotifyVariant } from './notify.js';

@customElement('or-toast')
export class OrToast extends LitElement {
  @property({ type: String }) accessor variant: NotifyVariant = 'info';
  @property({ type: String, attribute: 'auto-dismiss' }) accessor autoDismiss = '';

  override createRenderRoot() { return this; }

  override connectedCallback(): void {
    super.connectedCallback();
    const ms = parseInt(this.autoDismiss, 10);
    if (ms > 0) {
      setTimeout(() => {
        if (this.parentNode) this.parentNode.removeChild(this);
      }, ms);
    }
  }

  private get variantClass(): string {
    const map: Record<NotifyVariant, string> = {
      info: 'uk-notification-message-info',
      success: 'uk-notification-message-success',
      warning: 'uk-notification-message-warning',
      destructive: 'uk-notification-message-danger',
    };
    return map[this.variant] ?? '';
  }

  override render() {
    return html`
      <div
        class=${['uk-notification-message', this.variantClass].join(' ')}
        role="alert"
      >
        <slot></slot>
        <button
          type="button"
          class="uk-notification-close"
          aria-label="Close"
          @click=${() => { if (this.parentNode) this.parentNode.removeChild(this); }}
        >×</button>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-toast': OrToast;
  }
}
