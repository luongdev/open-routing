// web/apps/embed/src/locale.ts
import { configureLocalization } from '@lit/localize';

const sourceLocale = 'en' as const;
const targetLocales = ['vi'] as const;

export type EmbedLocale = typeof sourceLocale | (typeof targetLocales)[number];

const VALID_LOCALES: ReadonlySet<string> = new Set([sourceLocale, ...targetLocales]);

const { setLocale, getLocale } = configureLocalization({
  sourceLocale,
  targetLocales,
  loadLocale: (locale: string) => {
    if (locale === 'vi') {
      return import('./locales/vi.js') as unknown as ReturnType<
        Parameters<typeof configureLocalization>[0]['loadLocale']
      >;
    }
    return Promise.resolve({ templates: {} });
  },
});

export async function applyEmbedLocale(raw: string): Promise<void> {
  const locale: EmbedLocale = VALID_LOCALES.has(raw) ? (raw as EmbedLocale) : sourceLocale;
  if (getLocale() === locale) return;
  await setLocale(locale);
}
