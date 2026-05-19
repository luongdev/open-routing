// Public design-system demo page. Bypasses catalog-shell chrome (parent
// shell detects pathname === '/playground' and renders bare).
//
// Purpose: marketing-quality showcase of the Ember-style design language —
// brand hero, design tokens, components in context, theme switcher.

import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

@customElement('or-playground-route')
export class OrPlaygroundRoute extends LitElement {
  static override styles = css`
    :host {
      display: block;
      background: var(--background);
      color: var(--foreground);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    /* ============ Top bar ============ */
    .demo-top {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 16px 32px;
      background: var(--card);
      border-bottom: 1px solid var(--border);
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 10px;
      flex: 1;
    }
    .brand .logo {
      width: 32px; height: 32px; border-radius: 8px;
      background: linear-gradient(135deg, var(--primary), color-mix(in oklch, var(--primary) 60%, oklch(0.50 0.20 15)));
      display: inline-flex; align-items: center; justify-content: center;
      color: var(--primary-foreground);
      box-shadow: var(--shadow-primary);
    }
    .brand-text {
      font-weight: 700; font-size: 16px; letter-spacing: -0.01em;
      line-height: 1.1;
    }
    .brand-text small {
      display: block; font-size: 10px; font-weight: 600;
      letter-spacing: 0.12em; text-transform: uppercase;
      color: var(--muted-foreground); margin-top: 2px;
    }
    .demo-top nav { display: flex; gap: 4px; }
    .demo-top nav a {
      padding: 8px 14px; border-radius: 8px; font-size: 14px; font-weight: 500;
      color: var(--muted-foreground); text-decoration: none;
      transition: background .12s, color .12s;
    }
    .demo-top nav a:hover { background: var(--muted); color: var(--foreground); }
    .theme-switch { display: flex; gap: 4px; }
    .theme-pill {
      display: inline-flex; align-items: center; gap: 6px;
      padding: 7px 12px; border-radius: 8px;
      background: transparent; border: 1px solid var(--border);
      color: var(--foreground); cursor: pointer; font-size: 13px; font-weight: 500;
      transition: background .12s;
    }
    .theme-pill:hover { background: var(--muted); }
    .theme-pill.active {
      background: var(--primary); color: var(--primary-foreground);
      border-color: var(--primary); box-shadow: var(--shadow-primary);
    }

    /* ============ Hero ============ */
    .hero {
      padding: 80px 32px 64px;
      text-align: center;
      background:
        radial-gradient(ellipse at top, color-mix(in oklch, var(--primary) 8%, transparent) 0%, transparent 60%),
        var(--background);
    }
    .hero-badge {
      display: inline-flex; align-items: center; gap: 6px;
      padding: 6px 14px; border-radius: 9999px;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary); font-size: 12px; font-weight: 600;
      letter-spacing: 0.04em; margin-bottom: 20px;
    }
    .hero h1 {
      font-size: 56px; font-weight: 800; letter-spacing: -0.03em;
      margin: 0 0 16px; line-height: 1.05;
      color: var(--foreground);
    }
    .hero h1 .accent {
      background: linear-gradient(135deg, var(--primary), color-mix(in oklch, var(--primary) 55%, oklch(0.55 0.22 15)));
      -webkit-background-clip: text;
      background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    .hero p {
      font-size: 18px; line-height: 1.55; color: var(--muted-foreground);
      max-width: 620px; margin: 0 auto 32px;
    }
    .hero-cta {
      display: inline-flex; gap: 12px;
    }
    .btn-primary, .btn-ghost {
      display: inline-flex; align-items: center; gap: 8px;
      padding: 12px 22px; border-radius: 10px; font-size: 15px; font-weight: 600;
      cursor: pointer; transition: transform .12s, box-shadow .12s, background .12s;
      border: 1px solid transparent;
      text-decoration: none;
    }
    .btn-primary {
      background: var(--primary); color: var(--primary-foreground);
      box-shadow: var(--shadow-primary), inset 0 1px 0 0 oklch(1 0 0 / 0.15);
    }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-ghost {
      background: var(--card); color: var(--foreground);
      border-color: var(--border); box-shadow: var(--shadow-xs);
    }
    .btn-ghost:hover { background: var(--muted); }

    /* ============ Sections ============ */
    section.demo-section {
      padding: 64px 32px;
      max-width: 1280px; margin: 0 auto;
    }
    section.demo-section + section.demo-section { padding-top: 0; }
    .section-header { margin-bottom: 32px; text-align: center; }
    .section-eyebrow {
      font-size: 12px; font-weight: 700; letter-spacing: 0.12em;
      text-transform: uppercase; color: var(--primary); margin-bottom: 8px;
    }
    .section-header h2 {
      font-size: 32px; font-weight: 700; letter-spacing: -0.02em;
      margin: 0 0 8px; line-height: 1.2;
    }
    .section-header p {
      color: var(--muted-foreground); font-size: 15px; margin: 0;
      max-width: 580px; margin-inline: auto;
    }

    /* Colors swatch grid */
    .swatch-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: 12px;
    }
    .swatch {
      padding: 14px; border-radius: 12px; border: 1px solid var(--border);
      background: var(--card); box-shadow: var(--shadow-xs);
    }
    .swatch-color {
      height: 72px; border-radius: 8px; margin-bottom: 10px;
      border: 1px solid var(--border);
    }
    .swatch-label { font-size: 13px; font-weight: 600; }
    .swatch-token {
      font-family: 'SF Mono', Monaco, Consolas, monospace;
      font-size: 11px; color: var(--muted-foreground); margin-top: 2px;
    }

    /* Component preview cards */
    .preview-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
      gap: 20px;
    }
    .preview-card {
      background: var(--card); border: 1px solid var(--border);
      border-radius: 14px; padding: 24px; box-shadow: var(--shadow-sm);
    }
    .preview-card h3 {
      margin: 0 0 16px; font-size: 13px; font-weight: 700;
      color: var(--muted-foreground); letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    .preview-card .preview-row {
      display: flex; gap: 8px; flex-wrap: wrap; align-items: center;
      margin-bottom: 12px;
    }
    .preview-card .preview-row:last-child { margin-bottom: 0; }
    .preview-card input.uk-input {
      max-width: 280px;
    }
    .status-pill {
      display: inline-flex; padding: 3px 10px; border-radius: 9999px;
      font-size: 11px; font-weight: 600;
    }
    .status-pill--success {
      background: color-mix(in oklch, oklch(0.65 0.18 145) 18%, transparent);
      color: oklch(0.45 0.18 145);
    }
    .status-pill--warn {
      background: color-mix(in oklch, oklch(0.75 0.16 75) 22%, transparent);
      color: oklch(0.50 0.16 75);
    }
    .status-pill--danger {
      background: color-mix(in oklch, var(--destructive) 15%, transparent);
      color: var(--destructive);
    }
    .status-pill--muted { background: var(--muted); color: var(--muted-foreground); }

    /* Live admin preview */
    .admin-preview {
      border: 1px solid var(--border); border-radius: 16px;
      overflow: hidden; box-shadow: var(--shadow-lg);
      background: var(--card);
    }
    .admin-preview-bar {
      display: flex; align-items: center; gap: 8px;
      padding: 10px 16px; border-bottom: 1px solid var(--border);
      background: var(--muted);
    }
    .traffic-light { width: 12px; height: 12px; border-radius: 50%; }
    .traffic-light.red { background: oklch(0.7 0.2 25); }
    .traffic-light.yellow { background: oklch(0.78 0.18 90); }
    .traffic-light.green { background: oklch(0.7 0.18 145); }
    .admin-preview-url {
      flex: 1; text-align: center; font-family: 'SF Mono', Monaco, monospace;
      font-size: 12px; color: var(--muted-foreground);
      padding: 4px 12px; background: var(--card); border-radius: 6px;
      border: 1px solid var(--border);
    }
    .admin-preview-frame {
      display: flex; min-height: 380px; max-height: 480px;
    }
    .admin-side {
      width: 220px; background: var(--card);
      border-right: 1px solid var(--border);
      padding: 16px 10px; flex-shrink: 0;
    }
    .admin-side-item {
      display: flex; align-items: center; gap: 10px;
      padding: 7px 10px; border-radius: 6px;
      font-size: 13px; color: var(--foreground); margin-bottom: 2px;
    }
    .admin-side-item.active {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary); font-weight: 600;
    }
    .admin-main { flex: 1; padding: 24px; overflow: hidden; background: var(--background); }
    .admin-main h4 { font-size: 18px; font-weight: 700; margin: 0 0 4px; letter-spacing: -0.01em; }
    .admin-main p.muted { font-size: 12px; color: var(--muted-foreground); margin: 0 0 16px; }
    .admin-toolbar {
      display: flex; gap: 8px; margin-bottom: 12px;
    }
    .admin-toolbar input {
      flex: 1; max-width: 240px;
      padding: 6px 10px; border-radius: 6px; border: 1px solid var(--border);
      background: var(--card); font-size: 12px;
    }
    .admin-table {
      background: var(--card); border: 1px solid var(--border);
      border-radius: 8px; overflow: hidden; box-shadow: var(--shadow-xs);
    }
    .admin-row {
      display: grid; grid-template-columns: 1fr auto auto;
      gap: 12px; padding: 10px 14px; align-items: center;
      border-top: 1px solid var(--border); font-size: 13px;
    }
    .admin-row:first-child {
      border-top: none; background: var(--muted);
      font-weight: 600; font-size: 11px; text-transform: uppercase;
      letter-spacing: 0.04em; color: var(--muted-foreground);
    }
    .admin-avatar {
      width: 26px; height: 26px; border-radius: 50%;
      background: color-mix(in oklch, var(--primary) 15%, transparent);
      color: var(--primary); display: inline-flex; align-items: center; justify-content: center;
      font-size: 11px; font-weight: 600;
    }
    .admin-row-name { display: flex; align-items: center; gap: 10px; }

    /* Footer */
    footer.demo-footer {
      padding: 32px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 13px;
      border-top: 1px solid var(--border);
      margin-top: 32px;
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @state() private _currentTheme: 'ember-light' | 'ember-dark' = 'ember-light';

  private _setTheme(t: 'ember-light' | 'ember-dark'): void {
    this._currentTheme = t;
    this.dispatchEvent(
      new CustomEvent('open-routing:theme-change', {
        detail: { theme: t }, bubbles: true, composed: true,
      })
    );
  }

  override render() {
    return html`
      <header class="demo-top">
        <div class="brand">
          <span class="logo"><uk-icon icon="route" height="18" width="18"></uk-icon></span>
          <div class="brand-text">Open Routing<small>Design System</small></div>
        </div>
        <nav>
          <a href="#tokens">Tokens</a>
          <a href="#components">Components</a>
          <a href="#preview">Preview</a>
        </nav>
        <div class="theme-switch">
          <button class="theme-pill ${this._currentTheme === 'ember-light' ? 'active' : ''}"
                  @click=${() => this._setTheme('ember-light')}>
            <uk-icon icon="sun" height="14" width="14"></uk-icon>Light
          </button>
          <button class="theme-pill ${this._currentTheme === 'ember-dark' ? 'active' : ''}"
                  @click=${() => this._setTheme('ember-dark')}>
            <uk-icon icon="moon" height="14" width="14"></uk-icon>Dark
          </button>
        </div>
      </header>

      <section class="hero">
        <div class="hero-badge">
          <uk-icon icon="sparkles" height="12" width="12"></uk-icon>
          Now in Catalog v0.1
        </div>
        <h1>The modern admin for <span class="accent">routing catalogs</span></h1>
        <p>
          Open Routing Catalog gives operations teams a fast, focused interface for managing
          agents, skills, queues, and the channels that connect them — built on a typed
          API contract with zero drift between server and client.
        </p>
        <div class="hero-cta">
          <a class="btn-primary" href="#preview">
            See it in action <uk-icon icon="arrow-right" height="16" width="16"></uk-icon>
          </a>
          <a class="btn-ghost" href="#tokens">
            Browse design tokens
          </a>
        </div>
      </section>

      <section class="demo-section" id="tokens">
        <div class="section-header">
          <div class="section-eyebrow">Design Tokens</div>
          <h2>OKLCh color system</h2>
          <p>Every color is defined in perceptually-uniform OKLCh, so lightness shifts feel natural across the palette. Switch themes above to see all tokens adapt instantly.</p>
        </div>
        <div class="swatch-grid">
          ${[
            ['primary', 'var(--primary)', '--primary'],
            ['background', 'var(--background)', '--background'],
            ['card', 'var(--card)', '--card'],
            ['muted', 'var(--muted)', '--muted'],
            ['accent', 'var(--accent)', '--accent'],
            ['border', 'var(--border)', '--border'],
            ['destructive', 'var(--destructive)', '--destructive'],
            ['success', 'var(--success)', '--success'],
            ['warning', 'var(--warning)', '--warning'],
          ].map(([label, cssVar, token]) => html`
            <div class="swatch">
              <div class="swatch-color" style="background: ${cssVar}"></div>
              <div class="swatch-label">${label}</div>
              <div class="swatch-token">${token}</div>
            </div>
          `)}
        </div>
      </section>

      <section class="demo-section" id="components">
        <div class="section-header">
          <div class="section-eyebrow">Components</div>
          <h2>Built on Frankenstyle + Lit</h2>
          <p>HTML-first Hardened Web Components with Shadow DOM isolation. Each primitive adopts the Frankenstyle stylesheet directly into its shadow root.</p>
        </div>
        <div class="preview-grid">
          <div class="preview-card">
            <h3>Buttons</h3>
            <div class="preview-row">
              <button class="uk-button uk-button-primary">Primary</button>
              <button class="uk-button uk-button-default">Default</button>
              <button class="uk-button uk-button-danger">Destructive</button>
            </div>
            <div class="preview-row">
              <button class="uk-button uk-button-primary uk-button-small">Small</button>
              <button class="uk-button uk-button-default uk-button-small">Compact</button>
            </div>
          </div>
          <div class="preview-card">
            <h3>Inputs</h3>
            <div class="preview-row">
              <input class="uk-input" placeholder="Search agents…">
            </div>
            <div class="preview-row">
              <input class="uk-input" placeholder="With value" value="agent_voice_en">
            </div>
          </div>
          <div class="preview-card">
            <h3>Status badges</h3>
            <div class="preview-row">
              <span class="status-pill status-pill--success">Active</span>
              <span class="status-pill status-pill--warn">Moderate</span>
              <span class="status-pill status-pill--danger">Failed</span>
              <span class="status-pill status-pill--muted">Disabled</span>
            </div>
          </div>
          <div class="preview-card">
            <h3>Avatar + identity</h3>
            <div class="preview-row">
              <div class="admin-row-name">
                <span class="admin-avatar">AS</span>
                <div>
                  <div style="font-weight:500;font-size:14px">Aigars Silkalns</div>
                  <div style="font-family:monospace;font-size:11px;color:var(--muted-foreground)">aigars_silkalns</div>
                </div>
              </div>
            </div>
            <div class="preview-row">
              <div class="admin-row-name">
                <span class="admin-avatar">MS</span>
                <div>
                  <div style="font-weight:500;font-size:14px">Maria Santos</div>
                  <div style="font-family:monospace;font-size:11px;color:var(--muted-foreground)">maria_santos</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="demo-section" id="preview">
        <div class="section-header">
          <div class="section-eyebrow">Live Preview</div>
          <h2>Inside the admin</h2>
          <p>This is what your team sees after signing in. Toggle the theme above to preview both modes.</p>
        </div>
        <div class="admin-preview">
          <div class="admin-preview-bar">
            <span class="traffic-light red"></span>
            <span class="traffic-light yellow"></span>
            <span class="traffic-light green"></span>
            <span class="admin-preview-url">open-routing.example/orgs/.../agents</span>
          </div>
          <div class="admin-preview-frame">
            <aside class="admin-side">
              <div class="admin-side-item active">
                <uk-icon icon="users" height="14" width="14"></uk-icon> Agents
              </div>
              <div class="admin-side-item">
                <uk-icon icon="tag" height="14" width="14"></uk-icon> Skills
              </div>
              <div class="admin-side-item">
                <uk-icon icon="filter" height="14" width="14"></uk-icon> Queues
              </div>
              <div class="admin-side-item">
                <uk-icon icon="radio" height="14" width="14"></uk-icon> Channels
              </div>
              <div class="admin-side-item">
                <uk-icon icon="plug" height="14" width="14"></uk-icon> Adapters
              </div>
              <div class="admin-side-item">
                <uk-icon icon="pause-circle" height="14" width="14"></uk-icon> Break Reasons
              </div>
            </aside>
            <main class="admin-main">
              <h4>Agents</h4>
              <p class="muted">Manage team members, roles, and access.</p>
              <div class="admin-toolbar">
                <input placeholder="Search agents…" />
                <button class="uk-button uk-button-primary uk-button-small">+ Add Agent</button>
              </div>
              <div class="admin-table">
                <div class="admin-row">
                  <span>Name</span>
                  <span>Status</span>
                  <span></span>
                </div>
                ${[
                  ['AS', 'Aigars Silkalns', 'aigars_silkalns', 'success', 'Active'],
                  ['MS', 'Maria Santos', 'maria_santos', 'success', 'Active'],
                  ['JC', 'James Chen', 'james_chen', 'muted', 'Disabled'],
                  ['EW', 'Emma Wilson', 'emma_wilson', 'success', 'Active'],
                ].map(([initials, name, code, variant, label]) => html`
                  <div class="admin-row">
                    <div class="admin-row-name">
                      <span class="admin-avatar">${initials}</span>
                      <div>
                        <div style="font-weight:500;font-size:13px">${name}</div>
                        <div style="font-family:monospace;font-size:11px;color:var(--muted-foreground)">${code}</div>
                      </div>
                    </div>
                    <span class="status-pill status-pill--${variant}">${label}</span>
                    <span></span>
                  </div>
                `)}
              </div>
            </main>
          </div>
        </div>
      </section>

      <footer class="demo-footer">
        Open Routing &middot; Catalog v0.1 &middot;
        Designed with Frankenstyle, Lit, and the OKLCh color space
      </footer>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-playground-route': OrPlaygroundRoute;
  }
}
