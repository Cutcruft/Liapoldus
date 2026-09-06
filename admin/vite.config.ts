import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// Proxy targets come from the environment so the same config works out of the
// box on the host (defaults) and with an explicit VITE_API_URL later.
const API_TARGET = process.env.VITE_API_URL || 'http://localhost:8080';
const CLIENT_TARGET = process.env.VITE_CLIENT_URL || 'http://localhost:18080';

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
      '/api': { target: API_TARGET, changeOrigin: true, ws: true },
      '/runtime': { target: CLIENT_TARGET, changeOrigin: true },
      '/build': { target: CLIENT_TARGET, changeOrigin: true },
      '/dev': { target: CLIENT_TARGET, changeOrigin: true, ws: true },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test-setup.ts'],
  },
});