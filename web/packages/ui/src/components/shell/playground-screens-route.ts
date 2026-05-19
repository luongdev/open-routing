import { LitElement, html, css, nothing } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import { createMockApiClient } from './playground-mock-client.js';
import { SCREENS } from './playground-screens.js';
import type { ApiClient } from '../../api/client.js';

// Eager imports for all entity components so they render without lazy routes
import '../agents/agent-list.js';
import '../agents/agent-form.js';
import '../agents/agent-detail.js';
import '../skills/skill-list.js';
import '../skills/skill-form.js';
import '../skills/skill-detail.js';
import '../queues/queue-list.js';
import '../queues/queue-form.js';
import '../queues/queue-detail.js';
import '../channels/channel-list.js';
import '../channels/channel-form.js';
import '../channels/channel-detail.js';
import '../adapters/adapter-list.js';
import '../adapters/adapter-form.js';
import '../adapters/adapter-detail.js';
import '../break-reasons/break-reason-list.js';
import '../break-reasons/break-reason-form.js';
import '../break-reasons/break-reason-detail.js';
import '../status/agent-status-list.js';
import '../imports/import-page.js';
import '../imports/import-result.js';
import '../primitives/queue-picker.js';

const GROUPS = Array.from(new Set(SCREENS.map(s => s.group)));
const DEFAULT_SCREEN = SCREENS[0]!.id;

function slugFromPath(): string {
  const m = /^\/playground\/screens\/([^/?#]+)/.exec(window.location.pathname);
  return m?.[1] ?? '';
}

@customElement('or-playground-screens-route')
export class OrPlaygroundScreensRoute extends LitElement {
  static override styles = css`
    :host {
      display: block;
      min-height: 100vh;
      background: var(--background, #f8f7f5);
      color: var(--foreground, #111);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    /* ---- Top bar ---- */
    .topbar {
      display: flex;
      align-items: center;
      gap: 14px;
      padding: 0 24px;
      height: 56px;
      background: var(--card, #fafafa);
      border-bottom: 1px solid var(--border, #e5e5e5);
      flex-shrink: 0;
    }
    .topbar-brand {
      display: flex;
      align-items: center;
      gap: 10px;
      flex: 1;
    }
    .topbar-logo {
      width: 28px;
      height: 28px;
      border-radius: 7px;
      background: linear-gradient(135deg, var(--primary, #f12c3d), color-mix(in oklch, var(--primary, #f12c3d) 60%, oklch(0.50 0.20 15)));
      display: inline-flex;
      align-items: center;
      justify-content: center;
      color: #fff;
      flex-shrink: 0;
    }
    .topbar-title {
      font-weight: 700;
      font-size: 15px;
      letter-spacing: -0.01em;
      line-height: 1.1;
    }
    .topbar-title small {
      display: block;
      font-size: 10px;
      font-weight: 600;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: var(--muted-foreground, #888);
      margin-top: 1px;
    }
    .topbar-back {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 500;
      color: var(--muted-foreground, #888);
      text-decoration: none;
      border: 1px solid var(--border, #e5e5e5);
      background: transparent;
      cursor: pointer;
      transition: background .12s, color .12s;
    }
    .topbar-back:hover {
      background: var(--muted, #f5f5f5);
      color: var(--foreground, #111);
    }
    .topbar-badge {
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--primary, #f12c3d);
      background: color-mix(in oklch, var(--primary, #f12c3d) 10%, transparent);
      padding: 4px 10px;
      border-radius: 99px;
    }

    /* ---- Layout ---- */
    .layout {
      display: grid;
      grid-template-columns: 260px 1fr;
      min-height: calc(100vh - 56px);
    }

    /* ---- Side nav ---- */
    .sidenav {
      background: var(--card, #fafafa);
      border-right: 1px solid var(--border, #e5e5e5);
      overflow-y: auto;
      padding: 12px 0 24px;
    }
    .group-label {
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--muted-foreground, #888);
      padding: 16px 16px 6px;
    }
    .screen-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 8px 12px;
      margin: 1px 8px;
      border-radius: 8px;
      font-size: 13.5px;
      font-weight: 500;
      color: var(--foreground, #111);
      cursor: pointer;
      background: transparent;
      border: none;
      text-align: left;
      width: calc(100% - 16px);
      transition: background .12s, color .12s;
    }
    .screen-item:hover {
      background: var(--muted, #f5f5f5);
    }
    .screen-item--active {
      background: color-mix(in oklch, var(--primary, #f12c3d) 12%, transparent);
      color: var(--primary, #f12c3d);
      font-weight: 600;
    }
    .screen-item--active:hover {
      background: color-mix(in oklch, var(--primary, #f12c3d) 18%, transparent);
    }
    .screen-dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: currentColor;
      opacity: 0.4;
      flex-shrink: 0;
    }
    .screen-item--active .screen-dot {
      opacity: 1;
    }

    /* ---- Content ---- */
    .content {
      overflow-y: auto;
      background: var(--background, #f8f7f5);
    }
    .screen-frame {
      min-height: 100%;
      background: var(--background, #f8f7f5);
      padding: 24px 32px;
      box-sizing: border-box;
    }
    .screen-count {
      font-size: 11px;
      color: var(--muted-foreground, #888);
      padding: 4px 8px 0 16px;
    }
  `;

  @state() private _active = DEFAULT_SCREEN;
  private _client: ApiClient = createMockApiClient();
  private _handlePopState = () => {
    const slug = slugFromPath();
    this._active = slug || DEFAULT_SCREEN;
  };

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    const slug = slugFromPath();
    this._active = slug || DEFAULT_SCREEN;
    window.addEventListener('popstate', this._handlePopState);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener('popstate', this._handlePopState);
  }

  private _navigate(id: string): void {
    this._active = id;
    const url = `/playground/screens/${id}`;
    window.history.pushState({}, '', url);
    window.dispatchEvent(new PopStateEvent('popstate', { state: {} }));
  }

  private _renderNav() {
    return GROUPS.map(group => {
      const items = SCREENS.filter(s => s.group === group);
      return html`
        <div class="group-label">${group}</div>
        ${items.map(screen => html`
          <button
            class="screen-item ${this._active === screen.id ? 'screen-item--active' : ''}"
            @click=${() => this._navigate(screen.id)}
            aria-current=${this._active === screen.id ? 'page' : nothing}
          >
            <span class="screen-dot"></span>
            ${screen.label}
          </button>
        `)}
      `;
    });
  }

  override render() {
    const screen = SCREENS.find(s => s.id === this._active) ?? SCREENS[0]!;
    return html`
      <div class="topbar">
        <div class="topbar-brand">
          <span class="topbar-logo">
            <uk-icon icon="route" height="16" width="16"></uk-icon>
          </span>
          <div class="topbar-title">
            Design System Demo
            <small>Open Routing</small>
          </div>
        </div>
        <span class="topbar-badge">${SCREENS.length} screens</span>
        <a class="topbar-back" href="/playground"
           @click=${(e: Event) => {
             e.preventDefault();
             window.history.pushState({}, '', '/playground');
             window.dispatchEvent(new PopStateEvent('popstate', { state: {} }));
           }}>
          <uk-icon icon="arrow-left" height="14" width="14"></uk-icon>
          Back to Overview
        </a>
      </div>
      <div class="layout">
        <nav class="sidenav" aria-label="Screen navigation">
          <div class="screen-count">${SCREENS.length} screens total</div>
          ${this._renderNav()}
        </nav>
        <main class="content">
          <div class="screen-frame">
            ${screen.render(this._client)}
          </div>
        </main>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-playground-screens-route': OrPlaygroundScreensRoute;
  }
}
