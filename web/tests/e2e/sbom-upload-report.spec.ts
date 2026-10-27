// Risk: sbom-upload-report-smoke — an architect uploads a project SBOM with
// libraries the catalog has never seen, and the report for that project lists
// every direct library with the category the categorization pipeline assigned.
// Real: auth (storageState), routing, upload API, queue, Postgres, report UI.
// Mocked: deps.dev, GitHub and OpenRouter (server-side calls, see
// tests/e2e/mocks/external-apis-mock.mjs). Modeled on seed.spec.ts.
import { test, expect } from '@playwright/test';
import pg from 'pg';

// A fresh token per run makes every purl new to the catalog, so the upload
// really goes through the categorization pipeline instead of reusing rows.
const token = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
const projectName = `E2E smoke ${token}`;
const purlPrefix = `pkg:maven/com.e2e-${token}/`;
const libraries = [
  { artifact: 'json-a', category: 'json-processing' },
  { artifact: 'json-b', category: 'json-processing' },
  { artifact: 'http-x', category: 'http-client' },
];

const sbom = {
  bomFormat: 'CycloneDX',
  specVersion: '1.5',
  version: 1,
  metadata: { component: { type: 'application', 'bom-ref': 'app', name: projectName } },
  components: libraries.map(({ artifact }) => ({
    type: 'library',
    'bom-ref': artifact,
    name: artifact,
    purl: `${purlPrefix}${artifact}@1.0.0`,
  })),
  dependencies: [{ ref: 'app', dependsOn: libraries.map(({ artifact }) => artifact) }],
};

test.afterEach(async () => {
  // There is no API to delete a project or a library, so clean up in the E2E
  // database directly. Cascades remove project_libraries and library_categories.
  const db = new pg.Client({ connectionString: process.env.E2E_DATABASE_URL });
  await db.connect();
  try {
    await db.query('DELETE FROM projects WHERE name = $1', [projectName]);
    await db.query('DELETE FROM libraries WHERE purl LIKE $1', [`${purlPrefix}%`]);
    // Assert the outcome, not the delete: nothing of this run may be left.
    const left = await db.query(
      'SELECT (SELECT count(*) FROM projects WHERE name = $1)::int AS projects, (SELECT count(*) FROM libraries WHERE purl LIKE $2)::int AS libraries',
      [projectName, `${purlPrefix}%`],
    );
    expect(left.rows[0]).toEqual({ projects: 0, libraries: 0 });
  } finally {
    await db.end();
  }
});

test('uploaded SBOM libraries appear in the project report with their pipeline categories', async ({ page }) => {
  test.info().annotations.push({ type: 'test-data', description: token });

  // Open the report page as the signed-in architect; the upload form renders
  // only after the SPA has loaded, so its heading means the form is interactive.
  await page.goto('/redundancy');
  await expect(page.getByRole('heading', { name: 'Upload project SBOM' })).toBeVisible();

  // Upload the SBOM under a unique project name.
  await page.getByRole('textbox', { name: 'Project name' }).fill(projectName);
  await page.getByLabel('File').setInputFiles({
    name: `${token}.cdx.json`,
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify(sbom)),
  });
  await page.getByRole('button', { name: 'Upload' }).click();

  // Wait for the categorization batch to settle, then require a clean "done":
  // asserting the settled line's text makes an incomplete batch fail with its
  // real status ("done: 3 need review") instead of a bare timeout.
  const batchLine = page.getByText(/^Batch .+ — /);
  await expect(batchLine).toBeVisible();
  await expect(batchLine).not.toHaveText(/— processing…$/, { timeout: 30_000 });
  await expect(batchLine).toHaveText(/^Batch .+ — done$/);
  await expect(
    page.getByRole('combobox', { name: 'Project' }).getByRole('option', { name: projectName, selected: true }),
  ).toBeAttached();

  // Every uploaded library is listed exactly once, as direct, with its category.
  const reportRows = page.getByRole('row').filter({ hasText: purlPrefix });
  await expect(reportRows).toHaveCount(libraries.length);
  for (const { artifact, category } of libraries) {
    const row = reportRows.filter({ hasText: `${purlPrefix}${artifact}@1.0.0` });
    await expect(row.getByRole('cell', { name: 'Yes', exact: true })).toBeVisible();
    await expect(row.getByRole('cell', { name: category, exact: true })).toBeVisible();
  }
});
