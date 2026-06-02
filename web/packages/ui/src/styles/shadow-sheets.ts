import frankenCss from 'frankenstyle/css/frankenstyle-kit.css?inline';
import emberVarsCss from './ember-shadow-vars.css?inline';
import compatOrTokensCss from './compat-or-tokens.css?inline';
import emberPolishCss from './ember-polish.css?inline';

function makeSheet(css: string): CSSStyleSheet {
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(css);
  return sheet;
}

// Lazily created — only built once on first import, shared across all consumers.
let _frankenSheet: CSSStyleSheet | null = null;
let _emberVarsSheet: CSSStyleSheet | null = null;
let _compatOrSheet: CSSStyleSheet | null = null;
let _polishSheet: CSSStyleSheet | null = null;
let _darkObserver: MutationObserver | null = null;
const _darkHosts = new Set<Element>();

export function getShadowSheets(): CSSStyleSheet[] {
  if (!_frankenSheet) _frankenSheet = makeSheet(frankenCss);
  if (!_emberVarsSheet) _emberVarsSheet = makeSheet(emberVarsCss);
  if (!_compatOrSheet) _compatOrSheet = makeSheet(compatOrTokensCss);
  if (!_polishSheet) _polishSheet = makeSheet(emberPolishCss);
  return [_frankenSheet, _emberVarsSheet, _compatOrSheet, _polishSheet];
}

function syncDarkHosts(): void {
  const dark = document.documentElement.classList.contains('dark');
  for (const host of [..._darkHosts]) {
    if (!host.isConnected) {
      _darkHosts.delete(host);
      continue;
    }
    host.classList.toggle('dark', dark);
  }
}

function registerDarkHost(host: Element): void {
  _darkHosts.add(host);
  syncDarkHosts();
  if (_darkObserver) return;
  _darkObserver = new MutationObserver(syncDarkHosts);
  _darkObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
}

export function adoptShadowSheets(root: ShadowRoot): void {
  root.adoptedStyleSheets = [...root.adoptedStyleSheets, ...getShadowSheets()];
  registerDarkHost(root.host);
}
