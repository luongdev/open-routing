// ajv v8 programmatic validator codegen — D6-18 drift gate pipeline.
//
// Reads openapi/openapi.yaml, seeds Ajv with ALL #/components/schemas/* entries
// (so every $ref resolves), then compiles 13 targeted schemas into tree-shakeable
// ESM files under src/validators/.
//
// Run via: pnpm --filter @open-routing/ui gen:validators
// Or directly: node scripts/gen-validators.mjs (from web/packages/ui/ cwd)
//
// CRITICAL: addSchema for ALL schemas BEFORE compiling any target schema.
// Skipping this step causes MissingRefError on schemas that use $ref
// (e.g. CreateAgentRequest → AgentSkillAssignment, CreateChannelRequest → ChannelType).

import Ajv from 'ajv';
import standaloneModule from 'ajv/dist/standalone/index.js';
const standaloneCode = standaloneModule.standaloneCode ?? standaloneModule.default ?? standaloneModule;
import addFormats from 'ajv-formats';
import { readFileSync, writeFileSync, mkdirSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import { load } from 'js-yaml';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Resolve openapi.yaml relative to this script:
// scripts/ → packages/ui → web → repo root → openapi/openapi.yaml
// Path: scripts/../../../.. (4 levels up from scripts/)
const specPath = join(__dirname, '..', '..', '..', '..', 'openapi', 'openapi.yaml');
const spec = load(readFileSync(specPath, 'utf8'));

const schemas = spec.components?.schemas ?? {};

/**
 * Convert OpenAPI 3.0 `nullable: true` to JSON Schema `type: ["X", "null"]`.
 *
 * OpenAPI 3.0 uses `nullable: true` alongside a `type` keyword, but Ajv
 * follows JSON Schema (draft-07) which uses `type: ["X", "null"]`.
 * Without this conversion, nullable fields (e.g. external_id, default_queue_id)
 * will fail validation when the value is null — breaking clear/reset semantics.
 *
 * Also handles `allOf: [{ $ref }, { ... nullable: true ... }]` patterns.
 * Deep traversal applies to all nested properties and allOf/anyOf/oneOf items.
 */
function normalizeNullable(schema) {
  if (!schema || typeof schema !== 'object') return schema;
  if (Array.isArray(schema)) return schema.map(normalizeNullable);

  const result = { ...schema };

  // Convert nullable: true + type: "X" → type: ["X", "null"]
  if (result.nullable === true) {
    delete result.nullable;
    if (typeof result.type === 'string') {
      result.type = [result.type, 'null'];
    } else if (Array.isArray(result.type) && !result.type.includes('null')) {
      result.type = [...result.type, 'null'];
    } else if (result.type === undefined && !result.allOf && !result.$ref) {
      // nullable on a schema without a type — add null to anyOf
      result.anyOf = [result.anyOf ?? {}, { type: 'null' }];
    } else if (result.allOf) {
      // allOf with nullable: wrap in anyOf to allow null
      result.anyOf = [{ allOf: result.allOf }, { type: 'null' }];
      delete result.allOf;
    }
  }

  // Recurse into nested schema keywords
  if (result.properties) {
    result.properties = Object.fromEntries(
      Object.entries(result.properties).map(([k, v]) => [k, normalizeNullable(v)])
    );
  }
  if (result.items) result.items = normalizeNullable(result.items);
  if (result.allOf) result.allOf = result.allOf.map(normalizeNullable);
  if (result.anyOf) result.anyOf = result.anyOf.map(normalizeNullable);
  if (result.oneOf) result.oneOf = result.oneOf.map(normalizeNullable);
  if (result.not) result.not = normalizeNullable(result.not);
  if (result.additionalProperties && typeof result.additionalProperties === 'object') {
    result.additionalProperties = normalizeNullable(result.additionalProperties);
  }

  return result;
}

// Create Ajv instance with ESM standalone code output
const ajv = new Ajv({
  code: { source: true, esm: true },
  strict: false,      // OpenAPI 3.0 uses nullable: true which JSON Schema strict rejects
  allErrors: true,    // collect all errors, not just the first
});
addFormats(ajv);

// Step 1 — SEED: register ALL schemas so every $ref resolves.
// Must happen BEFORE compiling any target schema.
// normalizeNullable() pre-processes each schema to convert OpenAPI 3.0
// `nullable: true` → JSON Schema `type: [X, "null"]` so null values pass
// validation for nullable fields like external_id, default_queue_id, etc.
for (const [name, schema] of Object.entries(schemas)) {
  const normalized = normalizeNullable({ ...schema, $id: `#/components/schemas/${name}` });
  try {
    ajv.addSchema(normalized, `#/components/schemas/${name}`);
  } catch (e) {
    // If already added (shouldn't happen), skip gracefully
    if (!String(e).includes('already exists')) {
      console.warn(`WARN: addSchema(${name}) failed:`, e.message);
    }
  }
}

// Step 2 — COMPILE: generate ESM standalone code for the 13 target schemas
const targetSchemas = [
  'CreateAgentRequest', 'UpdateAgentRequest',
  'CreateSkillRequest', 'UpdateSkillRequest',
  'CreateQueueRequest', 'UpdateQueueRequest',
  'CreateChannelRequest', 'UpdateChannelRequest',
  'CreateAdapterRequest', 'UpdateAdapterRequest',
  'CreateBreakReasonRequest', 'UpdateBreakReasonRequest',
  'PatchAgentStatusRequest',
];

const outDir = join(__dirname, '..', 'src', 'validators');
mkdirSync(outDir, { recursive: true });

const generated = [];

for (const name of targetSchemas) {
  const schemaId = `#/components/schemas/${name}`;
  const validate = ajv.getSchema(schemaId);
  if (!validate) {
    console.warn(`WARN: Schema "${name}" not found in spec — skipping`);
    continue;
  }
  const rawCode = standaloneCode(ajv, validate);
  // Prepend ts-nocheck: ajv standalone generates JS-style code that does not
  // conform to strict TypeScript types (any parameters, union params, etc.).
  // The generated validators are correct at runtime; tsc type-checking them
  // would require a custom ajv-plugin which is out of scope for D6-18.
  const code = `// @ts-nocheck — generated by scripts/gen-validators.mjs; do not edit manually.\n${rawCode}`;
  const outPath = join(outDir, `${name}.ts`);
  writeFileSync(outPath, code, 'utf8');
  console.log(`Generated: src/validators/${name}.ts`);
  generated.push(name);
}

// Step 3 — Write barrel index that re-exports all generated validators
const barrelLines = generated.map(
  (name) => `export { default as ${name} } from './${name}.js';`
);
const indexPath = join(outDir, 'index.ts');
writeFileSync(
  indexPath,
  [
    '// Auto-generated by scripts/gen-validators.mjs — do not edit manually.',
    '// Re-run with: pnpm --filter @open-routing/ui gen:validators',
    '',
    ...barrelLines,
    '',
  ].join('\n'),
  'utf8'
);
console.log(`Generated: src/validators/index.ts (${generated.length} validators)`);
