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

  override createRenderRoot() { return this; }

  private _getMenuItems(): Exclude<DropdownItem, { divider: true }>[] {
    return this.items.filter((i): i is Exclude<DropdownItem, { divider: true }> =>
      !('divider' in i && i.divider) && !i.disabled
    );
  }

  private _onTriggerClick(e: MouseEvent): void {
    if (this.mode !== 'click') return;
    e.stopPropagation();
    this._open = !this._open;
    this._focusIndex = -1;
  }

  private _onMouseEnter(): void {
    if (this.mode === 'hover') {
      this._open = true;
    }
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
    const activeItems = this._getMenuItems();
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        this._focusIndex = Math.min(this._focusIndex + 1, activeItems.length - 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        this._focusIndex = Math.max(this._focusIndex - 1, 0);
        break;
      case 'Enter':
        e.preventDefault();
        if (this._focusIndex >= 0 && this._focusIndex < activeItems.length) {
          this._select(activeItems[this._focusIndex]!.id);
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
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    document.removeEventListener('click', this._onDocumentClick);
  }

  private _select(id: string, disabled?: boolean): void {
    if (disabled) return;
    this._open = false;
    this._focusIndex = -1;
    this.dispatchEvent(new CustomEvent('or-dropdown-select', {
      detail: { id }, bubbles: true, composed: true,
    }));
  }

  private _posClass(): string {
    const alignMap: Record<'left' | 'right' | 'center', string> = {
      left: '',
      right: 'uk-dropdown-right',
      center: 'uk-dropdown-center',
    };
    return alignMap[this.align];
  }

  override render() {
    const activeItems = this._getMenuItems();

    return html`
      <div
        class="uk-position-relative"
        style="display:inline-block"
        @mouseenter=${this._onMouseEnter}
        @mouseleave=${this._onMouseLeave}
        @keydown=${this._onKeydown}
      >
        <div
          role="button"
          tabindex="0"
          aria-haspopup="true"
          aria-expanded=${this._open ? 'true' : 'false'}
          @click=${this._onTriggerClick}
        >
          <slot name="trigger"></slot>
        </div>
        <div
          class=${['uk-dropdown', this._posClass(), this._open ? 'uk-open' : ''].filter(Boolean).join(' ')}
          style=${`margin-top:${this.offset}px;${this._open ? '' : 'display:none'}`}
          role="menu"
        >
          <ul class="uk-nav uk-dropdown-nav" role="presentation">
            ${this.items.map((item, idx) => {
              if ('divider' in item && item.divider) {
                return html`<li class="uk-nav-divider" role="separator"></li>`;
              }
              const i = item as Exclude<DropdownItem, { divider: true }>;
              const activeIdx = activeItems.indexOf(i);
              const isFocused = activeIdx === this._focusIndex;
              return html`
                <li class=${['uk-nav-item', i.disabled ? 'uk-disabled' : ''].filter(Boolean).join(' ')}>
                  <a
                    role="menuitem"
                    tabindex=${i.disabled ? '-1' : '0'}
                    aria-disabled=${i.disabled ? 'true' : 'false'}
                    data-focused=${isFocused ? 'true' : 'false'}
                    style=${i.destructive && !i.disabled ? 'color:var(--uk-danger-f)' : ''}
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
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-dropdown': OrDropdown;
  }
}
