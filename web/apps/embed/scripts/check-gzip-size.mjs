import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { gzipSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const distDir = join(packageRoot, 'dist');
const entry = join(distDir, 'embed.js');
const budgetBytes = 70 * 1024;

const formatKiB = (bytes) => `${(bytes / 1024).toFixed(2)} KiB`;

const gzipBytes = (filePath) => gzipSync(readFileSync(filePath), { level: 9 }).length;

if (!existsSync(entry)) {
  console.error('dist/embed.js is missing. Run pnpm --filter @open-routing/catalog-embed build first.');
  process.exit(1);
}

const entryGzip = gzipBytes(entry);
console.log(`dist/embed.js gzip: ${formatKiB(entryGzip)} / ${formatKiB(budgetBytes)}`);

const lazyChunks = readdirSync(distDir)
  .filter((name) => /^embed-.+\.js$/.test(name))
  .map((name) => ({ name, bytes: gzipBytes(join(distDir, name)) }))
  .sort((a, b) => b.bytes - a.bytes);

if (lazyChunks.length > 0) {
  console.log('lazy chunks gzip:');
  for (const chunk of lazyChunks) {
    console.log(`  ${chunk.name}: ${formatKiB(chunk.bytes)}`);
  }
}

if (entryGzip > budgetBytes) {
  console.error(`dist/embed.js exceeds gzip budget by ${formatKiB(entryGzip - budgetBytes)}.`);
  process.exit(1);
}
