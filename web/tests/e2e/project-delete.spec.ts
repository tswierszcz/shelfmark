import { test, expect } from '@playwright/test';
import pg from 'pg';

const token = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
const projectName = `E2E delete ${token}`;
const purlPrefix = `pkg:maven/com.e2e-${token}/`;
const sbom = {
  bomFormat: 'CycloneDX',
  specVersion: '1.5',
  version: 1,
  metadata: { component: { type: 'application', 'bom-ref': 'app', name: projectName } },
  components: [{ type: 'library', 'bom-ref': 'lib', name: 'delete-me', purl: `${purlPrefix}delete-me@1.0.0` }],
  dependencies: [{ ref: 'app', dependsOn: ['lib'] }],
};

test.afterEach(async () => {
  const db = new pg.Client({ connectionString: process.env.E2E_DATABASE_URL });
  await db.connect();
  try {
    await db.query('DELETE FROM projects WHERE name = $1', [projectName]);
    await db.query('DELETE FROM libraries WHERE purl LIKE $1', [`${purlPrefix}%`]);
  } finally {
    await db.end();
  }
});

test('architect can confirm project deletion', async ({ page }) => {
  await page.goto('/redundancy');
  await expect(page.getByRole('heading', { name: 'Upload project SBOM' })).toBeVisible();
  await page.getByRole('textbox', { name: 'Project name' }).fill(projectName);
  await page.getByLabel('File').setInputFiles({
    name: `${token}.cdx.json`,
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify(sbom)),
  });
  await page.getByRole('button', { name: 'Upload' }).click();
  await expect(page.getByRole('combobox', { name: 'Project' }).getByRole('option', { name: projectName, selected: true })).toBeAttached();
  await page.getByRole('button', { name: 'Delete project' }).click();
  await page.getByRole('button', { name: `Confirm delete ${projectName}` }).click();
  await expect(page.getByRole('combobox', { name: 'Project' }).getByRole('option', { name: projectName })).toHaveCount(0);
  await expect(page.getByRole('combobox', { name: 'Project' })).toHaveValue('');
  await page.reload();
  await expect(page.getByRole('combobox', { name: 'Project' }).getByRole('option', { name: projectName })).toHaveCount(0);
});
