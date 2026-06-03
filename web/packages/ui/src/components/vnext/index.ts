// vNext preview barrel — design-only components for the workflow milestone (v0.2+).
// Consumed by playground-screens.ts.
//
// Simulator merged into flow-builder (Simulate mode). Standalone
// flow-simulator.ts removed — kept the deletion clean to avoid two
// runner UIs claiming ownership of the same workflow.
//
// flow-list graduated to the real API-backed component (components/flows/);
// the playground previews it via the mock client.
export * from './flow-builder.js';
export * from './trace-viewer.js';
