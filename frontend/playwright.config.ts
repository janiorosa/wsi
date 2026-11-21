import { defineConfig } from '@playwright/test';

// Testes ponta a ponta contra o sistema em execução (docker compose up).
// WSI_URL_BASE=http://localhost:8080 npx playwright test
export default defineConfig({
  testDir: './tests/e2e',
  timeout: 60_000,
  use: {
    baseURL: process.env.WSI_URL_BASE ?? 'http://localhost:8080',
    locale: 'pt-BR',
    timezoneId: 'America/Sao_Paulo',
    trace: 'retain-on-failure',
  },
});
