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

import { LitElement, html, css, nothing, type PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Routes } from '@lit-labs/router';
import { orLight, orDark, orBrand, ALL_TOKEN_KEYS } from '../../themes/index.js';
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

// Agent components (Wave 2, Plan 06-05) — wired to real routes in Plan 06-06.

// Shoelace per-component imports (D6-08: tree-shaking required for Phase 7 70KB budget)
import '@shoelace-style/shoelace/dist/components/icon/icon.js';
import '@shoelace-style/shoelace/dist/components/icon-button/icon-button.js';
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/button-group/button-group.js';
import '@shoelace-style/shoelace/dist/components/select/select.js';
import '@shoelace-style/shoelace/dist/components/option/option.js';
import '@shoelace-style/shoelace/dist/components/tooltip/tooltip.js';

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
    .nav-item--active {
      background: var(--or-color-nav-active-bg, rgba(43, 138, 147, 0.1));
      color: var(--or-color-primary, #2b8a93);
      font-weight: 500;
    }
    .nav-item--active:hover {
      background: var(--or-color-nav-active-bg, rgba(43, 138, 147, 0.15));
    }
    .nav-item--active sl-icon {
      color: var(--or-color-primary, #2b8a93);
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
            this._navigate('/');
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
