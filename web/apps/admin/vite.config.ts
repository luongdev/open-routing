// Source: vite.dev/config/server-options + github.com/vitejs/vite/discussions/21891
// @rolldown/plugin-babel is REQUIRED: Vite 8 Oxc does not lower TypeScript
// experimentalDecorators that Lit 3 uses (@customElement, @property, @state, @query).
import { defineConfig } from 'vite';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
  appType: 'spa',
  plugins: [
    babel({
      presets: [
        {
          preset: () => ({
            plugins: [['@babel/plugin-proposal-decorators', { version: '2023-11' }]],
          }),
          rolldown: { filter: { code: '@' } }, // only files containing decorators
        },
      ],
    }),
  ],
  server: {
    proxy: {
      '/v1': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
    },
  },
  build: {
    target: 'esnext',
    rollupOptions: {
      output: {
        // Rolldown (Vite 8) requires manualChunks as a function, not an object.
        manualChunks: (id: string) => {
          if (id.includes('@shoelace-style/shoelace')) return 'shoelace';
          if (
            id.includes('/node_modules/lit/') ||
            id.includes('/node_modules/@lit/reactive-element/') ||
            id.includes('/node_modules/@lit/task/')
          )
            return 'lit';
        },
      },
    },
  },
});
