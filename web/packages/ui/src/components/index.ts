// Component barrel — Wave 1 establishes shell + primitives; Wave 2 adds Agents;
// Waves 3-4 add the remaining 5 entities. Each entity has its own subdir barrel
// that may export multiple components (list, detail, form, etc.).
export * from './shell/index.js';
export * from './primitives/index.js';
// Wave 2: Agents entity (Plan 06-05)
export * from './agents/index.js';
// Wave 3: Skills + Queues + BreakReasons + Adapters (Plans 06-07, 06-08, 06-09, 06-11)
export * from './skills/index.js';
export * from './queues/index.js';
export * from './break-reasons/index.js';
export * from './adapters/index.js';
// Wave 4: Channels (Plan 06-10)
export * from './channels/index.js';
