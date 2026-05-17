import tsParser from '@typescript-eslint/parser';
import tsPlugin from '@typescript-eslint/eslint-plugin';

export default [
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 2022,
        sourceType: 'module',
        project: false,
      },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
    },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      // Allow _-prefixed parameters and variables to be unused (standard TypeScript
      // convention for intentionally unused callback params, e.g. _url/_init).
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
  // Test file overrides: shadow DOM testing requires `any` casts for reactive
  // property access on Lit custom elements (no public TS interface at test time).
  {
    files: ['**/*.test.ts'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  {
    ignores: [
      '**/node_modules/**',
      '**/dist/**',
      '**/.turbo/**',
      // Pattern S1: generated code follows generator-only discipline — no lint
      'packages/ui/src/api/generated.ts',
      // ajv standalone output: // @ts-nocheck JS-style code; not conformant to ESLint rules
      'packages/ui/src/validators/**',
    ],
  },
];
