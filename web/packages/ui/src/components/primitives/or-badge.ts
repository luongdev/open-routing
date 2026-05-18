import { LitElement, html, nothing, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';

export type BadgeVariant = 'default' | 'success' | 'warning' | 'destructive' | 'info' | 'primary';

// uk-label-pill and uk-label-dot are absent from Frankenstyle v0.3.8 —
// injecting custom styles directly into the document once per class definition.
// Light DOM means :host pseudo-class is unavailable; inline styles on the
// host element would require the same injection anyway. The injection is
// idempotent (checked by marker id) so multiple instances are safe.
const BADGE_STYLES_ID = 'or-badge-custom-styles';
function ensureBadgeStyles(): void {
  if (document.getElementById(BADGE_STYLES_ID)) return;
  const style = document.createElement('style');
  style.id = BADGE_STYLES_ID;
  style.textContent = `
    .uk-label-pill { border-radius: 9999px; }
    .uk-label-dot {
      display: inline-block;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      padding: 0;
      vertical-align: middle;
    }
  `;
  document.head.appendChild(style);
}

@customElement('or-badge')
export class OrBadge extends LitElement {
  static override styles = css``;

  @property({ type: String }) variant: BadgeVariant = 'default';
  @property({ type: Boolean }) pill = false;
  @property({ type: Boolean }) dot = false;

  override createRenderRoot() { return this; }

  override connectedCallback(): void {
    super.connectedCallback();
    ensureBadgeStyles();
  }

  private get classes(): string {
    return [
      'uk-label',
      this.variant !== 'default' ? `uk-label-${this.variant}` : '',
      this.pill ? 'uk-label-pill' : '',
      this.dot ? 'uk-label-dot' : '',
    ].filter(Boolean).join(' ');
  }

  override render() {
    return html`
      <span class=${this.classes} role="status">
        ${this.dot ? nothing : html`<slot></slot>`}
      </span>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-badge': OrBadge;
  }
}
