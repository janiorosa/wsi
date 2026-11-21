import { expect, test } from '@playwright/test';

const unico = Date.now();

test('página inicial pública carrega', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Eventos de apresentação de TCC' })).toBeVisible();
});

test('login mostra validação junto ao campo', async ({ page }) => {
  await page.goto('/entrar');
  await page.getByRole('button', { name: 'Entrar', exact: true }).click();
  await expect(page.getByText('Informe um e-mail válido.')).toBeVisible();
});

test('aluno se cadastra e entra sem aprovação', async ({ page }) => {
  await page.goto('/cadastro');
  await page.getByLabel('Nome completo').fill('Aluno E2E Teste');
  await page.getByLabel('E-mail').fill(`aluno.e2e.${unico}@exemplo.com`);
  await page.getByLabel('Matrícula').fill(`E2E${unico % 100000}`);
  await page.getByLabel('Senha', { exact: true }).fill('senha-e2e-123');
  await page.getByLabel('Confirme a senha').fill('senha-e2e-123');
  await page.getByRole('button', { name: 'Criar conta de aluno' }).click();
  await expect(page).toHaveURL(/\/aluno$/);
  await expect(page.getByRole('link', { name: 'ENVIAR MEU TCC' }).first()).toBeVisible();
});

test('docente cadastrado fica aguardando aprovação', async ({ page }) => {
  await page.goto('/cadastro');
  await page.getByRole('tab', { name: 'Sou docente' }).click();
  await page.getByLabel('Nome completo').fill('Docente E2E Teste');
  await page.getByLabel('E-mail').fill(`docente.e2e.${unico}@exemplo.com`);
  await page.getByLabel('Senha', { exact: true }).fill('senha-e2e-123');
  await page.getByLabel('Confirme a senha').fill('senha-e2e-123');
  await page.getByRole('button', { name: 'Solicitar cadastro de docente' }).click();
  await expect(page).toHaveURL(/aguardando-aprovacao/);
  await expect(page.getByText('aguardando aprovação', { exact: false })).toBeVisible();
});

test('área administrativa exige autenticação', async ({ page }) => {
  await page.goto('/admin');
  await expect(page).toHaveURL(/\/entrar/);
});
