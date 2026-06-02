import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const tscBin = join(packageRoot, '..', '..', 'node_modules', '.bin', 'tsc');
const scratchRoot = mkdtempSync(join(tmpdir(), 'open-routing-embed-pack-'));
const packDir = join(scratchRoot, 'pack');
const appDir = join(scratchRoot, 'app');

mkdirSync(packDir);
mkdirSync(appDir);

try {
  execFileSync('pnpm', ['pack', '--pack-destination', packDir], { cwd: packageRoot, stdio: 'inherit' });
  const tarball = readdirSync(packDir).find((name) => name.endsWith('.tgz'));
  if (!tarball) {
    throw new Error('pnpm pack did not create a tarball');
  }

  writeFileSync(join(appDir, 'package.json'), JSON.stringify({ type: 'module' }, null, 2));
  execFileSync('pnpm', ['add', join(packDir, tarball)], { cwd: appDir, stdio: 'inherit' });
  writeFileSync(join(appDir, 'check.mjs'), `
const embedUrl = await import.meta.resolve('@open-routing/catalog-embed');
const routerUrl = await import.meta.resolve('@open-routing/catalog-embed/hash-router-adapter');
if (!embedUrl.endsWith('/dist/embed.js')) throw new Error(embedUrl);
if (!routerUrl.endsWith('/dist/hash-router-adapter.js')) throw new Error(routerUrl);
`);
  writeFileSync(join(appDir, 'check.ts'), `
import type { EmbedLocale, OpenRoutingCatalog } from '@open-routing/catalog-embed';
import { HashRouterAdapter, type HashRouterRoutes } from '@open-routing/catalog-embed/hash-router-adapter';

const locale: EmbedLocale = 'vi';
const element: OpenRoutingCatalog = document.createElement('open-routing-catalog');
element.locale = locale;
const routes: HashRouterRoutes = { goto: (_path: string) => undefined };
new HashRouterAdapter().start(routes);
HashRouterAdapter.buildHash('/orgs/test/agents');
`);
  writeFileSync(join(appDir, 'tsconfig.json'), JSON.stringify({
    compilerOptions: {
      target: 'ES2022',
      module: 'ESNext',
      moduleResolution: 'Bundler',
      strict: true,
      lib: ['ES2022', 'DOM'],
      skipLibCheck: true,
      noEmit: true,
    },
    include: ['check.ts'],
  }, null, 2));
  execFileSync(process.execPath, ['check.mjs'], { cwd: appDir, stdio: 'inherit' });
  execFileSync(tscBin, ['--noEmit', '-p', join(appDir, 'tsconfig.json')], { cwd: appDir, stdio: 'inherit' });
  console.log('tarball smoke import resolution passed');
} finally {
  rmSync(scratchRoot, { recursive: true, force: true });
}
