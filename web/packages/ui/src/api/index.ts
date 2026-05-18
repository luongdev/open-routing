// Barrel re-export for the @open-routing/ui API module (D-40).
// Consumers write:
//   import { createApiClient, ErrorCodes, isApiError } from '@open-routing/ui';

export { createApiClient } from './client';
export type { ApiClient, CreateApiClientConfig } from './client';
export { isApiError, parseApiError, ErrorCodes, ERROR_I18N_KEYS } from './errors';
export type { ApiError, ErrorCode } from './errors';
export { createImporter, ImportError } from './import';
export type {
  BulkImportResult,
  CatalogEntity,
  ImportContentType,
  ImportCatalogArgs,
  ImportCatalogDeps,
} from './import';
export { createApiTask } from './task';
// Re-export the generated types so consumers can narrow paths/operations
// directly: `import type { paths } from '@open-routing/ui'`.
export type { paths, components, operations } from './generated';
