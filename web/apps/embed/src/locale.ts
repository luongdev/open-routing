// web/apps/embed/src/locale.ts
import { configureLocalization } from '@lit/localize';
import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';

export type EmbedLocale = typeof sourceLocale | (typeof targetLocales)[number];

const VALID_LOCALES: ReadonlySet<string> = new Set([sourceLocale, ...targetLocales]);

const { setLocale, getLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  loadLocale: (locale: string) =>
    import(`@open-routing/ui/locales/${locale}.js`) as unknown as ReturnType<
      Parameters<typeof configureLocalization>[0]['loadLocale']
    >,
});

export async function applyEmbedLocale(raw: string): Promise<void> {
  const locale: EmbedLocale = VALID_LOCALES.has(raw) ? (raw as EmbedLocale) : sourceLocale;
  if (getLocale() === locale) return;
  await setLocale(locale);
}