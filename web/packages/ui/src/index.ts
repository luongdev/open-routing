// Root barrel — Phase 2 API module + Phase 6 additions (D6-05, D6-40).
export * from './api';
// Phase 6: component and validator exports
// Note: components are empty stubs until Wave 2+ plans fill them in.
export * from './components';
export * from './validators';
export * from './api/import';
// Note: themes are CSS files + TS token maps — apps import from './themes/index.ts' directly.
// Note: locales are lazy-loaded via @lit/localize configureLocalization in apps/admin.
