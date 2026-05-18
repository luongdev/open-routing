import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';

// or-dialog uses Shadow DOM so :host styles can manage display/z-index
// without polluting the light DOM. UIkit's .uk-modal CSS class is applied
// to the inner overlay div — this keeps the focus trap and backdrop contained.

const DIALOG_STYLES_ID = 'or-dialog-custom-styles';
function ensureDialogStyles(): void {
  if (document.getElementById(DIALOG_STYLES_ID)) return;
  const style = document.createElement('style');
  style.id = DIALOG_STYLES_ID;
  style.textContent = `
    or-dialog[open] { display: block !important; }
  `;
  document.head.appendChild(style);
}

@customElement('or-dialog')
export class OrDialog extends LitElement {
  static override styles = css`
    :host {
      display: none;
    }
    :host([open]) {
      display: block;
    }
    .or-dialog-overlay {
      position: fixed;
      inset: 0;
      z-index: var(--uk-modal-z-index, 1010);
      overflow-y: auto;
      background-color: color-mix(in srgb, #000 80%, transparent);
      -webkit-backdrop-filter: blur(var(--uk-global-blur, 0px));
      backdrop-filter: blur(var(--uk-global-blur, 0px));
      padding: 1rem;
      display: flex;
      align-items: flex-start;
      justify-content: center;
    }
    .uk-modal-dialog {
      box-sizing: border-box;
      margin: 2rem auto;
      width: var(--or-dialog-width, 32rem);
      max-width: 100%;
      background: var(--uk-background, var(--or-color-card-bg, #fff));
      border-radius: var(--uk-global-radius, 0.5rem);
      border: 1px solid color-mix(in srgb, var(--uk-border, #e5e5e5) 10%, transparent);
      box-shadow: var(--uk-drop-shadow, 0 4px 24px rgba(0,0,0,0.12));
      position: relative;
    }
    :host([size="sm"]) .uk-modal-dialog { --or-dialog-width: 20rem; }
    :host([size="md"]) .uk-modal-dialog { --or-dialog-width: 32rem; }
    :host([size="lg"]) .uk-modal-dialog { --or-dialog-width: 48rem; }
    :host([size="xl"]) .uk-modal-dialog { --or-dialog-width: 64rem; }
    .uk-modal-header {
      padding: 1rem;
      border-bottom: 1px solid var(--uk-border, #e5e5e5);
    }
    .uk-modal-body {
      padding: 1rem;
    }
    .uk-modal-footer {
      padding: 1rem;
      border-top: 1px solid var(--uk-border, #e5e5e5);
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }
  `;

  @property({ type: Boolean, reflect: true }) open = false;
  @property({ type: String, reflect: true }) size: 'sm' | 'md' | 'lg' | 'xl' = 'md';
  @property({ type: Boolean, attribute: 'prevent-close' }) preventClose = false;

  private _previouslyFocused: Element | null = null;

  override connectedCallback(): void {
    super.connectedCallback();
    ensureDialogStyles();
  }

  override updated(changed: Map<string, unknown>): void {
    if (!changed.has('open')) return;
    if (this.open) {
      this._previouslyFocused = document.activeElement;
      this._setupKeyboardHandler();
      this._dispatchOpen();
      // Move focus to first focusable inside on next tick
      setTimeout(() => this._focusFirst(), 0);
    } else {
      this._removeKeyboardHandler();
      // Restore focus to triggering element
      if (this._previouslyFocused && 'focus' in this._previouslyFocused) {
        (this._previouslyFocused as HTMLElement).focus();
      }
      this._previouslyFocused = null;
    }
  }

  private _onKeydown = (e: KeyboardEvent): void => {
    if (e.key === 'Escape' && !this.preventClose) {
      this.close();
    }
  };

  private _setupKeyboardHandler(): void {
    document.addEventListener('keydown', this._onKeydown);
  }

  private _removeKeyboardHandler(): void {
    document.removeEventListener('keydown', this._onKeydown);
  }

  private _focusFirst(): void {
    if (!this.shadowRoot) return;
    const focusable = this.shadowRoot.querySelector<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    focusable?.focus();
  }

  private _dispatchOpen(): void {
    this.dispatchEvent(new CustomEvent('or-dialog-open', { bubbles: true, composed: true }));
  }

  close(): void {
    this.open = false;
    this.dispatchEvent(new CustomEvent('or-dialog-close', { bubbles: true, composed: true }));
  }

  private _onOverlayClick(e: Event): void {
    if (this.preventClose) return;
    // Only close when clicking the overlay backdrop (not the dialog itself)
    if (e.target === e.currentTarget) {
      this.close();
    }
  }

  override render() {
    return html`
      <div
        class="or-dialog-overlay"
        role="dialog"
        aria-modal="true"
        @click=${this._onOverlayClick}
      >
        <div class="uk-modal-dialog" data-size=${this.size}>
          <header class="uk-modal-header"><slot name="header"></slot></header>
          <div class="uk-modal-body"><slot></slot></div>
          <footer class="uk-modal-footer"><slot name="footer"></slot></footer>
        </div>
      </div>
    `;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._removeKeyboardHandler();
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-dialog': OrDialog;
  }
}
