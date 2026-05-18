import { defineConfig } from 'vite';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
  appType: 'spa',
  plugins: [
    babel({
      preset: () => ({
        plugins: [['@babel/plugin-proposal-decorators', { version: '2023-11' }]]
      }),
      rolldown: {
        filter: { code: '@' }
      }
    })
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
