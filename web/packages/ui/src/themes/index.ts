// Phase 6: Theme token maps for or-light, or-dark, or-brand.
// Applied as CSS custom properties on <or-catalog-shell> host element (D6-20).
// Each map entry is a [CSS custom property, value] pair.
// var() references are resolved by the browser at paint time via CSS cascade.

export type ThemeName = 'or-light' | 'or-dark' | 'or-brand';

/**
 * or-light: Clean white app shell, neutral grays, mid-tone teal primary.
 * --sl-color-primary-500 = #2b8a93
 */
export const orLight: Record<string, string> = {
  // Shoelace primary scale (teal, hue=185, sat=43%)
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
  // Shoelace semantic
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
  // Open Routing custom
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
 * or-dark: Dark variant. --sl-color-primary-500 = #4faab2 (lighter to pop on dark).
 */
export const orDark: Record<string, string> = {
  // Shoelace primary scale (teal, brighter so it sits on dark)
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
  // Shoelace semantic (lifted for dark contrast)
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
  // Open Routing custom
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
 * or-brand: Bold brand identity. --sl-color-primary-500 = #0d8b96 (deep petrol teal).
 */
export const orBrand: Record<string, string> = {
  // Shoelace primary scale (deeper teal, hue=190, sat=60%)
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
  // Shoelace semantic (same as or-light)
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
  // Open Routing custom
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

/** Union of all token keys across all themes. Used for cleanup before switching. */
export const ALL_TOKEN_KEYS: readonly string[] = Array.from(
  new Set([...Object.keys(orLight), ...Object.keys(orDark), ...Object.keys(orBrand)])
);
