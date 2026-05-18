import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss()],
  appType: 'spa',
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
