import '@open-routing/ui/styles/frankenstyle.css';
import '@open-routing/ui/components/hwc-bootstrap.js';
import '@shoelace-style/shoelace/dist/themes/light.css';
import { setBasePath } from '@shoelace-style/shoelace/dist/utilities/base-path.js';
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';

// Must run before the shell import so sl-icon connectedCallback sees the path.
// Static imports are hoisted before module body code, so the shell is loaded
// via dynamic import to guarantee ordering.
setBasePath('/shoelace/');
await import('@open-routing/ui/components/shell');

type AppLocale = typeof sourceLocale | (typeof targetLocales)[number];
const VALID_LOCALES: ReadonlySet<string> = new Set([sourceLocale, ...targetLocales]);

// STATIC per-locale loaders: a template-literal dynamic import of a BARE package
// specifier (`@open-routing/ui/locales/${locale}.js`) can't be statically analysed
// by Vite, so it shipped as an unresolvable bare import that threw at runtime.
// Listing each target locale as a literal import() lets Vite bundle it as a chunk.
const localeLoaders: Record<string, () => Promise<unknown>> = {
  vi: () => import('@open-routing/ui/locales/vi.js'),
};

const { setLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  // Cast through unknown — @lit/localize types locale modules as Record<string,
  // unknown> until `lit-localize build` runs.
  loadLocale: (locale: string) =>
    (localeLoaders[locale]?.() ??
      Promise.reject(new Error(`no locale bundle for ${locale}`))) as unknown as ReturnType<
      Parameters<typeof configureLocalization>[0]['loadLocale']
    >,
});

// Locale detection: validate localStorage override → navigator.language → 'en'.
const saved = localStorage.getItem('or-locale');
const detected: AppLocale = navigator.language.startsWith('vi') ? 'vi' : 'en';
const locale: AppLocale =
  saved !== null && VALID_LOCALES.has(saved) ? (saved as AppLocale) : detected;
// A locale-load failure must not break boot — fall back to the source locale.
try {
  await setLocale(locale);
} catch (err) {
  console.warn('locale load failed; using', sourceLocale, err);
}
