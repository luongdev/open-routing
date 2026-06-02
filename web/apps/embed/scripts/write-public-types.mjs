import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const distDir = join(packageRoot, 'dist');

mkdirSync(distDir, { recursive: true });

writeFileSync(join(distDir, 'index.d.ts'), `export type EmbedLocale = 'en' | 'vi';

export declare function applyEmbedLocale(raw: string): Promise<void>;

export declare class OpenRoutingCatalog extends HTMLElement {
  orgId: string;
  apiBaseUrl: string;
  theme: string;
  modules: string;
  locale: EmbedLocale;
}

declare global {
  interface HTMLElementTagNameMap {
    'open-routing-catalog': OpenRoutingCatalog;
  }
}
`);

writeFileSync(join(distDir, 'hash-router-adapter.d.ts'), `export interface HashRouterRoutes {
  goto(path: string): unknown;
}

export declare class HashRouterAdapter {
  start(routes: HashRouterRoutes): void;
  stop(): void;
  static buildHash(path: string): string;
}
`);
