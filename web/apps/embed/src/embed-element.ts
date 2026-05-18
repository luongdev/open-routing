// <open-routing-catalog> Custom Element — Phase 7 single artifact.
// Wraps <or-catalog-shell> (Phase 6 D6-05) inside Shadow DOM for host
// integration. Attribute parsing → reactive state → shell mount.
// Events bubble cross-Shadow via composed:true (D7-13/D7-14, Pitfall 4).
// UUIDv7 validation reuses shell's regex (D6-13). Theme JSON parse
// falls back to or-light on malformed input (RESEARCH §5). createApiClient
// is per-element with getOrgId closure over this.orgId so live attribute
// changes propagate (D-38).
//
// Iteration-2 BLOCKER #1 — typed-interface adapter injection. embed-element
// constructs HashRouterAdapter (parameterless), assigns it to the rendered
// shell's `routerAdapter` property BEFORE flipping `routing-mode="hash"`.
// Shell's firstUpdated calls adapter.start(this._routes) — no `_routes`
// cast leaves embed-element. Dep direction is apps/embed → packages/ui
// only; packages/ui has no static or dynamic import of @open-routing/embed.
//
// Single-embed-per-page is a documented v0.1 limitation (D7-07);
// hash-prefix attribute deferred to v0.2.

import { LitElement, html, css, type PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { Middleware } from 'openapi-fetch';
import { createApiClient, type ApiClient, type CatalogShellRouterAdapter } from '@open-routing/ui';
import { HashRouterAdapter } from './hash-router-adapter.js';
import { applyEmbedLocale } from './locale.js';
import '@open-routing/ui/components/shell';

const UUIDV7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const NAMED_THEMES = ['or-light', 'or-dark', 'or-brand'] as const;
type NamedTheme = typeof NAMED_THEMES[number];

@customElement('open-routing-catalog')
export class OpenRoutingCatalog extends LitElement {
  static override styles = css`
    :host {
      display: block;
      contain: content;
    }
    .error {
      padding: 16px;
      color: var(--or-color-danger, #d92d20);
      font-family: var(--sl-font-family, system-ui, sans-serif);
    }
    .auth-expired {
      padding: 12px 16px;
      background: var(--or-color-conflict-bg, #fef3c7);
      border-bottom: 1px solid var(--or-color-conflict-border, #f59e0b);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      font-family: var(--sl-font-family, system-ui, sans-serif);
    }
    .auth-expired button {
      background: transparent;
      border: 1px solid currentColor;
      border-radius: 4px;
      padding: 4px 12px;
      cursor: pointer;
      font: inherit;
    }
  `;

  @property({ type: String, attribute: 'org-id' }) orgId = '';
  @property({ type: String, attribute: 'api-base-url' }) apiBaseUrl = '';
  @property({ type: String }) theme = '';
  @property({ type: String }) modules = '';
  @property({ type: String }) locale: 'en' | 'vi' = 'en';

  @state() private _client: ApiClient | null = null;
  @state() private _validationError: string | null = null;
  @state() private _authExpired: { requestId: string; path: string } | null = null;
  @state() private _parsedTheme: NamedTheme | Record<string, string> | '' = '';

  private _hashAdapter: HashRouterAdapter | null = null;
  
  @query('or-catalog-shell') 
  private _shellEl!: (HTMLElement & {
    routerAdapter?: CatalogShellRouterAdapter;
    routingMode?: 'history' | 'hash';
  }) | null;

  override connectedCallback(): void {
    super.connectedCallback();
    this._validateAndBuildClient();
    this._parseTheme();
    if (!this._validationError) {
      this._dispatchRequestContext();
    }
  }

  override firstUpdated(_changed: PropertyValues): void {
    this._tryWireHashAdapter();
  }

  private _tryWireHashAdapter(): void {
    if (!this._client) return;
    const shellEl = this._shellEl;
    if (!shellEl) return;
    if (this._hashAdapter) return;
    this._hashAdapter = new HashRouterAdapter();
    shellEl.routerAdapter = this._hashAdapter;
    shellEl.routingMode = 'hash';
  }

  override updated(changed: PropertyValues): void {
    if (changed.has('orgId') || changed.has('apiBaseUrl')) {
      this._validateAndBuildClient();
    }
    if (changed.has('theme')) {
      this._parseTheme();
    }
    if (changed.has('locale')) {
      void applyEmbedLocale(this.locale);
    }
    // Re-attempt adapter wiring in case shell mounted after first render
    // (e.g., validation error cleared and shell re-rendered).
    this._tryWireHashAdapter();
  }

  override disconnectedCallback(): void {
    this._client = null;
    // Shell's own disconnectedCallback calls routerAdapter.stop() — but if
    // shell is disconnected before us OR has already been GC'd, also stop
    // here defensively (idempotent — stop() handles already-stopped state).
    if (this._hashAdapter) {
      this._hashAdapter.stop();
      this._hashAdapter = null;
    }
    super.disconnectedCallback();
  }

  private _validateAndBuildClient(): void {
    if (!this.orgId) {
      this._validationError = 'org-id attribute required';
      this._client = null;
      return;
    }
    if (!UUIDV7_PATTERN.test(this.orgId)) {
      this._validationError = 'Invalid org-id attribute (expected UUIDv7)';
      this._client = null;
      return;
    }
    this._validationError = null;
    this._client = createApiClient({
      baseURL: this.apiBaseUrl,
      getOrgId: () => this.orgId,
    });
    this._client.use(this._buildAuthExpiredMiddleware());
  }

  private _buildAuthExpiredMiddleware(): Middleware {
    return {
      onResponse: async ({ response, request }) => {
        if (response.status !== 401) return response;
        let requestId = '';
        try {
          const body = await response.clone().json();
          requestId = (body && typeof body === 'object' && 'request_id' in body && typeof body.request_id === 'string')
            ? body.request_id : '';
        } catch {
          // body not JSON; preserve empty requestId
        }
        const path = new URL(request.url).pathname;
        this._authExpired = { requestId, path };
        this.dispatchEvent(new CustomEvent('open-routing:auth-expired', {
          detail: { statusCode: 401, requestId, path },
          bubbles: true,
          composed: true,
        }));
        return response;
      },
    };
  }

  private _parseTheme(): void {
    if (!this.theme) {
      this._parsedTheme = '';
      return;
    }
    if ((NAMED_THEMES as readonly string[]).includes(this.theme)) {
      this._parsedTheme = this.theme as NamedTheme;
      return;
    }
    try {
      const parsed = JSON.parse(this.theme);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        // Reject prototype-pollution attempts (V8 — Tampering threat)
        delete (parsed as Record<string, unknown>).__proto__;
        this._parsedTheme = parsed as Record<string, string>;
        return;
      }
    } catch {
      // fall through to or-light default
    }
    this._parsedTheme = 'or-light';
  }

  private _dispatchRequestContext(): void {
    this.dispatchEvent(new CustomEvent('open-routing:request-context', {
      detail: {
        orgId: this.orgId,
        apiBaseUrl: this.apiBaseUrl,
        theme: this.theme,
        modules: this.modules,
      },
      bubbles: true,
      composed: true,
    }));
  }

  private _dismissAuthExpired(): void {
    this._authExpired = null;
  }

  override render() {
    if (this._validationError) {
      return html`<div class="error" role="alert">${this._validationError}</div>`;
    }
    return html`
      ${when(this._authExpired, () => html`
        <div class="auth-expired" role="alert">
          <span>Session expired${this._authExpired!.requestId
            ? html` — request ${this._authExpired!.requestId}` : ''}</span>
          <button @click=${this._dismissAuthExpired} aria-label="Dismiss">Dismiss</button>
        </div>
      `)}
      ${this._client ? html`
        <or-catalog-shell
          .orgId=${this.orgId}
          .theme=${this._parsedTheme}
          .modules=${this.modules}
          .locale=${this.locale}
          .client=${this._client}
        ></or-catalog-shell>
      ` : ''}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'open-routing-catalog': OpenRoutingCatalog;
  }
}
