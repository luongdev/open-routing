import { LitElement, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

export type DropdownItem =
  | { id: string; label: string; icon?: string; destructive?: boolean; disabled?: boolean; divider?: false }
  | { divider: true };

@customElement('or-dropdown')
export class OrDropdown extends LitElement {
  @property({ type: String }) align: 'left' | 'right' | 'center' = 'left';
  @property({ type: String }) mode: 'click' | 'hover' = 'click';
  @property({ type: Number }) offset = 8;
  @property({ type: Array }) items: DropdownItem[] = [];

  @state() private _open = false;
  @state() private _focusIndex = -1;

  // Light DOM — Frankenstyle uk-dropdown classes resolve from global stylesheet.
  // Slots are not used: slot projection requires Shadow DOM, and this component
  // must inherit Frankenstyle CSS from the page. The trigger is the host element
  // itself (tabindex, keyboard, click), and children placed inside or-dropdown
  // in light DOM serve as the visual trigger (managed by the parent template).
  override createRenderRoot() { return this; }

  private _getActiveItems(): Exclude<DropdownItem, { divider: true }>[] {
    return this.items.filter(
      (i): i is Exclude<DropdownItem, { divider: true }> =>
        !('divider' in i && i.divider) && !i.disabled
    );
  }

  private _onHostClick(e: MouseEvent): void {
    if (this.mode !== 'click') return;
    const popup = this.querySelector('.or-dropdown-popup');
    if (popup && popup.contains(e.target as Node)) return;
    e.stopPropagation();
    this._open = !this._open;
    this._focusIndex = -1;
  }

  private _onMouseEnter(): void {
    if (this.mode === 'hover') this._open = true;
  }

  private _onMouseLeave(): void {
    if (this.mode === 'hover') {
      this._open = false;
      this._focusIndex = -1;
    }
  }

  private _onKeydown(e: KeyboardEvent): void {
    if (!this._open && (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown')) {
      e.preventDefault();
      this._open = true;
      this._focusIndex = 0;
      return;
    }
    if (!this._open) return;
    const active = this._getActiveItems();
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        this._focusIndex = Math.min(this._focusIndex + 1, active.length - 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        this._focusIndex = Math.max(this._focusIndex - 1, 0);
        break;
      case 'Enter':
        e.preventDefault();
        if (this._focusIndex >= 0 && this._focusIndex < active.length) {
          this._select(active[this._focusIndex]!.id);
        }
        break;
      case 'Escape':
        e.preventDefault();
        this._open = false;
        this._focusIndex = -1;
        break;
    }
  }

  private _onDocumentClick = (e: MouseEvent): void => {
    if (!this.contains(e.target as Node)) {
      this._open = false;
      this._focusIndex = -1;
    }
  };

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('click', this._onDocumentClick);
    this.setAttribute('tabindex', '0');
    this.setAttribute('role', 'combobox');
    this.style.setProperty('position', 'relative');
    this.style.setProperty('display', 'inline-block');
    this.addEventListener('click', this._onHostClick);
    this.addEventListener('mouseenter', this._onMouseEnter);
    this.addEventListener('mouseleave', this._onMouseLeave);
    this.addEventListener('keydown', this._onKeydown);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._onDocumentClick);
    this.removeEventListener('click', this._onHostClick);
    this.removeEventListener('mouseenter', this._onMouseEnter);
    this.removeEventListener('mouseleave', this._onMouseLeave);
    this.removeEventListener('keydown', this._onKeydown);
  }

  private _select(id: string, disabled?: boolean): void {
    if (disabled) return;
    this._open = false;
    this._focusIndex = -1;
    this.dispatchEvent(new CustomEvent('or-dropdown-select', {
      detail: { id }, bubbles: true, composed: true,
    }));
  }

  override render() {
    const active = this._getActiveItems();
    const alignStyle =
      this.align === 'right'
        ? 'right:0;left:auto;'
        : this.align === 'center'
        ? 'left:50%;transform:translateX(-50%);'
        : '';

    return html`
      <div
        class=${'or-dropdown-popup uk-dropdown' + (this._open ? ' uk-open' : '')}
        style=${`position:absolute;top:100%;margin-top:${this.offset}px;${alignStyle}min-width:180px;z-index:1020;${this._open ? '' : 'display:none'}`}
        role="menu"
        aria-hidden=${this._open ? 'false' : 'true'}
      >
        <ul class="uk-nav uk-dropdown-nav" role="presentation">
          ${this.items.map((item) => {
            if ('divider' in item && item.divider) {
              return html`<li class="uk-nav-divider" role="separator"></li>`;
            }
            const i = item as Exclude<DropdownItem, { divider: true }>;
            const activeIdx = active.indexOf(i);
            const isFocused = activeIdx === this._focusIndex;
            return html`
              <li class=${['uk-nav-item', i.disabled ? 'uk-disabled' : ''].filter(Boolean).join(' ')}>
                <a
                  role="menuitem"
                  tabindex=${i.disabled ? '-1' : '0'}
                  aria-disabled=${i.disabled ? 'true' : 'false'}
                  style=${i.destructive && !i.disabled ? 'color:var(--uk-danger-f)' : ''}
                  class=${isFocused ? 'uk-active' : ''}
                  @click=${(e: MouseEvent) => { e.stopPropagation(); this._select(i.id, i.disabled); }}
                >
                  ${i.icon ? html`<or-icon name=${i.icon} size="14" style="margin-inline-end:6px;vertical-align:middle"></or-icon>` : nothing}
                  ${i.label}
                </a>
              </li>
            `;
          })}
        </ul>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-dropdown': OrDropdown;
  }
}
