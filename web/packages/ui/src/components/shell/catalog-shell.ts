// Phase 6 Plan 03: <or-catalog-shell> — page-level shell with @lit-labs/router,
// sidebar nav (8 entries per D6-11), top bar with theme toggle (D6-19),
// UUIDv7 route guard (D6-13). This is the mounting point for the admin SPA.
//
// D6-20: Theme CSS custom properties applied to THIS host element (this.style).
// Applying to the shell host (not the document root) is required for Phase 7 Shadow DOM isolation.
//
// Plan 06-06: Wire real agent routes + createApiClient bootstrap from URL org_id.
// - Shared _orgRouteEnter() guard runs on ALL entity routes (D6-13: reject malformed before any API call).
// - State sync (orgId, _currentOrgId, client bootstrap) moved to enter() — render() stays pure.
// - Agent routes render OrAgentList/OrAgentDetail/OrAgentForm with .client property.
// - Placeholder divs for Wave 3-5 entities (skills/queues/break-reasons/adapters/channels/imports/status).
// - 'open-routing:navigate' events from entity components reach this._routes.goto().
// - Switch org clears _client + _currentOrgId and navigates to '/'.

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Routes } from '@lit-labs/router';
import { orLight, orDark, orBrand, ALL_TOKEN_KEYS } from '../../themes/index.js';
import type { ThemeName } from '../../themes/index.js';
import { createApiClient } from '../../api/client.js';
import type { ApiClient } from '../../api/client.js';

// Agent components (Wave 2, Plan 06-05) — wired to real routes in Plan 06-06.
import '../agents/agent-list.js';
import '../agents/agent-detail.js';
import '../agents/agent-form.js';

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/button-group/button-group.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';
import '@shoelace-style/shoelace/dist/components/drawer/drawer.js';

// Import primitives used in route renders (must be registered before outlet renders them)
import '../primitives/org-picker.js';

// Wave 3: Break Reasons entity (Plan 06-09)
import '../break-reasons/break-reason-list.js';
import '../break-reasons/break-reason-detail.js';
import '../break-reasons/break-reason-form.js';

// Wave 4: Channels entity + queue-picker primitive (Plan 06-10)
import '../channels/channel-list.js';
import '../channels/channel-detail.js';
import '../channels/channel-form.js';

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
  // Agent Status nav links to the agents list (user selects an agent to view its status).
  // D6-12: status route is per-agent /agents/:id/status — no standalone status list page.
  { key: 'status',        label: 'Agent Status',   icon: 'circle-fill',       path: '/orgs/{orgId}/agents' },
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
  static override styles = css`
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
    .placeholder-wave {
      color: var(--or-color-text-muted, #888);
      font-size: 14px;
      padding: 32px;
      text-align: center;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String }) theme: ThemeName = 'or-light';
  @property({ type: String }) modules = '';
  @property({ type: String }) locale: 'en' | 'vi' = 'en';

  @state() private _sidebarOpen = false;

  /**
   * D6-09: current org_id parsed from URL. getOrgId() in createApiClient closes
   * over _currentOrgId (not a snapshot), so the client returns the latest value
   * without recreation on org change.
   */
  @state() private _currentOrgId = '';

  /**
   * D6-09: single client instance for the lifetime of an org session.
   * Null before an org is selected (org-picker screen).
   * Replaced (not mutated) when org changes via org-picker or URL.
   */
  @state() private _client: ApiClient | null = null;

  /**
   * Shared enter() guard for all /orgs/:org_id/* routes (D6-13).
   * Rejects malformed org_id before any component renders + API call fires.
   * Also syncs _currentOrgId and bootstraps the API client on direct URL navigation.
   * render() functions stay pure — no state mutation inside render.
   */
  private _orgRouteEnter = async ({ org_id }: Record<string, string | undefined>): Promise<boolean> => {
    if (!UUIDV7_PATTERN.test(org_id ?? '')) {
      this._routes.goto('/');
      return false;
    }
    const safeOrgId = org_id as string;
    this._currentOrgId = safeOrgId;
    this.orgId = safeOrgId;
    if (!this._client) {
      this._client = createApiClient({ baseURL: '', getOrgId: () => this._currentOrgId });
    }
    return true;
  };

  /** @lit-labs/router Routes — outlet() MUST stay in this component's render() per Pitfall 7. */
  private _routes = new Routes(this, [
    {
      path: '/',
      render: () => html`<or-org-picker></or-org-picker>`,
    },
    {
      path: '/orgs/:org_id/agents',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-agent-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-list>`,
    },
    {
      path: '/orgs/:org_id/agents/new',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-agent-form .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-form>`,
    },
    {
      path: '/orgs/:org_id/agents/:id',
      enter: this._orgRouteEnter,
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-agent-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-agent-detail>`,
    },
    {
      // D6-12: status panel nested per-agent. Placeholder for Wave 5 (Plan 06-12).
      path: '/orgs/:org_id/agents/:id/status',
      enter: this._orgRouteEnter,
      render: ({ id }: Record<string, string | undefined>) =>
        html`<div class="placeholder-wave" data-route="agent-status" data-agent-id="${id}">Agent Status Panel — coming in Wave 5 (Plan 06-12)</div>`,
    },
    {
      // Skills — placeholder for Wave 3 (Plan 06-07)
      path: '/orgs/:org_id/skills',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="skills">Skills — coming in Wave 3 (Plan 06-07)</div>`,
    },
    {
      path: '/orgs/:org_id/skills/new',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="skills-new">Skill Create — coming in Wave 3 (Plan 06-07)</div>`,
    },
    {
      path: '/orgs/:org_id/skills/:id',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="skill-detail">Skill Detail — coming in Wave 3 (Plan 06-07)</div>`,
    },
    {
      // Queues — placeholder for Wave 3 (Plan 06-08)
      path: '/orgs/:org_id/queues',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="queues">Queues — coming in Wave 3 (Plan 06-08)</div>`,
    },
    {
      path: '/orgs/:org_id/queues/new',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="queues-new">Queue Create — coming in Wave 3 (Plan 06-08)</div>`,
    },
    {
      path: '/orgs/:org_id/queues/:id',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="queue-detail">Queue Detail — coming in Wave 3 (Plan 06-08)</div>`,
    },
    {
      // Channels — wired in Wave 4 (Plan 06-10)
      path: '/orgs/:org_id/channels',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-channel-list .orgId=${org_id ?? ''} .client=${this._client!}></or-channel-list>`,
    },
    {
      path: '/orgs/:org_id/channels/new',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-channel-form .orgId=${org_id ?? ''} .client=${this._client!}></or-channel-form>`,
    },
    {
      path: '/orgs/:org_id/channels/:id',
      enter: this._orgRouteEnter,
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-channel-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-channel-detail>`,
    },
    {
      // Adapters — placeholder for Wave 3 (Plan 06-11)
      path: '/orgs/:org_id/adapters',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="adapters">Adapters — coming in Wave 3 (Plan 06-11)</div>`,
    },
    {
      path: '/orgs/:org_id/adapters/new',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="adapters-new">Adapter Create — coming in Wave 3 (Plan 06-11)</div>`,
    },
    {
      path: '/orgs/:org_id/adapters/:id',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="adapter-detail">Adapter Detail — coming in Wave 3 (Plan 06-11)</div>`,
    },
    {
      // Break Reasons — wired in Wave 3 (Plan 06-09); client prop added here (Rule 2 fix)
      path: '/orgs/:org_id/break-reasons',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-break-reason-list .orgId=${org_id ?? ''} .client=${this._client!}></or-break-reason-list>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/new',
      enter: this._orgRouteEnter,
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-break-reason-form .orgId=${org_id ?? ''} .client=${this._client!}></or-break-reason-form>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/:id',
      enter: this._orgRouteEnter,
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-break-reason-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-break-reason-detail>`,
    },
    {
      // D6-12: Import is top-level org route. Placeholder for Wave 5 (Plan 06-13).
      path: '/orgs/:org_id/imports/new',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="imports-new">Bulk Import — coming in Wave 5 (Plan 06-13)</div>`,
    },
    {
      path: '/orgs/:org_id/imports/:id',
      enter: this._orgRouteEnter,
      render: () => html`<div class="placeholder-wave" data-route="import-result">Import Result — coming in Wave 5 (Plan 06-13)</div>`,
    },
  ]);

  override connectedCallback(): void {
    super.connectedCallback();
    try {
      const saved = localStorage.getItem('or-theme') as ThemeName | null;
      if (saved && (VALID_THEME_NAMES as readonly string[]).includes(saved)) {
        this.theme = saved as ThemeName;
      }
    } catch {
      // localStorage unavailable (private browsing, cross-origin embed) — use default theme
    }
    this._applyTheme();

    // D6-09: Parse initial org_id from URL path on first connect.
    this._syncOrgIdFromUrl();

    this._handlePopState = () => { this._syncOrgIdFromUrl(); };
    window.addEventListener('popstate', this._handlePopState);

    this.addEventListener('open-routing:org-selected', this._handleOrgSelected);
    this.addEventListener('open-routing:navigate', this._handleNavigate);
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this.removeEventListener('open-routing:org-selected', this._handleOrgSelected);
    this.removeEventListener('open-routing:navigate', this._handleNavigate);
    if (this._handlePopState) {
      window.removeEventListener('popstate', this._handlePopState);
    }
  }

  /** Popstate handler reference for cleanup on disconnectedCallback. */
  private _handlePopState: (() => void) | null = null;

  /**
   * D6-09: Parse /orgs/:org_id/ segment from window.location.pathname.
   * The regex requires a trailing slash (all entity routes have one).
   * UUIDv7 validation intentionally skipped here — route enter() guards handle it.
   */
  private _syncOrgIdFromUrl(): void {
    const match = /\/orgs\/([^/]+)\//.exec(window.location.pathname);
    if (match?.[1]) {
      const org_id: string = match[1];
      this._currentOrgId = org_id;
      this.orgId = org_id;
      if (!this._client) {
        this._client = createApiClient({ baseURL: '', getOrgId: () => this._currentOrgId });
      }
    }
  }

  private _handleNavigate = (e: Event): void => {
    const { path } = (e as CustomEvent<{ path: string }>).detail;
    this._routes.goto(path);
  };

  private _handleOrgSelected = (e: Event): void => {
    const { orgId } = (e as CustomEvent<{ orgId: string }>).detail;
    this._currentOrgId = orgId;
    this.orgId = orgId;
    this._client = createApiClient({ baseURL: '', getOrgId: () => this._currentOrgId });
    this._routes.goto(`/orgs/${orgId}/agents`);
  };

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('theme')) {
      this._applyTheme();
    }
  }

  private _applyTheme(): void {
    for (const key of ALL_TOKEN_KEYS) {
      this.style.removeProperty(key);
    }
    const tokens = _THEME_TOKENS[this.theme] ?? {};
    for (const [key, val] of Object.entries(tokens)) {
      this.style.setProperty(key, val);
    }
    try {
      localStorage.setItem('or-theme', this.theme);
    } catch {
      // Ignore — localStorage unavailable in some embed contexts
    }
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
        <sl-button
          variant="text"
          @click=${() => {
            this._currentOrgId = '';
            this.orgId = '';
            this._client = null;
            this._routes.goto('/');
          }}
        >
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
