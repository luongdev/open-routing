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
