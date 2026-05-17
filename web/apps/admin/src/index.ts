// Source: CONTEXT.md D6-09, D6-20, D6-21, D6-22
// App bootstrap: configure i18n and register the shell component.
// Shell component reads org_id from URL path and feeds it to the router.
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';
import '@open-routing/ui/components/shell'; // registers <or-catalog-shell>

type AppLocale = typeof sourceLocale | (typeof targetLocales)[number];
const VALID_LOCALES: ReadonlySet<string> = new Set([sourceLocale, ...targetLocales]);

const { setLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  // Known typing limitation of @lit/localize — locale modules are typed as
  // Record<string, unknown> by the generator until `lit-localize build` runs.
  // Cast through unknown to satisfy the configureLocalization contract.
  // (Verifier-flagged ship fix; once lit-localize build runs the cast can drop.)
  loadLocale: (locale: string) =>
    import(`@open-routing/ui/locales/${locale}.js`) as unknown as ReturnType<
      Parameters<typeof configureLocalization>[0]['loadLocale']
    >,
});

// Locale detection: validate localStorage override → navigator.language → 'en'
const saved = localStorage.getItem('or-locale');
const detected: AppLocale = navigator.language.startsWith('vi') ? 'vi' : 'en';
const locale: AppLocale =
  saved !== null && VALID_LOCALES.has(saved) ? (saved as AppLocale) : detected;
await setLocale(locale);
