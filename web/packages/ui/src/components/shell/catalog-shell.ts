// Phase 7 Plan W1-01: <or-catalog-shell> — Ember-style healthcare dashboard layout.
// Sidebar (260px) + topbar + content area grid. uk-* Frankenstyle classes throughout.
// Routing, theme, and state logic unchanged from Phase 6.

import { LitElement, html, css, nothing, type PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Routes } from '@lit-labs/router';
import { orLight, orDark, orBrand, ALL_TOKEN_KEYS, isDarkClassTheme } from '../../themes/index.js';
import type { ThemeName } from '../../themes/index.js';

// Iteration-2 BLOCKER #1 — typed seam for hash-routing injection.
// Defined in packages/ui so consumers (apps/embed) implement against
// the contract without packages/ui needing a static dep on
// @open-routing/embed. Symmetric coupling: shell hands its Routes
// instance to adapter.start(routes); adapter never reaches into
// shell. No workspace dep cycle.
export interface CatalogShellRouterAdapter {
  start(routes: Routes): void;
  stop(): void;
}

import { createApiClient } from '../../api/client.js';
import type { ApiClient } from '../../api/client.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Agent components (Wave 2, Plan 06-05) — wired to real routes in Plan 06-06.

// Import primitives used in route renders (must be registered before outlet renders them)
import '../primitives/org-picker.js';

// Wave 3: Break Reasons entity (Plan 06-09)

// Wave 4: Channels entity + queue-picker primitive (Plan 06-10)

// Ship-fix (Codex review HIGH): wire Skills, Queues, Adapters routes that
// were still rendering Wave-3 placeholders despite their components landing.

// Wave 4: Status Panel (Plan 06-12)

// Wave 4: Bulk Import (Plan 06-13)

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
  { key: 'agents',        label: 'Agents',        icon: 'users',        path: '/orgs/{orgId}/agents' },
  { key: 'skills',        label: 'Skills',         icon: 'tag',          path: '/orgs/{orgId}/skills' },
  { key: 'queues',        label: 'Queues',         icon: 'filter',       path: '/orgs/{orgId}/queues' },
  { key: 'channels',      label: 'Channels',       icon: 'radio',        path: '/orgs/{orgId}/channels' },
  { key: 'adapters',      label: 'Adapters',       icon: 'plug',         path: '/orgs/{orgId}/adapters' },
  { key: 'break-reasons', label: 'Break Reasons',  icon: 'pause-circle', path: '/orgs/{orgId}/break-reasons' },
  { key: '__divider__',   label: '',               icon: '',             path: '' },
  { key: 'imports',       label: 'Bulk Import',    icon: 'upload',       path: '/orgs/{orgId}/imports/new' },
  { key: 'status',        label: 'Agent Status',   icon: 'activity',     path: '/orgs/{orgId}/agents/status' },
] as const;

/** Entries that always show regardless of modules filter (divider, imports, status). */
const ALWAYS_VISIBLE_KEYS = new Set(['__divider__', 'imports', 'status']);

/** Theme token lookup table. */
const _THEME_TOKENS: Record<ThemeName, Record<string, string>> = {
  'or-light': orLight,
  'or-dark': orDark,
  'or-brand': orBrand,
  'ember-light': {},
  'ember-dark': {},
};

/** Valid theme names for localStorage restore guard. */
const VALID_THEME_NAMES: readonly ThemeName[] = ['or-light', 'or-dark', 'or-brand', 'ember-light', 'ember-dark'];

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
      display: block;
      height: 100vh;
      background: var(--background, #fff);
      color: var(--foreground, #111);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    .app-grid {
      display: grid;
      grid-template-columns: 260px 1fr;
      grid-template-rows: 56px 1fr;
      grid-template-areas:
        "sidebar topbar"
        "sidebar content";
      height: 100vh;
      transition: grid-template-columns 0.15s ease;
    }

    .app-grid--collapsed {
      grid-template-columns: 64px 1fr;
    }

    .sidebar {
      grid-area: sidebar;
      grid-row: 1 / -1;
      background: var(--card, #fafafa);
      border-right: 1px solid var(--border, #e5e5e5);
      display: flex;
      flex-direction: column;
      overflow: hidden;
    }

    .sidebar-header {
      padding: 16px;
      display: flex;
      align-items: center;
      gap: 12px;
      border-bottom: 1px solid var(--border, #e5e5e5);
      flex-shrink: 0;
    }

    .wordmark {
      font-weight: 600;
      font-size: 15px;
      color: var(--foreground, #111);
      white-space: nowrap;
      overflow: hidden;
      flex: 1;
    }
    .app-grid--collapsed .wordmark { display: none; }

    .sidebar-nav {
      flex: 1;
      overflow-y: auto;
      padding: 12px 0;
    }

    .nav-section-label {
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--muted-foreground, #888);
      padding: 16px 16px 6px;
    }
    .app-grid--collapsed .nav-section-label { display: none; }

    .nav-item {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 12px;
      margin: 1px 8px;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 500;
      color: var(--foreground, #111);
      cursor: pointer;
      background: transparent;
      border: none;
      text-align: left;
      width: calc(100% - 16px);
      transition: background 0.12s, color 0.12s;
    }
    .nav-item:hover {
      background: var(--muted, #f5f5f5);
    }
    .nav-item--active {
      background: color-mix(in oklch, var(--primary, #f12c3d) 12%, transparent);
      color: var(--primary, #f12c3d);
      font-weight: 600;
    }
    .nav-item--active:hover {
      background: color-mix(in oklch, var(--primary, #f12c3d) 18%, transparent);
    }
    .nav-item-label { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .app-grid--collapsed .nav-item-label { display: none; }
    .app-grid--collapsed .nav-item { justify-content: center; }

    .nav-divider {
      height: 1px;
      background: var(--border, #e5e5e5);
      margin: 8px 16px;
    }

    .sidebar-footer {
      padding: 12px;
      border-top: 1px solid var(--border, #e5e5e5);
      display: flex;
      flex-direction: column;
      gap: 8px;
      flex-shrink: 0;
    }

    .theme-row { display: flex; gap: 4px; }
    .theme-btn {
      flex: 1;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 6px;
      background: transparent;
      border: 1px solid var(--border, #e5e5e5);
      border-radius: 6px;
      color: var(--foreground, #111);
      cursor: pointer;
    }
    .theme-btn:hover { background: var(--muted, #f5f5f5); }
    .theme-btn--active {
      background: var(--primary, #f12c3d);
      color: var(--primary-foreground, #fff);
      border-color: var(--primary, #f12c3d);
    }
    .app-grid--collapsed .theme-row { flex-direction: column; }

    .topbar {
      grid-area: topbar;
      display: flex;
      align-items: center;
      padding: 0 24px;
      border-bottom: 1px solid var(--border, #e5e5e5);
      background: var(--card, #fafafa);
      gap: 16px;
    }

    .hamburger {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      background: transparent;
      border: none;
      border-radius: 6px;
      color: var(--foreground, #111);
      cursor: pointer;
      flex-shrink: 0;
    }
    .hamburger:hover { background: var(--muted, #f5f5f5); }

    .topbar-spacer { flex: 1; }

    .org-chip {
      font-family: 'SF Mono', Monaco, Consolas, monospace;
      font-size: 12px;
      background: var(--muted, #f5f5f5);
      color: var(--muted-foreground, #888);
      padding: 4px 10px;
      border-radius: 6px;
      cursor: pointer;
      border: none;
    }
    .org-chip:hover {
      background: color-mix(in oklch, var(--muted, #f5f5f5) 80%, var(--foreground, #111) 20%);
    }

    .content {
      grid-area: content;
      background: var(--background, #fff);
      overflow-y: auto;
      padding: 24px 32px;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String }) theme: ThemeName = 'or-light';
  @property({ type: String }) modules = '';
  @property({ type: String }) locale: 'en' | 'vi' = 'en';


  // routingMode + routerAdapter — admin defaults to history (pretty URLs);
  // embed sets 'hash' via routing-mode attribute AND assigns routerAdapter
  // (HashRouterAdapter instance) BEFORE the attribute flip. Iteration-2
  // BLOCKER #1: typed-interface seam in packages/ui prevents a workspace
  // dep cycle (packages/ui → embed → ui). Shell hands Routes to adapter
  // via start(routes); no `_routes` cast in consumer.
  @property({ type: String, attribute: 'routing-mode' })
  routingMode: 'history' | 'hash' = 'history';

  // Iteration-2 BLOCKER #1 — typed adapter injection seam.
  // Consumers (embed-element in apps/embed) assign this BEFORE flipping
  // routingMode to 'hash'. Shell calls adapter.start(this._routes) in
  // firstUpdated. attribute: false because Custom Element attributes
  // can only carry strings; consumers set the property programmatically.
  @property({ attribute: false })
  routerAdapter: CatalogShellRouterAdapter | undefined = undefined;

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
      window.history.pushState(null, '', '/');
      this._syncOrgIdFromUrl();
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


  private async _composedEnter(
    params: Record<string, string | undefined>,
    loader: () => Promise<unknown>,
    entity?: string,
  ): Promise<boolean> {
    // _orgRouteEnter MUST run FIRST — malformed UUIDv7 wastes a chunk fetch.
    if (!(await this._orgRouteEnter(params))) return false;
    // WARNING #6 — EMBED-06-b: when modules attribute is set and the
    // entity is not in the CSV, redirect to the first allowed entity.
    // Prevents direct-URL navigation past the sidebar filter.
    if (entity && this.modules) {
      const allowed = this.modules.split(',').map((s) => s.trim()).filter(Boolean);
      if (allowed.length > 0 && !allowed.includes(entity)) {
        const orgId = params.org_id ?? '';
        void this._routes.goto('/orgs/' + orgId + '/' + allowed[0]);
        return false;
      }
    }
    await loader();
    return true;
  }

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  /**
   * @lit-labs/router Routes
   * Lazy routes — D7-03 single-source shell across admin + embed (D7-02 eager baseline = chrome + 5 primitives only;
   * entities split per route). _composedEnter runs _orgRouteEnter guard first (avoids chunk fetch on malformed UUID)
   * then dynamic import for code splitting. @lit-labs/router goto() awaits the enter Promise
   * (verified npm README + Lit discussion #3354).
   * outlet() MUST stay in this component's render() per Pitfall 7.
   */
  private _routes = new Routes(this, [
    {
      path: '/',
      render: () => html`<or-org-picker></or-org-picker>`,
    },
    {
      path: '/orgs/:org_id/agents',
      enter: (params) => this._composedEnter(params, () => import('../agents/agent-list.js'), 'agents'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-agent-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-list>`,
    },
    {
      path: '/orgs/:org_id/agents/new',
      enter: (params) => this._composedEnter(params, () => import('../agents/agent-form.js'), 'agents'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-agent-form .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-form>`,
    },
    {
      path: '/orgs/:org_id/agents/status',
      enter: (params) => this._composedEnter(params, () => import('../status/agent-status-list.js'), 'status'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-agent-status-list .orgId=${org_id ?? ''} .client=${this._client!}></or-agent-status-list>`,
    },
    {
      path: '/orgs/:org_id/agents/:id',
      enter: (params) => this._composedEnter(params, () => import('../agents/agent-detail.js'), 'agents'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-agent-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-agent-detail>`,
    },
    {
      // D6-12: status panel nested per-agent. Wired in Wave 5 (Plan 06-12).
      // enter: this._orgRouteEnter — UUIDv7 guard preserved (mirrors 06-09 BreakReasons pattern).
      path: '/orgs/:org_id/agents/:id/status',
      enter: (params) => this._composedEnter(params, () => import('../status/status-panel.js'), 'status'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-status-panel .orgId=${org_id ?? ''} .agentId=${id ?? ''} .client=${this._client!}></or-status-panel>`,
    },
    {
      // Skills (Plan 06-07) — wired by ship-fix
      path: '/orgs/:org_id/skills',
      enter: (params) => this._composedEnter(params, () => import('../skills/skill-list.js'), 'skills'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-skill-list .orgId=${org_id ?? ''} .client=${this._client!}></or-skill-list>`,
    },
    {
      path: '/orgs/:org_id/skills/new',
      enter: (params) => this._composedEnter(params, () => import('../skills/skill-form.js'), 'skills'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-skill-form .orgId=${org_id ?? ''} .client=${this._client!}></or-skill-form>`,
    },
    {
      path: '/orgs/:org_id/skills/:id',
      enter: (params) => this._composedEnter(params, () => import('../skills/skill-detail.js'), 'skills'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-skill-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-skill-detail>`,
    },
    {
      // Queues (Plan 06-08) — wired by ship-fix
      path: '/orgs/:org_id/queues',
      enter: (params) => this._composedEnter(params, () => import('../queues/queue-list.js'), 'queues'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-queue-list .orgId=${org_id ?? ''} .client=${this._client!}></or-queue-list>`,
    },
    {
      path: '/orgs/:org_id/queues/new',
      enter: (params) => this._composedEnter(params, () => import('../queues/queue-form.js'), 'queues'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-queue-form .orgId=${org_id ?? ''} .client=${this._client!}></or-queue-form>`,
    },
    {
      path: '/orgs/:org_id/queues/:id',
      enter: (params) => this._composedEnter(params, () => import('../queues/queue-detail.js'), 'queues'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-queue-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-queue-detail>`,
    },
    {
      // Channels — wired in Wave 4 (Plan 06-10)
      path: '/orgs/:org_id/channels',
      enter: (params) => this._composedEnter(params, () => import('../channels/channel-list.js'), 'channels'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-channel-list .orgId=${org_id ?? ''} .client=${this._client!}></or-channel-list>`,
    },
    {
      path: '/orgs/:org_id/channels/new',
      enter: (params) => this._composedEnter(params, () => Promise.all([import('../channels/channel-form.js'), import('../primitives/queue-picker.js')]), 'channels'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-channel-form .orgId=${org_id ?? ''} .client=${this._client!}></or-channel-form>`,
    },
    {
      path: '/orgs/:org_id/channels/:id',
      enter: (params) => this._composedEnter(params, () => import('../channels/channel-detail.js'), 'channels'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-channel-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-channel-detail>`,
    },
    {
      // Adapters (Plan 06-11) — wired by ship-fix
      path: '/orgs/:org_id/adapters',
      enter: (params) => this._composedEnter(params, () => import('../adapters/adapter-list.js'), 'adapters'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-adapter-list .orgId=${org_id ?? ''} .client=${this._client!}></or-adapter-list>`,
    },
    {
      path: '/orgs/:org_id/adapters/new',
      enter: (params) => this._composedEnter(params, () => import('../adapters/adapter-form.js'), 'adapters'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-adapter-form .orgId=${org_id ?? ''} .client=${this._client!}></or-adapter-form>`,
    },
    {
      path: '/orgs/:org_id/adapters/:id',
      enter: (params) => this._composedEnter(params, () => import('../adapters/adapter-detail.js'), 'adapters'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-adapter-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-adapter-detail>`,
    },
    {
      // Break Reasons — wired in Wave 3 (Plan 06-09); client prop added here (Rule 2 fix)
      path: '/orgs/:org_id/break-reasons',
      enter: (params) => this._composedEnter(params, () => import('../break-reasons/break-reason-list.js'), 'break-reasons'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-break-reason-list .orgId=${org_id ?? ''} .client=${this._client!}></or-break-reason-list>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/new',
      enter: (params) => this._composedEnter(params, () => import('../break-reasons/break-reason-form.js'), 'break-reasons'),
      render: ({ org_id }: Record<string, string | undefined>) =>
        html`<or-break-reason-form .orgId=${org_id ?? ''} .client=${this._client!}></or-break-reason-form>`,
    },
    {
      path: '/orgs/:org_id/break-reasons/:id',
      enter: (params) => this._composedEnter(params, () => import('../break-reasons/break-reason-detail.js'), 'break-reasons'),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-break-reason-detail .orgId=${org_id ?? ''} .entityId=${id ?? ''} .client=${this._client!}></or-break-reason-detail>`,
    },
    {
      // D6-12: Import POST flow (Plan 06-13). Wire real or-import-page.
      path: '/orgs/:org_id/imports/new',
      enter: (params) => this._composedEnter(params, () => import('../imports/import-page.js')),
      render: ({ org_id }: Record<string, string | undefined>) => {
        this._currentOrgId = org_id ?? '';
        return html`<or-import-page
          .orgId=${org_id ?? ''}
          .baseURL=${''}
          .client=${this._client!}
        ></or-import-page>`;
      },
    },
    {
      // D6-12: Import GET historical result (Plan 06-13). Wire real or-import-result.
      path: '/orgs/:org_id/imports/:id',
      enter: (params) => this._composedEnter(params, () => import('../imports/import-result.js')),
      render: ({ org_id, id }: Record<string, string | undefined>) =>
        html`<or-import-result
          .orgId=${org_id ?? ''}
          .importId=${id ?? ''}
          .client=${this._client!}
        ></or-import-result>`,
    },
    {
      // W0.0-03: Dev-only UI playground — no org scope, no UUIDv7 guard.
      // Renders 13 component slots for downstream W0.0 plan verification.
      path: '/playground',
      enter: async () => {
        await import('./playground-route.js');
        return true;
      },
      render: () => html`<or-playground-route></or-playground-route>`,
    },
  ]);


  override firstUpdated(_changed: PropertyValues): void {
    if (this.routingMode === 'hash' && this.routerAdapter) {
      this.routerAdapter.start(this._routes);
    }
  }



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

    this._handlePopState = () => {
      this._syncOrgIdFromUrl();
      this._routes.goto(window.location.pathname);
    };
    window.addEventListener('popstate', this._handlePopState);

    this.addEventListener('open-routing:org-selected', this._handleOrgSelected);
    this.addEventListener('open-routing:navigate', this._handleNavigate);
    this.addEventListener('open-routing:theme-change', this._handleThemeChange);

    setTimeout(() => {
      this._routes.goto(window.location.pathname);
    }, 0);
  }

  override disconnectedCallback(): void {
    if (this.routerAdapter) {
      this.routerAdapter.stop();
    }
    super.disconnectedCallback();
    this.removeEventListener('open-routing:org-selected', this._handleOrgSelected);
    this.removeEventListener('open-routing:navigate', this._handleNavigate);
    this.removeEventListener('open-routing:theme-change', this._handleThemeChange);
    if (this._handlePopState) {
      window.removeEventListener('popstate', this._handlePopState);
    }
  }

  /** Popstate handler reference for cleanup on disconnectedCallback. */
  private _handlePopState: (() => void) | null = null;

  private _navigate(path: string, replace = false): void {
    if (replace) {
      window.history.replaceState(null, '', path);
    } else {
      window.history.pushState(null, '', path);
    }
    this._routes.goto(path.split('?')[0] ?? path);
  }

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
    } else {
      this._currentOrgId = '';
      this.orgId = '';
      this._client = null;
    }
  }

  private _handleNavigate = (e: Event): void => {
    const { path } = (e as CustomEvent<{ path: string }>).detail;
    this._navigate(path);
  };

  private _handleThemeChange = (e: Event): void => {
    const { theme } = (e as CustomEvent<{ theme: ThemeName }>).detail;
    this.theme = theme;
  };

  private _handleOrgSelected = (e: Event): void => {
    const { orgId } = (e as CustomEvent<{ orgId: string }>).detail;
    this._currentOrgId = orgId;
    this.orgId = orgId;
    this._client = createApiClient({ baseURL: '', getOrgId: () => this._currentOrgId });
    this._navigate(`/orgs/${orgId}/agents`);
  };

  override updated(changed: Map<string, unknown>): void {
    if (changed.has('routingMode') && changed.get('routingMode') !== undefined) {
      console.warn('[catalog-shell] routingMode changed after construction — admin sets once, embed sets once; behavior is undefined');
    }
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
    document.documentElement.classList.toggle('dark', isDarkClassTheme(this.theme));
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
    const currentPath = window.location.pathname;
    return this._visibleEntries().map((entry) => {
      if (entry.key === '__divider__') {
        return html`<div class="nav-divider" role="separator"></div>`;
      }
      const resolved = this._navPath(entry.path);
      const basePath = resolved.split('/').slice(0, 4).join('/');
      const onStatusRoute = currentPath.includes('/agents/') && currentPath.endsWith('/status')
        || currentPath.endsWith('/agents/status');
      const isActive = entry.key === 'status'
        ? onStatusRoute
        : !!basePath && currentPath.startsWith(basePath) && !onStatusRoute;
      return html`
        <button
          class="nav-item ${isActive ? 'nav-item--active' : ''}"
          @click=${() => this._navigate(resolved)}
          aria-current=${isActive ? 'page' : nothing}
          aria-label=${entry.label}
        >
          <uk-icon icon=${entry.icon} height="18" width="18"></uk-icon>
          <span class="nav-item-label">${entry.label}</span>
        </button>
      `;
    });
  }

  override render() {
    // _sidebarOpen=false → sidebar expanded (default); true → collapsed to icon-only
    const collapsed = this._sidebarOpen ? 'app-grid--collapsed' : '';
    return html`
      <div class="app-grid ${collapsed}">
        <aside class="sidebar" aria-label="Navigation">
          <header class="sidebar-header">
            <button class="hamburger" @click=${() => { this._sidebarOpen = !this._sidebarOpen; }} aria-label="Toggle sidebar">
              <uk-icon icon="menu" height="20" width="20"></uk-icon>
            </button>
            <span class="wordmark">Open Routing</span>
          </header>
          <nav class="sidebar-nav">
            ${this._renderNav()}
          </nav>
          <footer class="sidebar-footer">
            <div class="theme-row" role="group" aria-label="Theme">
              <button
                class="theme-btn ${this.theme === 'or-light' || this.theme === 'ember-light' ? 'theme-btn--active' : ''}"
                @click=${() => { this.theme = 'ember-light'; }}
                title="Light"
              >
                <uk-icon icon="sun" height="16" width="16"></uk-icon>
              </button>
              <button
                class="theme-btn ${this.theme === 'or-dark' || this.theme === 'ember-dark' ? 'theme-btn--active' : ''}"
                @click=${() => { this.theme = 'ember-dark'; }}
                title="Dark"
              >
                <uk-icon icon="moon" height="16" width="16"></uk-icon>
              </button>
              <button
                class="theme-btn ${this.theme === 'or-brand' ? 'theme-btn--active' : ''}"
                @click=${() => { this.theme = 'or-brand'; }}
                title="Brand"
              >
                <uk-icon icon="palette" height="16" width="16"></uk-icon>
              </button>
            </div>
          </footer>
        </aside>
        <header class="topbar">
          <div class="topbar-spacer"></div>
          <code
            class="org-chip"
            title=${this.orgId}
            @click=${() => { navigator.clipboard?.writeText(this.orgId); }}
          >org: ${this.orgId.slice(0, 8)}&hellip;</code>
          <button
            class="hamburger"
            @click=${() => { this._currentOrgId = ''; this.orgId = ''; this._client = null; this._navigate('/'); }}
            aria-label="Switch org"
            title="Switch org"
          >
            <uk-icon icon="log-out" height="18" width="18"></uk-icon>
          </button>
        </header>
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
