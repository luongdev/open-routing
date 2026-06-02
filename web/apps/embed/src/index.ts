// Embed library entry — Phase 7 D7-15.
// Importing this module registers <open-routing-catalog> as a Custom
// Element (side-effect import of embed-element.js below) and bootstraps
// lit-localize so the shell's locale-aware strings can be loaded.
//
// Why deferred setLocale: admin reads locale from storage +
// browser language at module load; embed reads it per-element from
// the `locale` attribute (CONTEXT.md <out_of_scope>: no embed-side
// storage, no browser language detection). configureLocalization
// runs once (module-level) to register the loader; setLocale is
// triggered per-element in embed-element's lifecycle (D7-13 firing
// open-routing:request-context is the trigger).

import './locale.js';        // configureLocalization runs once
import './embed-element.js'; // customElements.define runs

export { OpenRoutingCatalog } from './embed-element.js';
export { applyEmbedLocale, type EmbedLocale } from './locale.js';