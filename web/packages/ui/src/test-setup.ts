// urlpattern-polyfill is needed in Node/happy-dom test environments for @lit-labs/router
// route parsing. All major browsers support URLPattern natively as of 2025 (Baseline 2025).
// Do NOT import this in any component source file — only in test setup.
import 'urlpattern-polyfill';
