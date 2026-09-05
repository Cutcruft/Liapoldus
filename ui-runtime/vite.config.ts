import { resolve } from 'node:path';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Vite library mode: ESM + CJS бандлы пакета (React-слой, jsx-js).
export default defineConfig({
  plugins: [react()],
  build: {
    lib: {
      entry: resolve(import.meta.dirname, 'src/index.ts'),
      name: 'LiapoldusUiRuntime',
      formats: ['es', 'cjs'],
      fileName: (format) => (format === 'es' ? 'runtime.mjs' : 'runtime.cjs'),
    },
    rollupOptions: {
      external: ['react', 'react-dom', 'zustand', 'react/jsx-runtime'],
    },
    sourcemap: true,
  },
});