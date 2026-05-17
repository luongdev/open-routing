/**
 * Theme token maps for the <or-catalog-shell> _applyTheme() method.
 *
 * These are the JS-side equivalents of the CSS files in this directory.
 * The shell component imports these and calls style.setProperty(key, val)
 * on the host element so CSS custom properties cascade through all
 * nested Shadow DOM children — including Shoelace components.
 *
 * Per D6-19 / D6-20 / UI-SPEC §2.2.
 */

export type ThemeName = 'or-light' | 'or-dark' | 'or-brand';

/**
 * Theme is either a named preset or an arbitrary token map (Phase 7 embed).
 */
export type Theme = ThemeName | Record<string, string>;

/**
 * or-light token map — clean white shell, mid-tone teal primary (hue=185, sat=43%).
 * or-light is the default; values shown are overrides from Shoelace defaults.
 */
export const orLight: Record<string, string> = {
  '--sl-color-primary-50': '#f0fafa',
  '--sl-color-primary-100': '#d3f1f2',
  '--sl-color-primary-200': '#aae0e2',
  '--sl-color-primary-300': '#79c9ce',
  '--sl-color-primary-400': '#4faab2',
  '--sl-color-primary-500': '#2b8a93',
  '--sl-color-primary-600': '#1f6e77',
  '--sl-color-primary-700': '#1a575f',
  '--sl-color-primary-800': '#18464c',
  '--sl-color-primary-900': '#173a3f',
  '--sl-color-primary-950': '#0c2024',
  '--sl-color-danger-500': '#d92d20',
  '--sl-color-warning-500': '#b54708',
  '--sl-color-success-500': '#027a48',
  '--sl-color-neutral-0': '#ffffff',
  '--sl-color-neutral-50': '#fafafa',
  '--sl-color-neutral-100': '#f5f5f5',
  '--sl-color-neutral-200': '#e5e5e5',
  '--sl-color-neutral-300': '#d4d4d4',
  '--sl-color-neutral-500': '#737373',
  '--sl-color-neutral-700': '#404040',
  '--sl-color-neutral-900': '#171717',
  '--sl-color-neutral-1000': '#000000',
  '--or-color-app-bg': 'var(--sl-color-neutral-0)',
  '--or-color-sidebar-bg': 'var(--sl-color-neutral-100)',
  '--or-color-topbar-bg': 'var(--sl-color-neutral-0)',
  '--or-color-card-bg': 'var(--sl-color-neutral-0)',
  '--or-color-card-border': 'var(--sl-color-neutral-200)',
  '--or-color-text-body': 'var(--sl-color-neutral-700)',
  '--or-color-text-strong': 'var(--sl-color-neutral-900)',
  '--or-color-text-muted': 'var(--sl-color-neutral-500)',
  '--or-color-text-on-primary': '#ffffff',
  '--or-color-conflict-bg': '#fef3c7',
  '--or-color-conflict-border': '#f59e0b',
  '--or-color-diff-removed': '#fecaca',
  '--or-color-diff-added': '#bbf7d0',
  '--or-color-code-bg': 'var(--sl-color-neutral-100)',
  '--or-color-code-fg': 'var(--sl-color-primary-700)',
  '--or-color-focus-ring': 'var(--sl-color-primary-500)',
  '--or-color-row-hover': 'var(--sl-color-neutral-50)',
  '--or-color-row-selected': 'var(--sl-color-primary-50)',
  '--or-color-skeleton-base': 'var(--sl-color-neutral-200)',
  '--or-color-skeleton-shimmer': 'var(--sl-color-neutral-100)',
  '--or-color-divider': 'var(--sl-color-neutral-200)',
  '--or-color-overlay-scrim': 'rgb(0 0 0 / 0.4)',
};

/**
 * or-dark token map — dark surfaces, lighter teal primary so it reads on dark backgrounds.
 */
export const orDark: Record<string, string> = {
  '--sl-color-primary-50': '#0c2024',
  '--sl-color-primary-100': '#173a3f',
  '--sl-color-primary-200': '#18464c',
  '--sl-color-primary-300': '#1a575f',
  '--sl-color-primary-400': '#1f6e77',
  '--sl-color-primary-500': '#4faab2',
  '--sl-color-primary-600': '#79c9ce',
  '--sl-color-primary-700': '#aae0e2',
  '--sl-color-primary-800': '#d3f1f2',
  '--sl-color-primary-900': '#f0fafa',
  '--sl-color-primary-950': '#ffffff',
  '--sl-color-danger-500': '#f97066',
  '--sl-color-warning-500': '#fdb022',
  '--sl-color-success-500': '#32d583',
  '--sl-color-neutral-0': '#0a0a0a',
  '--sl-color-neutral-50': '#121212',
  '--sl-color-neutral-100': '#1a1a1a',
  '--sl-color-neutral-200': '#262626',
  '--sl-color-neutral-300': '#404040',
  '--sl-color-neutral-500': '#a3a3a3',
  '--sl-color-neutral-700': '#d4d4d4',
  '--sl-color-neutral-900': '#f5f5f5',
  '--sl-color-neutral-1000': '#ffffff',
  '--or-color-app-bg': 'var(--sl-color-neutral-50)',
  '--or-color-sidebar-bg': 'var(--sl-color-neutral-100)',
  '--or-color-topbar-bg': 'var(--sl-color-neutral-50)',
  '--or-color-card-bg': 'var(--sl-color-neutral-100)',
  '--or-color-card-border': 'var(--sl-color-neutral-200)',
  '--or-color-text-body': 'var(--sl-color-neutral-700)',
  '--or-color-text-strong': 'var(--sl-color-neutral-900)',
  '--or-color-text-muted': 'var(--sl-color-neutral-500)',
  '--or-color-text-on-primary': 'var(--sl-color-neutral-900)',
  '--or-color-conflict-bg': '#44331b',
  '--or-color-conflict-border': '#fdb022',
  '--or-color-diff-removed': '#5b1d1d',
  '--or-color-diff-added': '#1d4d2c',
  '--or-color-code-bg': 'var(--sl-color-neutral-200)',
  '--or-color-code-fg': 'var(--sl-color-primary-500)',
  '--or-color-focus-ring': 'var(--sl-color-primary-500)',
  '--or-color-row-hover': 'var(--sl-color-neutral-200)',
  '--or-color-row-selected': '#143036',
  '--or-color-skeleton-base': 'var(--sl-color-neutral-200)',
  '--or-color-skeleton-shimmer': 'var(--sl-color-neutral-300)',
  '--or-color-divider': 'var(--sl-color-neutral-200)',
  '--or-color-overlay-scrim': 'rgb(0 0 0 / 0.6)',
};

/**
 * or-brand token map — deeper, more saturated brand teal (hue=190, sat=60%).
 * Sidebar gets a subtle brand-color tint for visual identity.
 */
export const orBrand: Record<string, string> = {
  '--sl-color-primary-50': '#ebfafa',
  '--sl-color-primary-100': '#cdf0f1',
  '--sl-color-primary-200': '#9be0e3',
  '--sl-color-primary-300': '#5ec9cf',
  '--sl-color-primary-400': '#2dadb7',
  '--sl-color-primary-500': '#0d8b96',
  '--sl-color-primary-600': '#086e78',
  '--sl-color-primary-700': '#075a63',
  '--sl-color-primary-800': '#094a52',
  '--sl-color-primary-900': '#0a3d44',
  '--sl-color-primary-950': '#03252a',
  '--sl-color-danger-500': '#d92d20',
  '--sl-color-warning-500': '#b54708',
  '--sl-color-success-500': '#027a48',
  '--sl-color-neutral-0': '#ffffff',
  '--sl-color-neutral-50': '#f8fafa',
  '--sl-color-neutral-100': '#eef5f5',
  '--sl-color-neutral-200': '#d9e7e8',
  '--sl-color-neutral-300': '#b8cdce',
  '--sl-color-neutral-500': '#678384',
  '--sl-color-neutral-700': '#364949',
  '--sl-color-neutral-900': '#0e1e1e',
  '--sl-color-neutral-1000': '#000000',
  '--or-color-app-bg': 'var(--sl-color-neutral-0)',
  '--or-color-sidebar-bg': 'var(--sl-color-neutral-100)',
  '--or-color-topbar-bg': 'var(--sl-color-neutral-0)',
  '--or-color-card-bg': 'var(--sl-color-neutral-0)',
  '--or-color-card-border': 'var(--sl-color-neutral-200)',
  '--or-color-text-body': 'var(--sl-color-neutral-700)',
  '--or-color-text-strong': 'var(--sl-color-neutral-900)',
  '--or-color-text-muted': 'var(--sl-color-neutral-500)',
  '--or-color-text-on-primary': '#ffffff',
  '--or-color-conflict-bg': '#fef3c7',
  '--or-color-conflict-border': '#f59e0b',
  '--or-color-diff-removed': '#fecaca',
  '--or-color-diff-added': '#bbf7d0',
  '--or-color-code-bg': 'var(--sl-color-neutral-100)',
  '--or-color-code-fg': 'var(--sl-color-primary-700)',
  '--or-color-focus-ring': 'var(--sl-color-primary-500)',
  '--or-color-row-hover': 'var(--sl-color-neutral-50)',
  '--or-color-row-selected': 'var(--sl-color-primary-50)',
  '--or-color-skeleton-base': 'var(--sl-color-neutral-200)',
  '--or-color-skeleton-shimmer': 'var(--sl-color-neutral-100)',
  '--or-color-divider': 'var(--sl-color-neutral-200)',
  '--or-color-overlay-scrim': 'rgb(0 0 0 / 0.4)',
};

/**
 * Lookup map for named themes.
 * Used by <or-catalog-shell>._applyTheme() to resolve ThemeName → token map.
 */
export const THEME_TOKENS: Record<ThemeName, Record<string, string>> = {
  'or-light': orLight,
  'or-dark': orDark,
  'or-brand': orBrand,
};
