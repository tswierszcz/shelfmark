// Local stand-in for the three external APIs the backend calls server-side
// while categorizing a library: deps.dev (repo lookup), GitHub (topics) and
// OpenRouter (category). page.route() can't see server-side calls, so the E2E
// build points DEPSDEV_BASE_URL, GITHUB_API_BASE_URL and OPENROUTER_BASE_URL
// here (see web/playwright.config.ts).
//
// It only knows E2E libraries: group "com.e2e-<token>", artifact names that
// start with the category they should get ("json-…" → json-processing,
// "http-…" → http-client). Anything else gets 503, so a wrong wiring fails
// loudly instead of returning a guess.
import { createServer } from 'node:http';

const port = Number(process.env.EXTERNAL_APIS_MOCK_PORT ?? 3199);

const categoryFor = (artifact) =>
  artifact.startsWith('json-') ? 'json-processing' : artifact.startsWith('http-') ? 'http-client' : null;

const send = (res, status, body) => {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
};

const readBody = (req) =>
  new Promise((resolve, reject) => {
    let data = '';
    req.on('data', (chunk) => (data += chunk));
    req.on('end', () => resolve(data));
    req.on('error', reject);
  });

createServer(async (req, res) => {
  const path = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);

  // Readiness check for Playwright's webServer.
  if (req.method === 'GET' && path === '/') return send(res, 200, { ok: true });

  // deps.dev: /v3/systems/maven/packages/<group>:<artifact>/versions/<version>
  const depsDev = path.match(/^\/v3\/systems\/maven\/packages\/(com\.e2e-[\w-]+):([\w-]+)\/versions\//);
  if (req.method === 'GET' && depsDev) {
    const [, group, artifact] = depsDev;
    return send(res, 200, { links: { repo: `https://github.com/${group.replace('com.', '')}/${artifact}` } });
  }

  // GitHub: /repos/e2e-<token>/<artifact>
  const github = path.match(/^\/repos\/(e2e-[\w-]+)\/([\w-]+)$/);
  if (req.method === 'GET' && github) {
    return send(res, 200, { topics: [github[2].split('-')[0]] });
  }

  // OpenRouter: /api/v1/chat/completions — only the Categorize prompt, which
  // names the library's purl.
  if (req.method === 'POST' && path === '/api/v1/chat/completions') {
    const { messages } = JSON.parse(await readBody(req));
    const prompt = messages?.[0]?.content ?? '';
    const purl = prompt.match(/pkg:maven\/com\.e2e-[\w-]+\/([\w-]+)@/);
    const category = purl && prompt.includes('taxonomy') ? categoryFor(purl[1]) : null;
    if (category) {
      return send(res, 200, { choices: [{ message: { role: 'assistant', content: JSON.stringify([category]) } }] });
    }
  }

  send(res, 503, { error: `external-apis mock: no stub for ${req.method} ${path}` });
}).listen(port, () => console.log(`external-apis mock listening on ${port}`));
