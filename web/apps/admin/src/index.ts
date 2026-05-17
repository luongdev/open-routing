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
  loadLocale: (locale: string) =>
    import(`@open-routing/ui/locales/${locale}.js`) as Promise<Record<string, unknown>>,
});

// Locale detection: validate localStorage override → navigator.language → 'en'
const saved = localStorage.getItem('or-locale');
const detected: AppLocale = navigator.language.startsWith('vi') ? 'vi' : 'en';
const locale: AppLocale =
  saved !== null && VALID_LOCALES.has(saved) ? (saved as AppLocale) : detected;
await setLocale(locale);
