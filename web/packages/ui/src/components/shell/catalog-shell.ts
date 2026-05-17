// Phase 6 Plan 03: <or-catalog-shell> — page-level shell with @lit-labs/router,
// sidebar nav (8 entries per D6-11), top bar with theme toggle (D6-19),
// UUIDv7 route guard (D6-13). This is the mounting point for the admin SPA.
//
// D6-20: Theme CSS custom properties applied to THIS host element (this.style).
// Applying to the shell host (not the document root) is required for Phase 7 Shadow DOM isolation.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Routes } from '@lit-labs/router';
import { orLight, orDark, orBrand, ALL_TOKEN_KEYS } from '../../themes/index.js';
import type { ThemeName } from '../../themes/index.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/button-group/button-group.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/drawer/drawer.js';

/** UUIDv7 regex per D6-13 and CONTEXT.md. Client-side UX nicety; server is authoritative. */
const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** Sidebar nav entry definition */
interface NavEntry {
  key: string;
  label: string;
  icon: string;
  path: string;
}

/** All 8 sidebar nav entries per D6-11. Divider is a special entry with key '__divider__'. */
const NAV_ENTRIES: readonly NavEntry[] = [
  { key: 'agents',        label: 'Agents',        icon: 'people-fill',       path: '/orgs/{orgId}/agents' },
  { key: 'skills',        label: 'Skills',         icon: 'tag-fill',          path: '/orgs/{orgId}/skills' },
  { key: 'queues',        label: 'Queues',         icon: 'funnel-fill',       path: '/orgs/{orgId}/queues' },
  { key: 'channels',      label: 'Channels',       icon: 'broadcast-pin',     path: '/orgs/{orgId}/channels' },
  { key: 'adapters',      label: 'Adapters',       icon: 'plug-fill',         path: '/orgs/{orgId}/adapters' },
  { key: 'break-reasons', label: 'Break Reasons',  icon: 'pause-circle-fill', path: '/orgs/{orgId}/break-reasons' },
  { key: '__divider__',   label: '',               icon: '',                  path: '' },
  { key: 'imports',       label: 'Bulk Import',    icon: 'upload',            path: '/orgs/{orgId}/imports/new' },
  { key: 'status',        label: 'Agent Status',   icon: 'circle-fill',       path: '/orgs/{orgId}/agents/status' },
] as const;

/** Entries that always show regardless of modules filter (divider, imports, status). */
const ALWAYS_VISIBLE_KEYS = new Set(['__divider__', 'imports', 'status']);

/** Theme token lookup table. */
const _THEME_TOKENS: Record<ThemeName, Record<string, string>> = {
  'or-light': orLight,
  'or-dark': orDark,
  'or-brand': orBrand,
};

/** Valid theme names for localStorage restore guard. */
const VALID_THEME_NAMES: readonly ThemeName[] = ['or-light', 'or-dark', 'or-brand'];

/**
 * <or-catalog-shell> — top-level admin SPA shell.
 *
 * Attributes:
 *   org-id    — current organization UUID v7 (read from URL path by the app)
 *   theme     — 'or-light' | 'or-dark' | 'or-brand' (default 'or-light')
 *   modules   — comma-separated entity keys to show in sidebar (empty = show all)
 *   locale    — 'en' | 'vi' (default 'en')
 *
 * D6-20: CSS custom properties are applied to THIS element's inline style (this.style),
 * enabling Shadow DOM isolation for Phase 7 embed reuse.
 */
@customElement('or-catalog-shell')
export class OrCatalogShell extends LitElement {
  static styles = css`
    :host {
      display: flex;
      flex-direction: column;
      height: 100vh;
    }
    .topbar {
      height: 56px;
      display: flex;
      align-items: center;
      background: var(--or-color-topbar-bg, #fff);
      border-bottom: 1px solid var(--or-color-divider, #e5e5e5);
      padding: 0 8px;
      flex-shrink: 0;
    }
    .wordmark {
      font-weight: 600;
      margin-left: 8px;
      font-size: 16px;
    }
    .topbar-spacer {
      flex: 1;
    }
    .org-chip {
      font-size: 12px;
      font-family: monospace;
      background: var(--or-color-code-bg, #f5f5f5);
      color: var(--or-color-code-fg, #1a575f);
      padding: 2px 8px;
      border-radius: 4px;
      cursor: pointer;
      margin: 0 8px;
    }
    .body {
      display: flex;
      flex: 1;
      overflow: hidden;
    }
    .sidebar {
      width: 248px;
      background: var(--or-color-sidebar-bg, #f5f5f5);
      border-right: 1px solid var(--or-color-divider, #e5e5e5);
      overflow-y: auto;
      flex-shrink: 0;
    }
    .nav-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      cursor: pointer;
      color: var(--or-color-text-body, #404040);
      text-decoration: none;
      font-size: 14px;
      border: none;
      background: none;
      width: 100%;
      text-align: left;
    }
    .nav-item:hover {
      background: var(--or-color-row-hover, #fafafa);
    }
    .nav-divider {
      height: 1px;
      background: var(--or-color-divider, #e5e5e5);
      margin: 8px 16px;
    }
    .content {
      flex: 1;
      overflow-y: auto;
      padding: 32px 32px 24px;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String }) theme: ThemeName = 'or-light';
  @property({ type: String }) modules = '';
  @property({ type: String }) locale: 'en' | 'vi' = 'en';

  @state() private _sidebarOpen = false;

  /** @lit-labs/router Routes — outlet() MUST stay in this component's render() per Pitfall 7. */
  private _routes = new Routes(this, [
    {
      path: '/',
      render: () => html`<or-org-picker></or-org-picker>`,
    },
    {
      // Route guard: redirect to / if org_id is not valid UUIDv7 (D6-13)
      path: '/orgs/:org_id/*',
      enter: async ({ org_id }: Record<string, string | undefined>) => {
        if (!UUIDV7_PATTERN.test(org_id ?? '')) {
          this._routes.goto('/');
          return false;
        }
        return true;
      },
    },
    {
      path: '/orgs/:org_id/agents',
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<div data-route="agents" data-org-id="${org_id}">Agents list</div>`,
    },
    {
      path: '/orgs/:org_id/agents/new',
      render: () => html`<div data-route="agents-new">Agent create</div>`,
    },
    {
      path: '/orgs/:org_id/agents/:id',
      render: () => html`<div data-route="agent-detail">Agent detail</div>`,
    },
    {
      path: '/orgs/:org_id/agents/:id/status',
      render: () => html`<div data-route="agent-status">Agent status</div>`,
    },
    {
      path: '/orgs/:org_id/skills',
      render: () => html`<div data-route="skills">Skills list</div>`,
    },
    {
      path: '/orgs/:org_id/skills/new',
      render: () => html`<div data-route="skills-new">Skill create</div>`,
    },
    {
      path: '/orgs/:org_id/skills/:id',
      render: () => html`<div data-route="skill-detail">Skill detail</div>`,
    },
    {
      path: '/orgs/:org_id/queues',
      render: () => html`<div data-route="queues">Queues list</div>`,
    },
    {
      path: '/orgs/:org_id/queues/new',
      render: () => html`<div data-route="queues-new">Queue create</div>`,
    },
    {
      path: '/orgs/:org_id/queues/:id',
      render: () => html`<div data-route="queue-detail">Queue detail</div>`,
    },
    {
      path: '/orgs/:org_id/channels',
      render: () => html`<div data-route="channels">Channels list</div>`,
    },
    {
      path: '/orgs/:org_id/channels/new',
      render: () => html`<div data-route="channels-new">Channel create</div>`,
    },
    {
      path: '/orgs/:org_id/channels/:id',
      render: () => html`<div data-route="channel-detail">Channel detail</div>`,
    },
    {
      path: '/orgs/:org_id/adapters',
      render: () => html`<div data-route="adapters">Adapters list</div>`,
    },
    {
      path: '/orgs/:org_id/adapters/new',
      render: () => html`<div data-route="adapters-new">Adapter create</div>`,
    },
    {
      path: '/orgs/:org_id/adapters/:id',
      render: () => html`<div data-route="adapter-detail">Adapter detail</div>`,
    },
    {
      path: '/orgs/:org_id/break-reasons',
      render: () => html`<div data-route="break-reasons">Break Reasons list</div>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/new',
      render: () => html`<div data-route="break-reasons-new">Break Reason create</div>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/:id',
      render: () => html`<div data-route="break-reason-detail">Break Reason detail</div>`,
    },
    {
      path: '/orgs/:org_id/imports/new',
      render: () => html`<div data-route="imports-new">Import page</div>`,
    },
    {
      path: '/orgs/:org_id/imports/:id',
      render: () => html`<div data-route="import-result">Import result</div>`,
    },
  ]);

  override connectedCallback(): void {
    super.connectedCallback();
    // Restore theme from localStorage on connect (D6-19)
    const saved = localStorage.getItem('or-theme') as ThemeName | null;
    if (saved && (VALID_THEME_NAMES as readonly string[]).includes(saved)) {
      this.theme = saved as ThemeName;
    }
    this._applyTheme();
  }

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('theme')) {
      this._applyTheme();
    }
  }

  /**
   * Apply theme CSS custom properties to THIS element's inline style (D6-20).
   * Sets on shell host element — CSS cascade reaches all nested Shadow DOM children.
   * Clears all prior tokens first to avoid stale overrides across theme switches.
   */
  private _applyTheme(): void {
    // Clear all prior theme tokens (across all themes)
    for (const key of ALL_TOKEN_KEYS) {
      this.style.removeProperty(key);
    }
    // Apply resolved token map for current theme
    const tokens = _THEME_TOKENS[this.theme] ?? {};
    for (const [key, val] of Object.entries(tokens)) {
      this.style.setProperty(key, val);
    }
    // Persist named theme to localStorage
    localStorage.setItem('or-theme', this.theme);
  }

  /** Resolve sidebar nav path for the current orgId. */
  private _navPath(template: string): string {
    return template.replace('{orgId}', this.orgId);
  }

  /** Filter nav entries based on modules property. */
  private _visibleEntries(): NavEntry[] {
    if (!this.modules.trim()) {
      return [...NAV_ENTRIES];
    }
    const allowed = new Set(
      this.modules.split(',').map((m) => m.trim()).filter(Boolean)
    );
    return NAV_ENTRIES.filter(
      (entry) => ALWAYS_VISIBLE_KEYS.has(entry.key) || allowed.has(entry.key)
    );
  }

  private _renderNav() {
    return this._visibleEntries().map((entry) => {
      if (entry.key === '__divider__') {
        return html`<div class="nav-divider" role="separator"></div>`;
      }
      return html`
        <button
          class="nav-item"
          @click=${() => this._routes.goto(this._navPath(entry.path))}
          aria-label="${entry.label}"
        >
          <sl-icon name="${entry.icon}"></sl-icon>
          ${entry.label}
        </button>
      `;
    });
  }

  override render() {
    return html`
      <div class="topbar">
        <sl-icon-button
          name="list"
          class="hamburger-toggle"
          label="Toggle sidebar"
          @click=${() => { this._sidebarOpen = !this._sidebarOpen; }}
        ></sl-icon-button>
        <span class="wordmark">Open Routing</span>
        <div class="topbar-spacer"></div>
        <sl-tooltip content="${this.orgId}">
          <code
            class="org-chip"
            @click=${() => { navigator.clipboard?.writeText(this.orgId); }}
          >
            org: ${this.orgId.slice(0, 8)}&hellip;
          </code>
        </sl-tooltip>
        <sl-button-group label="Theme" style="margin: 0 8px;">
          <sl-icon-button
            name="sun"
            title="Light theme"
            @click=${() => { this.theme = 'or-light'; }}
          ></sl-icon-button>
          <sl-icon-button
            name="moon"
            title="Dark theme"
            @click=${() => { this.theme = 'or-dark'; }}
          ></sl-icon-button>
          <sl-icon-button
            name="palette"
            title="Brand theme"
            @click=${() => { this.theme = 'or-brand'; }}
          ></sl-icon-button>
        </sl-button-group>
        <sl-button variant="text" @click=${() => { this._routes.goto('/'); }}>
          Switch org
        </sl-button>
      </div>
      <div class="body">
        <nav class="sidebar" aria-label="Navigation">
          ${this._renderNav()}
        </nav>
        <main class="content">
          ${this._routes.outlet()}
        </main>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-catalog-shell': OrCatalogShell;
  }
}
