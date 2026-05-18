import { defineConfig } from 'vite';

export default defineConfig({
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
