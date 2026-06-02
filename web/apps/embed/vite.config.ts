import { defineConfig } from 'vite';
import { resolve } from 'node:path';
import babel from '@rolldown/plugin-babel';

export default defineConfig({
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
      '/readyz': 'http://localhost:8080'
    }
  },
  build: {
    target: 'esnext',
    sourcemap: true,
    cssCodeSplit: false,
    lib: {
      entry: resolve(import.meta.dirname, 'src/index.ts'),
      formats: ['es'],
      fileName: () => 'embed.js'
    },
    rollupOptions: {
      output: {
        chunkFileNames: 'embed-[name]-[hash].js'
      }
    }
  },
  publicDir: 'e2e/hosts'
});
