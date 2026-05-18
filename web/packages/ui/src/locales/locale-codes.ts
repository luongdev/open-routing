/**
 * Locale code constants for @lit/localize (D6-22).
 *
 * This file is initially hand-authored and will be overwritten when
 * `lit-localize build` runs. Do not edit manually after localization
 * pipeline is active.
 *
 * Usage in apps/admin:
 *   import { sourceLocale, targetLocales } from '@open-routing/ui/locales/locale-codes.js';
 *   const { setLocale } = configureLocalization({ sourceLocale, targetLocales, loadLocale });
 */

export const sourceLocale = 'en' as const;
export const targetLocales = ['vi'] as const;
export const allLocales = [sourceLocale, ...targetLocales] as const;
export type LocaleCode = (typeof allLocales)[number];
