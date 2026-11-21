/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Em desenvolvimento, a API Go roda em localhost:8080.
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./tests/unidade/configuracao.ts'],
    include: ['tests/unidade/**/*.test.{ts,tsx}'],
  },
});
