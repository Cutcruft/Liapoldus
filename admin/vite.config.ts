import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@liapoldus/ui-kit': fileURLToPath(new URL('../ui-kit/src/index.ts', import.meta.url)),
      '@liapoldus/ui-runtime': fileURLToPath(new URL('../ui-runtime/src/index.ts', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/runtime': { target: 'http://localhost:18080', changeOrigin: true },
      '/build': { target: 'http://localhost:18080', changeOrigin: true },
      '/dev': { target: 'http://localhost:18080', changeOrigin: true, ws: true },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test-setup.ts'],
  },
});