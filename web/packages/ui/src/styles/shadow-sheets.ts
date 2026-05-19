// Shared constructed CSSStyleSheets for Shadow DOM hosts that render uk-* descendants.
//
// Frankenstyle CSS lives at :root in the main document. Shadow DOM roots cannot
// inherit from it — they need the sheets adopted explicitly. ?inline imports give
// us the raw CSS text at bundle time so we can call sheet.replaceSync() once and
// share the same CSSStyleSheet object across all shadow roots (spec §4.1: one
// CSSStyleSheet instance can be adopted by multiple shadow roots with no copy).
//
// Usage: call adoptShadowSheets(this.shadowRoot) in createRenderRoot() for any
// LitElement that uses Shadow DOM and renders uk-* class descendants.

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

export function getShadowSheets(): CSSStyleSheet[] {
  if (!_frankenSheet) _frankenSheet = makeSheet(frankenCss);
  if (!_emberVarsSheet) _emberVarsSheet = makeSheet(emberVarsCss);
  if (!_compatOrSheet) _compatOrSheet = makeSheet(compatOrTokensCss);
  if (!_polishSheet) _polishSheet = makeSheet(emberPolishCss);
  // Polish LAST so it overrides Frankenstyle defaults.
  return [_frankenSheet, _emberVarsSheet, _compatOrSheet, _polishSheet];
}

/**
 * Adopt Frankenstyle + Ember token sheets into a shadow root.
 * Call this inside createRenderRoot() after super.createRenderRoot():
 *
 *   override createRenderRoot() {
 *     const root = super.createRenderRoot() as ShadowRoot;
 *     adoptShadowSheets(root);
 *     return root;
 *   }
 */
export function adoptShadowSheets(root: ShadowRoot): void {
  root.adoptedStyleSheets = [...root.adoptedStyleSheets, ...getShadowSheets()];
}
