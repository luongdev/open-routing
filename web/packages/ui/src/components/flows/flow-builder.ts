// v0.2 Layer 2 — API-backed flow builder (FLOW authoring). Graduated from the
// vNext mock onto GET/PATCH/POST /v1/orgs/{org_id}/flows{,/{id},/validate,
// /publish}. The graph (nodes + edges) loads from and saves to the flow's
// opaque `graph` JSONB; Save draft is a real optimistic-version PATCH (POST on
// create).
//
// Validate / Publish / Rollback and live Simulate are 501 stubs until Layer 3,
// so those actions surface "lands in Layer 3" instead of faking success. The
// rich sim panel stays a sample-trace PREVIEW (mock playback) — kept visible on
// purpose to surface layout/wiring bugs early (review choice "b").

import { LitElement, html, css, svg, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import type { ApiClient } from '../../api/client.js';
import { autoArrange } from './flow-layout.js';
import {
  groupToDsl, dslToGroup, newComparison, newGroup, COND_OPS,
  type Group, type CondNode, type Comparison,
} from './flow-condition.js';
import type { components } from '../../api/generated.js';

type ExprFunction = components['schemas']['ExprFunction'];
import {
  MOCK_TRACE_STEPS,
  MOCK_TRACE_STEPS_FAIL,
  MOCK_INIT_VARS,
  MOCK_TEST_CASES,
  WAIT_INPUT_KINDS,
  computeVarBag,
  type FlowNode,
  type FlowEdge,
  type FlowNodeKind,
  type FlowNodeOutput,
  type TraceStep,
  type StepStatus,
  type InitVar,
  type TestCase,
} from '../shell/playground-mock-data.js';

type Flow = components['schemas']['Flow'];

// Validate/publish responses (Layer 3 backend). Kept as local shapes rather
// than the generated component aliases so the file has one source for the
// fields the UI actually reads.
interface FlowValidationIssue {
  code: string;
  message: string;
  node_id?: string;
  edge_id?: string;
  field?: string;
}
interface FlowValidationResult {
  valid: boolean;
  issues: FlowValidationIssue[];
}

interface PaletteEntry {
  kind: FlowNodeKind;
  label: string;
  icon: string;
  tone: 'neutral' | 'primary' | 'warning' | 'destructive' | 'success';
  desc: string;
}

interface PaletteGroup {
  label: string;
  items: PaletteEntry[];
}

// Grouped node taxonomy. Categories match the runtime engine families:
// triggers, control flow, data ops, integration, routing (domain), side
// effects, end. Two of each category are usually plenty; we list the
// realistic primitives the user can drop on canvas.
const PALETTE: PaletteGroup[] = [
  {
    label: 'Triggers',
    items: [
      { kind: 'trigger', label: 'Trigger', icon: 'play', tone: 'primary', desc: 'Channel inbound / webhook / cron' },
    ],
  },
  {
    label: 'Control flow',
    items: [
      { kind: 'if_else',     label: 'If / Else',     icon: 'git-fork',          tone: 'neutral',     desc: 'Binary branch on expr' },
      { kind: 'switch_case', label: 'Switch / Case', icon: 'split',             tone: 'neutral',     desc: 'N-way branch on value' },
      { kind: 'loop_for',    label: 'For each',      icon: 'repeat',            tone: 'neutral',     desc: 'Iterate over array' },
      { kind: 'loop_while',  label: 'While',         icon: 'repeat-2',          tone: 'neutral',     desc: 'Repeat until condition' },
      { kind: 'parallel',    label: 'Parallel',      icon: 'rows-3',            tone: 'neutral',     desc: 'Fan-out / join' },
      { kind: 'try_catch',   label: 'Try / Catch',   icon: 'shield',            tone: 'warning',     desc: 'Error boundary' },
      { kind: 'wait',        label: 'Wait',          icon: 'clock',             tone: 'neutral',     desc: 'Delay (ms / cron)' },
    ],
  },
  {
    label: 'User input',
    items: [
      { kind: 'get_dtmf',        label: 'Get DTMF',         icon: 'phone-call',    tone: 'primary', desc: 'Capture IVR digits — sim pauses' },
      { kind: 'prompt_text',     label: 'Prompt text',      icon: 'message-square',tone: 'primary', desc: 'Capture free-text — sim pauses' },
      { kind: 'wait_signal',     label: 'Wait for signal',  icon: 'satellite',     tone: 'primary', desc: 'Pause until external event' },
      { kind: 'manual_approval', label: 'Manual approval',  icon: 'gavel',         tone: 'warning', desc: 'Human gate — sim pauses' },
    ],
  },
  {
    label: 'Data',
    items: [
      { kind: 'set_var',     label: 'Set variable',  icon: 'pencil-line',       tone: 'primary',     desc: 'Assign expression to a var' },
      { kind: 'compute',     label: 'Compute',       icon: 'sigma',             tone: 'primary',     desc: 'Evaluate an expression' },
      { kind: 'script',      label: 'Script',        icon: 'code',              tone: 'warning',     desc: 'Inline JS / Lua / Python — DSL escape hatch' },
    ],
  },
  {
    label: 'Integration',
    items: [
      { kind: 'http_request', label: 'HTTP request', icon: 'globe',             tone: 'primary',     desc: 'GET/POST sync or async' },
      { kind: 'webhook',      label: 'Webhook',      icon: 'send',              tone: 'primary',     desc: 'Fire-and-forget outbound' },
    ],
  },
  {
    label: 'Routing',
    items: [
      { kind: 'match_skill', label: 'Match skill',   icon: 'target',            tone: 'neutral',     desc: 'Filter candidate agents' },
      { kind: 'filter',      label: 'Filter pool',   icon: 'filter',            tone: 'neutral',     desc: 'Narrow the candidate pool' },
      { kind: 'route_queue', label: 'Route → queue', icon: 'list',              tone: 'success',     desc: 'Enqueue with priority' },
      { kind: 'reservation', label: 'Reservation',   icon: 'user-check',        tone: 'primary',     desc: 'Offer + accept loop' },
      { kind: 'fallback',    label: 'Fallback',      icon: 'shield-alert',      tone: 'destructive', desc: 'On timeout / no agents' },
      { kind: 'set_agent_state', label: 'Set agent state', icon: 'user-cog',    tone: 'neutral',     desc: 'Change agent presence (Available / Break / Wrap)' },
      { kind: 'wrapup_timer',    label: 'Wrap-up timer',  icon: 'timer',         tone: 'neutral',     desc: 'Server-owned ACW countdown' },
    ],
  },
  {
    label: 'Voice channel',
    items: [
      { kind: 'tts_speak',     label: 'TTS speak',      icon: 'volume-2',       tone: 'primary',     desc: 'Synthesize + play to caller' },
      { kind: 'play_prompt',   label: 'Play prompt',    icon: 'megaphone',      tone: 'primary',     desc: 'Play a pre-recorded audio file' },
      { kind: 'detect_speech', label: 'Detect speech',  icon: 'mic',            tone: 'primary',     desc: 'ASR — recognize spoken intent' },
      { kind: 'transfer_call', label: 'Transfer call',  icon: 'phone-forwarded',tone: 'primary',     desc: 'Bridge to external number' },
      { kind: 'hangup',        label: 'Hangup',         icon: 'phone-off',      tone: 'destructive', desc: 'Drop the call' },
    ],
  },
  {
    label: 'Chat channel',
    items: [
      { kind: 'send_message',     label: 'Send message',     icon: 'message-circle', tone: 'primary', desc: 'Outbound chat line' },
      { kind: 'quick_replies',    label: 'Quick replies',    icon: 'mouse-pointer-2',tone: 'primary', desc: 'Buttons / suggested chips' },
      { kind: 'typing_indicator', label: 'Typing indicator', icon: 'more-horizontal',tone: 'neutral', desc: 'Show "agent is typing"' },
      { kind: 'attach_file',      label: 'Attach file',      icon: 'paperclip',      tone: 'neutral', desc: 'Send a file to the customer' },
      { kind: 'bot_handoff',      label: 'Bot handoff',      icon: 'bot',            tone: 'warning', desc: 'Escalate bot → human agent' },
    ],
  },
  {
    label: 'Email channel',
    items: [
      { kind: 'send_template',    label: 'Send template',    icon: 'mail',          tone: 'primary',  desc: 'Render + send templated email' },
    ],
  },
  {
    label: 'Surveys (post-interaction)',
    items: [
      { kind: 'csat_survey',      label: 'CSAT survey',      icon: 'star',          tone: 'warning',  desc: '1-5 satisfaction rating' },
      { kind: 'nps_survey',       label: 'NPS survey',       icon: 'gauge',         tone: 'warning',  desc: '0-10 Net Promoter Score' },
    ],
  },
  {
    label: 'Side effects',
    items: [
      { kind: 'effect',      label: 'Effect',        icon: 'zap',               tone: 'warning',     desc: 'Webhook / external API' },
      { kind: 'log',         label: 'Log',           icon: 'file-text',         tone: 'neutral',     desc: 'Structured audit log' },
    ],
  },
  {
    label: 'End',
    items: [
      { kind: 'end',         label: 'End',           icon: 'circle-check',      tone: 'neutral',     desc: 'Flow completes' },
    ],
  },
];

// Flat lookup for tone/icon by kind. Built once.
const KIND_PROPS: Record<FlowNodeKind, { tone: PaletteEntry['tone']; icon: string }> = (() => {
  const map = {} as Record<FlowNodeKind, { tone: PaletteEntry['tone']; icon: string }>;
  for (const group of PALETTE) {
    for (const p of group.items) map[p.kind] = { tone: p.tone, icon: p.icon };
  }
  return map;
})();

// ── Runtime contract registries (Phase 0) ──────────────────────────────────
// The node kinds the v0.2 backend runtime supports. Only these are draggable;
// authoring any other kind would validate as `unknown_node_kind`.
const RUNTIME_KINDS = new Set<FlowNodeKind>([
  'trigger', 'if_else', 'switch_case', 'wait', 'match_skill', 'filter',
  'route_queue', 'reservation', 'fallback', 'effect', 'log', 'end',
  'set_var', 'compute',
  // 3D-2 control flow (region-owning): a `body`/`body:N` port declares the
  // body region entry; `done`/`catch` continue the top-level flow.
  'loop_for', 'loop_while', 'parallel', 'try_catch',
  // 3D-3 record/mock side effects (channel / integration / agent-state). v0.2
  // records the intent and continues down `done` — no live external call.
  'tts_speak', 'play_prompt', 'transfer_call', 'hangup',
  'send_message', 'quick_replies', 'typing_indicator', 'attach_file', 'bot_handoff',
  'send_template',
  'http_request', 'webhook',
  'set_agent_state', 'wrapup_timer',
  // 3D-4 interactive input (suspend-for-value). Branch on the captured value;
  // sim resolves from a scripted value, live times out.
  'get_dtmf', 'prompt_text', 'wait_signal', 'manual_approval', 'detect_speech',
  'csat_survey', 'nps_survey',
  // 3D-5 sandboxed Lua script over the var bag.
  'script',
]);

// Control-flow kinds own a body region (their `body`/`body:N` port enters a
// sub-graph whose member nodes carry node.region).
const CONTROL_KINDS = new Set<FlowNodeKind>(['loop_for', 'loop_while', 'parallel', 'try_catch']);

const DONE_OUT: FlowNodeOutput[] = [{ id: 'done', label: 'done', kind: 'success' }];

// Output ports per kind. `FlowNodeOutput.id` IS the runtime port the backend
// compiles from the edge label (compile.go) — if_else→true/false,
// switch_case→case value/default, end→none (terminal), everything else→done.
const KIND_OUTPUTS: Partial<Record<FlowNodeKind, FlowNodeOutput[]>> = {
  trigger: DONE_OUT,
  if_else: [{ id: 'true', label: 'true', kind: 'branch' }, { id: 'false', label: 'false', kind: 'default' }],
  wait: DONE_OUT,
  match_skill: DONE_OUT,
  filter: DONE_OUT,
  route_queue: DONE_OUT,
  // Reservation outcome ports (runtime emits one of these): wire `accepted` to
  // the success path and timeout/no_candidate to a fallback.
  reservation: [
    { id: 'accepted', label: 'accepted', kind: 'success' },
    { id: 'timeout', label: 'timeout', kind: 'timeout' },
    { id: 'no_candidate', label: 'no_candidate', kind: 'error' },
  ],
  fallback: DONE_OUT,
  effect: DONE_OUT,
  log: DONE_OUT,
  end: [],
  set_var: DONE_OUT,
  compute: DONE_OUT,
  // Control flow: `body`/`catch` ports are region/continuation declarations. The
  // display labels disambiguate the continuation from a plain node's "done" — the
  // backend port id (body/done/catch) is unchanged (FlowNodeOutput.id).
  loop_for: [{ id: 'body', label: 'loop body', kind: 'branch' }, { id: 'done', label: 'after loop', kind: 'success' }],
  loop_while: [{ id: 'body', label: 'loop body', kind: 'branch' }, { id: 'done', label: 'after loop', kind: 'success' }],
  try_catch: [
    { id: 'body', label: 'try', kind: 'branch' },
    { id: 'catch', label: 'on error', kind: 'error' },
    { id: 'done', label: 'after', kind: 'success' },
  ],
  // parallel is dynamic (body:0..body:N) — see outputsForNode.
  // 3D-4 interactive input: branch on the captured value (or timeout). Port ids
  // match node_input.go inputSpecs.
  get_dtmf: [{ id: 'captured', label: 'captured', kind: 'success' }, { id: 'timeout', label: 'timeout', kind: 'timeout' }],
  prompt_text: [{ id: 'captured', label: 'captured', kind: 'success' }, { id: 'timeout', label: 'timeout', kind: 'timeout' }],
  wait_signal: [{ id: 'received', label: 'received', kind: 'success' }, { id: 'timeout', label: 'timeout', kind: 'timeout' }],
  manual_approval: [
    { id: 'approved', label: 'approved', kind: 'success' },
    { id: 'rejected', label: 'rejected', kind: 'error' },
    { id: 'timeout', label: 'timeout', kind: 'timeout' },
  ],
  detect_speech: [
    { id: 'recognized', label: 'recognized', kind: 'success' },
    { id: 'no_match', label: 'no match', kind: 'default' },
    { id: 'timeout', label: 'timeout', kind: 'timeout' },
  ],
  csat_survey: [{ id: 'done', label: 'done', kind: 'success' }, { id: 'timeout', label: 'timeout', kind: 'timeout' }],
  nps_survey: [{ id: 'done', label: 'done', kind: 'success' }, { id: 'timeout', label: 'timeout', kind: 'timeout' }],
};

// parallel branch count (body:0..body:N-1). Bounded so the port row stays legible.
const PARALLEL_MIN_BRANCHES = 2;
const PARALLEL_MAX_BRANCHES = 6;
function parallelBranchCount(params?: FlowNode['params']): number {
  const n = Number(params?.['branches'] ?? PARALLEL_MIN_BRANCHES);
  if (!Number.isFinite(n)) return PARALLEL_MIN_BRANCHES;
  return Math.min(PARALLEL_MAX_BRANCHES, Math.max(PARALLEL_MIN_BRANCHES, Math.floor(n)));
}

// switch_case ports are dynamic (one per case + default); everything else is
// static from KIND_OUTPUTS. Used to (re)seed node.outputs.
function outputsForNode(node: Pick<FlowNode, 'kind' | 'params'>): FlowNodeOutput[] {
  if (node.kind === 'parallel') {
    const n = parallelBranchCount(node.params);
    return [
      ...Array.from({ length: n }, (_, i) => ({ id: `body:${i}`, label: `body:${i}`, kind: 'branch' as const })),
      { id: 'done', label: 'done', kind: 'success' as const },
    ];
  }
  if (node.kind === 'switch_case') {
    const raw = Array.isArray(node.params?.cases) ? (node.params!.cases as unknown[]) : [];
    // Drop a user case literally named "default" — it would collide with the
    // built-in default port (agy MED-5).
    const cases = [...new Set(raw.map(c => String(c).trim()).filter(c => c && c !== 'default'))];
    return [
      ...cases.map(c => ({ id: c, label: c, kind: 'branch' as const })),
      { id: 'default', label: 'default', kind: 'default' as const },
    ];
  }
  return KIND_OUTPUTS[node.kind] ?? DONE_OUT;
}

interface FieldDef {
  key: string;
  label: string;
  type: 'text' | 'number' | 'select' | 'cases' | 'condition' | 'catalog' | 'expr' | 'textarea';
  options?: string[];
  // For type 'catalog': which catalog list feeds the searchable picker.
  source?: 'skill' | 'queue' | 'adapter';
  // Optional helper line under the field (e.g. ${var} interpolation note).
  hint?: string;
  placeholder?: string;
}

interface CatalogRef {
  code: string;
  name: string;
}

// Comparison operators for the basic condition builder — longest first so
// `<=`/`>=`/`==`/`!=` match before `<`/`>`. Mirrors the backend expr grammar.
const EXPR_OPS = ['==', '!=', '<=', '>=', '<', '>'] as const;

// Editable config fields per kind. Keys match the backend config structs
// (internal/runtime/node_kinds.go) — authoritative.
const KIND_FIELDS: Partial<Record<FlowNodeKind, FieldDef[]>> = {
  trigger: [{ key: 'channel', label: 'Channel', type: 'text' }, { key: 'entry_code', label: 'Entry code', type: 'text' }],
  if_else: [{ key: 'expr', label: 'Condition', type: 'condition' }],
  switch_case: [{ key: 'expr', label: 'Value expression', type: 'text' }, { key: 'cases', label: 'Cases', type: 'cases' }],
  wait: [{ key: 'duration_ms', label: 'Duration (ms)', type: 'number' }],
  match_skill: [{ key: 'skill', label: 'Skill', type: 'catalog', source: 'skill' }, { key: 'min_proficiency', label: 'Min proficiency', type: 'number' }],
  filter: [{ key: 'expr', label: 'Predicate', type: 'condition' }],
  route_queue: [{ key: 'queue', label: 'Queue', type: 'catalog', source: 'queue' }],
  reservation: [{ key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' }, { key: 'max_attempts', label: 'Max attempts', type: 'number' }],
  fallback: [{ key: 'reason', label: 'Reason', type: 'text' }],
  effect: [{ key: 'adapter', label: 'Adapter', type: 'catalog', source: 'adapter' }, { key: 'action', label: 'Action', type: 'text' }],
  log: [{ key: 'message', label: 'Message', type: 'text' }, { key: 'level', label: 'Level', type: 'select', options: ['debug', 'info', 'warn', 'error'] }],
  end: [{ key: 'outcome', label: 'Outcome', type: 'text' }],
  set_var: [{ key: 'name', label: 'Variable name', type: 'text' }, { key: 'value_expr', label: 'Value (expression)', type: 'expr' }],
  compute: [{ key: 'expr', label: 'Expression', type: 'expr' }, { key: 'var', label: 'Store in variable (optional)', type: 'text' }],
  loop_for: [
    { key: 'array_expr', label: 'Array (expression)', type: 'expr' },
    { key: 'item_var', label: 'Item variable', type: 'text' },
    { key: 'index_var', label: 'Index variable', type: 'text' },
    { key: 'max_iter', label: 'Max iterations', type: 'number' },
  ],
  loop_while: [
    { key: 'cond_expr', label: 'While condition', type: 'condition' },
    { key: 'max_iter', label: 'Max iterations', type: 'number' },
  ],
  parallel: [{ key: 'branches', label: 'Branches', type: 'number' }],
  try_catch: [{ key: 'error_var', label: 'Error variable', type: 'text' }],
  // 3D-3 record/mock side effects. Field keys match the backend spec
  // (node_side_effects.go sideEffectSpecs) — required fields are enforced there.
  // String fields interpolate ${expr} against the flow vars at run time.
  tts_speak: [
    { key: 'text', label: 'Text to speak', type: 'textarea', hint: 'Supports ${var} — e.g. Hello ${customer.name}', placeholder: 'Hello ${customer.name}, welcome back.' },
    { key: 'voice', label: 'Voice (optional)', type: 'text' },
  ],
  play_prompt: [{ key: 'prompt', label: 'Prompt id / file', type: 'text', hint: 'Supports ${var}' }],
  transfer_call: [{ key: 'destination', label: 'Destination (number / SIP)', type: 'text', hint: 'Supports ${var}' }],
  hangup: [{ key: 'reason', label: 'Reason (optional)', type: 'text' }],
  send_message: [{ key: 'text', label: 'Message', type: 'textarea', hint: 'Supports ${var} — e.g. Order ${order.id} is ready', placeholder: 'Hi ${customer.name} 👋' }],
  quick_replies: [
    { key: 'text', label: 'Prompt', type: 'textarea', hint: 'Supports ${var}' },
    { key: 'options', label: 'Options (comma-separated)', type: 'text', placeholder: 'Yes, No, Maybe' },
  ],
  typing_indicator: [{ key: 'seconds', label: 'Duration (sec, optional)', type: 'number' }],
  attach_file: [
    { key: 'url', label: 'File URL', type: 'text', hint: 'Supports ${var}' },
    { key: 'filename', label: 'Filename (optional)', type: 'text' },
  ],
  bot_handoff: [{ key: 'reason', label: 'Handoff reason (optional)', type: 'text' }],
  send_template: [
    { key: 'template', label: 'Template id', type: 'text' },
    { key: 'to', label: 'To (optional)', type: 'text', hint: 'Supports ${var} — e.g. ${customer.email}' },
  ],
  http_request: [
    { key: 'method', label: 'Method', type: 'select', options: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] },
    { key: 'url', label: 'URL', type: 'text', hint: 'Supports ${var} — e.g. https://api/users/${customer.id}', placeholder: 'https://api.example.com/orders/${order.id}' },
    { key: 'headers', label: 'Headers (one per line: Key: Value)', type: 'textarea', placeholder: 'Authorization: Bearer ${token}\nContent-Type: application/json' },
    { key: 'body', label: 'Body', type: 'textarea', hint: 'Supports ${var} — interpolated into the (recorded) request body', placeholder: '{\n  "customer_id": "${customer.id}",\n  "tier": "${customer.tier}"\n}' },
    { key: 'save_as', label: 'Save response to variable', type: 'text', hint: 'Then read it downstream: ${resp.user.name} (nested), ${resp.items.0.id} (array), arr.len(resp.items) (length)', placeholder: 'resp' },
    { key: 'mock_response', label: 'Mock response (JSON, v0.2)', type: 'textarea', hint: 'v0.2 makes no live call — this JSON is recorded as the response and stored in the variable above.', placeholder: '{\n  "items": [\n    { "id": "a1", "name": "Alice" },\n    { "id": "b2", "name": "Bob" }\n  ]\n}' },
  ],
  webhook: [
    { key: 'url', label: 'URL', type: 'text', hint: 'Supports ${var}' },
    { key: 'body', label: 'Body', type: 'textarea', hint: 'Supports ${var}', placeholder: '{\n  "event": "routed",\n  "id": "${interaction.id}"\n}' },
  ],
  set_agent_state: [{ key: 'state', label: 'State', type: 'select', options: ['Ready', 'NotReady', 'Break', 'WrapUp', 'Offline'] }],
  wrapup_timer: [{ key: 'duration_sec', label: 'Duration (sec)', type: 'number' }],
  // 3D-4 interactive input. The captured value is stored in save_as (defaults
  // shown); in the simulator set a value per node to take the captured branch,
  // else it times out. prompt supports ${var}.
  get_dtmf: [
    { key: 'prompt', label: 'Prompt', type: 'textarea', hint: 'Supports ${var}', placeholder: 'Press 1 for sales, 2 for support' },
    { key: 'save_as', label: 'Save digits to variable', type: 'text', placeholder: 'digits' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  prompt_text: [
    { key: 'prompt', label: 'Prompt', type: 'textarea', hint: 'Supports ${var}' },
    { key: 'save_as', label: 'Save text to variable', type: 'text', placeholder: 'text' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  wait_signal: [
    { key: 'save_as', label: 'Save signal payload to variable', type: 'text', placeholder: 'signal' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  manual_approval: [
    { key: 'prompt', label: 'Approval question', type: 'textarea', hint: 'Supports ${var}', placeholder: 'Approve refund for ${customer.name}?' },
    { key: 'save_as', label: 'Save decision to variable', type: 'text', placeholder: 'decision' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  detect_speech: [
    { key: 'prompt', label: 'Prompt', type: 'textarea', hint: 'Supports ${var}' },
    { key: 'save_as', label: 'Save transcript to variable', type: 'text', placeholder: 'speech' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  csat_survey: [
    { key: 'prompt', label: 'Question', type: 'textarea', hint: 'Supports ${var}', placeholder: 'How satisfied were you? (1-5)' },
    { key: 'save_as', label: 'Save rating to variable', type: 'text', placeholder: 'csat' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  nps_survey: [
    { key: 'prompt', label: 'Question', type: 'textarea', hint: 'Supports ${var}', placeholder: 'How likely are you to recommend us? (0-10)' },
    { key: 'save_as', label: 'Save score to variable', type: 'text', placeholder: 'nps' },
    { key: 'timeout_sec', label: 'Timeout (sec)', type: 'number' },
  ],
  // 3D-5 sandboxed Lua. Read the bag via `vars`, return a value → save_as.
  script: [
    { key: 'code', label: 'Lua script', type: 'textarea', hint: 'Sandboxed Lua: read `vars`, `return` a value. No IO/clock. e.g. return vars.a + vars.b', placeholder: 'local t = vars.customer.tier\nif t == "gold" then return 2 else return 1 end' },
    { key: 'save_as', label: 'Save result to variable', type: 'text', placeholder: 'result' },
  ],
};

// Fraction (0..1) of card WIDTH where output port `idx` sits along the
// bottom edge. Matches CSS `justify-content: space-around` on
// .node-card-ports — equal half-gaps at left/right, even spacing between.
// Consumed by portX() (edge anchoring) — single source of truth. MUST stay in
// sync with the .node-card width/height in `static styles`.
const NODE_W_PX = 220;
const NODE_H_PX = 132;
const _portFracsCache = new Map<number, ReadonlyArray<number>>();
function portFracs(count: number): ReadonlyArray<number> {
  const cached = _portFracsCache.get(count);
  if (cached) return cached;
  const result: ReadonlyArray<number> = count <= 1
    ? [0.5]
    : Array.from({ length: count }, (_, i) => (i + 0.5) / count);
  _portFracsCache.set(count, result);
  return result;
}

@customElement('or-flow-builder')
export class OrFlowBuilder extends LitElement {
  static override styles = css`
    :host {
      display: flex;
      flex-direction: column;
      min-height: 680px;
      height: calc(100vh - 160px);
      background: var(--background);
      position: relative;
    }

    /* ============ Toolbar ============ */
    .toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px 20px;
      background: var(--card);
      border-bottom: 1px solid var(--border);
    }
    .toolbar-back {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      border-radius: 8px;
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background .12s, color .12s;
    }
    .toolbar-back:hover { background: var(--muted); color: var(--foreground); }

    .toolbar-title {
      display: flex;
      flex-direction: column;
      gap: 1px;
    }
    .toolbar-title strong {
      font-size: 15px;
      font-weight: 700;
      color: var(--foreground);
    }
    .toolbar-title .meta {
      font-size: 11px;
      color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
    }

    .toolbar .status-pill {
      display: inline-flex; align-items: center; gap: 4px;
      padding: 3px 10px; border-radius: 9999px;
      font-size: 11px; font-weight: 500; text-transform: capitalize;
      background: color-mix(in oklch, var(--warning) 22%, transparent);
      color: var(--warning);
    }
    .toolbar .status-pill::before {
      content: '';
      width: 6px; height: 6px; border-radius: 50%; background: currentColor;
    }

    .toolbar-spacer { flex: 1; }

    .toolbar-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 500;
      background: var(--card);
      border: 1px solid var(--border);
      color: var(--foreground);
      cursor: pointer;
      box-shadow: var(--shadow-xs);
      transition: background .12s, border-color .12s, transform .1s;
    }
    .toolbar-btn:hover {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
    }
    .toolbar-btn--icon { padding: 7px 9px; gap: 0; }
    .toolbar-btn--primary {
      background: var(--primary);
      color: var(--primary-foreground);
      border-color: var(--primary);
      box-shadow: var(--shadow-primary);
    }
    .toolbar-btn--primary:hover {
      background: var(--primary);
      color: var(--primary-foreground);
      transform: translateY(-1px);
    }

    .toolbar-btn[disabled] { opacity: .55; cursor: default; transform: none; }

    .toolbar-title--create {
      display: flex;
      gap: 8px;
      align-items: center;
    }
    .toolbar-title--create input { width: 150px; }

    .builder-status {
      padding: 48px 24px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 14px;
      display: flex;
      flex-direction: column;
      gap: 10px;
      align-items: center;
    }
    .builder-status--error strong { color: var(--destructive); }

    .action-toast {
      position: absolute;
      top: 12px;
      left: 50%;
      transform: translateX(-50%);
      z-index: 20;
      padding: 8px 16px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 500;
      box-shadow: var(--shadow-md, 0 4px 12px rgba(0,0,0,.15));
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--foreground);
    }
    .action-toast--ok { border-color: var(--success); color: var(--success); }
    .action-toast--warn { border-color: var(--warning); color: var(--warning); }
    .action-toast--error { border-color: var(--destructive); color: var(--destructive); }

    /* ============ 3-pane body ============ */
    .body {
      display: grid;
      grid-template-columns: 220px 1fr 344px;
      flex: 1;
      min-height: 0;
    }

    .pane {
      display: flex;
      flex-direction: column;
      min-height: 0;
      background: var(--card);
    }

    .pane-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 12px 12px 8px 16px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }
    .pane-collapse {
      border: none;
      background: transparent;
      cursor: pointer;
      color: var(--muted-foreground);
      display: inline-flex;
      padding: 2px;
      border-radius: 5px;
    }
    .pane-collapse:hover { background: var(--muted); color: var(--foreground); }

    /* Collapsed pane = a thin rail with an expand button + rotated label. */
    .pane-rail {
      border-right: 1px solid var(--border);
      align-items: center;
      padding-top: 10px;
      gap: 10px;
      overflow: hidden;
    }
    .pane.inspector + .pane-rail, .pane-rail:last-child { border-right: none; border-left: 1px solid var(--border); }
    .pane-rail-btn {
      border: 1px solid var(--border);
      background: var(--card);
      cursor: pointer;
      color: var(--muted-foreground);
      display: inline-flex;
      padding: 5px;
      border-radius: 7px;
    }
    .pane-rail-btn:hover { color: var(--foreground); border-color: color-mix(in oklch, var(--primary) 35%, var(--border)); }
    .pane-rail-label {
      writing-mode: vertical-rl;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: var(--muted-foreground);
    }

    /* ----- Palette ----- */
    /* overflow:hidden keeps the search box pinned; only .palette-list scrolls. */
    .palette {
      border-right: 1px solid var(--border);
      overflow: hidden;
    }
    .palette-search {
      padding: 8px;
      border-bottom: 1px solid var(--border);
    }
    .palette-search-box {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 9px;
      border: 1px solid var(--border);
      border-radius: 8px;
      background: var(--background);
      color: var(--muted-foreground);
    }
    .palette-search-box:focus-within {
      border-color: color-mix(in oklch, var(--primary) 45%, var(--border));
    }
    .palette-search-input {
      border: none;
      background: transparent;
      outline: none;
      font: inherit;
      font-size: 13px;
      color: var(--foreground);
      width: 100%;
      min-width: 0;
    }
    .palette-search-clear {
      border: none;
      background: transparent;
      cursor: pointer;
      color: var(--muted-foreground);
      display: inline-flex;
      padding: 0;
      flex-shrink: 0;
    }
    .palette-search-clear:hover { color: var(--foreground); }
    .palette-list {
      padding: 8px;
      display: flex;
      flex-direction: column;
      gap: 2px;
      flex: 1;
      overflow-y: auto;
    }
    .palette-group-label {
      display: flex;
      align-items: center;
      gap: 6px;
      width: 100%;
      border: none;
      background: transparent;
      cursor: pointer;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      padding: 10px 10px 4px;
      margin-top: 4px;
      border-radius: 6px;
    }
    .palette-group-label:hover { color: var(--foreground); }
    .palette-group-label:first-child { margin-top: 0; padding-top: 4px; }
    .palette-group-label .chev { flex-shrink: 0; }
    .palette-group-label .grp-count {
      margin-left: auto;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
    }
    .palette-empty {
      padding: 20px 10px;
      font-size: 12px;
      color: var(--muted-foreground);
      text-align: center;
      line-height: 1.4;
    }
    .palette-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 9px 10px;
      border-radius: 8px;
      border: 1px solid transparent;
      cursor: grab;
      transition: background .12s, border-color .12s;
      background: transparent;
      text-align: left;
      user-select: none;
    }
    .palette-item:hover {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 25%, var(--border));
    }
    .palette-item:active { cursor: grabbing; }
    .palette-item--disabled { cursor: default; opacity: .7; }
    .palette-item--disabled:hover { background: transparent; border-color: transparent; }
    .palette-soon {
      flex-shrink: 0;
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      background: var(--muted);
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 1px 6px;
    }
    .palette-item .icon-tile {
      width: 28px; height: 28px;
      border-radius: 7px;
      display: inline-flex; align-items: center; justify-content: center;
      flex-shrink: 0;
    }
    .icon-tile--neutral     { background: var(--muted); color: var(--muted-foreground); }
    .icon-tile--primary     { background: color-mix(in oklch, var(--primary) 18%, transparent); color: var(--primary); }
    .icon-tile--success     { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .icon-tile--warning     { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    .icon-tile--destructive { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }

    .palette-item-text {
      min-width: 0;
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 1px;
    }
    .palette-item-label {
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
    }
    .palette-item-desc {
      font-size: 11px;
      color: var(--muted-foreground);
      line-height: 1.3;
    }

    /* ----- Canvas ----- */
    /* Fixed viewport: the SVG fills the pane and pan/zoom happen via a <g>
       transform (react-flow style). No scrollbars — the dot grid is bound to
       pan/zoom inline so it tracks the world. */
    .canvas-wrap {
      position: relative;
      overflow: hidden;
      min-width: 0;
      background-image:
        radial-gradient(circle, color-mix(in oklch, var(--muted-foreground) 18%, transparent) 1px, transparent 1px);
      background-color: var(--background);
    }
    .canvas-svg {
      display: block;
      width: 100%;
      height: 100%;
      touch-action: none;
    }
    .canvas-svg .canvas-bg { cursor: grab; }
    .canvas-svg.is-panning .canvas-bg { cursor: grabbing; }
    .zoom-controls {
      position: absolute;
      right: 14px;
      bottom: 44px;
      display: flex;
      align-items: center;
      gap: 2px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 9px;
      box-shadow: var(--shadow-md);
      padding: 2px;
      z-index: 4;
    }
    .zoom-controls button {
      border: none;
      background: transparent;
      cursor: pointer;
      color: var(--muted-foreground);
      display: inline-flex;
      align-items: center;
      padding: 5px;
      border-radius: 6px;
    }
    .zoom-controls button:hover { background: var(--muted); color: var(--foreground); }
    .zoom-controls .zoom-pct { font-size: 11px; font-variant-numeric: tabular-nums; min-width: 38px; text-align: center; }

    .edge {
      fill: none;
      stroke: var(--border);
      stroke-width: 1.5;
    }
    .edge--success { stroke: color-mix(in oklch, var(--success) 80%, transparent); }
    .edge--timeout { stroke: color-mix(in oklch, var(--destructive) 70%, transparent); stroke-dasharray: 6 4; }
    /* An edge flagged by validation (e.g. region_boundary_crossing) — paint it
       red so the user can find what the issue panel names. */
    .edge--invalid { stroke: var(--destructive); stroke-width: 2.5; stroke-dasharray: 5 3; }
    /* Edge labels: visible colored pills at the midpoint of each bezier
       carrying the case name (yes / no / ok / error / timeout / case_X)
       so the reader can tell at a glance which branch an edge represents
       without tracing back to the source node's chip rail. */
    .edge-label-bg {
      fill: var(--card);
      stroke: var(--border);
      stroke-width: 1;
      rx: 4;
    }
    .edge-label-bg--success { fill: color-mix(in oklch, var(--success) 16%, var(--card)); stroke: var(--success); }
    .edge-label-bg--error   { fill: color-mix(in oklch, var(--destructive) 14%, var(--card)); stroke: var(--destructive); }
    .edge-label-bg--timeout { fill: color-mix(in oklch, var(--warning) 22%, var(--card)); stroke: var(--warning); }
    .edge-label-bg--fallback { fill: color-mix(in oklch, var(--destructive) 10%, var(--card)); stroke: var(--destructive); stroke-dasharray: 3 2; }
    .edge-label {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 10px;
      font-weight: 700;
      fill: var(--foreground);
      letter-spacing: 0.02em;
    }
    .edge-label--success { fill: var(--success); }
    .edge-label--error   { fill: var(--destructive); }
    .edge-label--timeout { fill: var(--warning); }
    .edge-label--fallback{ fill: var(--destructive); }
    .edge--draft {
      stroke: var(--primary);
      stroke-dasharray: 2 5;
      stroke-width: 2;
      stroke-linecap: round;
      opacity: 0.8;
      pointer-events: none;
    }
    /* Wide invisible hit-target so thin edges are easy to click-select. */
    .edge-hit { stroke: transparent; stroke-width: 14; fill: none; cursor: pointer; }
    .edge--selected { stroke: var(--primary) !important; stroke-width: 2.5; }
    .node-card-port--handle { cursor: crosshair; }
    .node-card-port--handle:hover {
      outline: 2px solid color-mix(in oklch, var(--primary) 50%, transparent);
      outline-offset: 1px;
    }

    .node-card {
      display: flex;
      flex-direction: column;
      gap: 4px;
      width: 220px;
      height: 132px;
      padding: 9px 11px;
      border-radius: 10px;
      background: var(--card);
      border: 1.5px solid var(--border);
      box-shadow: var(--shadow-sm);
      cursor: pointer;
      transition: border-color .12s, box-shadow .12s;
      box-sizing: border-box;
      position: relative;
    }
    /* Grab cursor only in edit mode — sim mode renders cards read-only;
       showing grab would imply dragging during playback is supported. */
    .canvas-wrap:not(.canvas-wrap--sim) .node-card { cursor: grab; }
    .node-card.is-dragging {
      cursor: grabbing;
      box-shadow: var(--shadow-lg);
      opacity: 0.9;
      z-index: 10;
    }

    /* (Right-edge handle dots removed when the layout pivoted from
       horizontal to vertical — outputs are now expressed exclusively as
       .node-card-port chips at the bottom of each card.) */
    .node-card:hover {
      border-color: color-mix(in oklch, var(--primary) 50%, var(--border));
      box-shadow: var(--shadow-md);
    }
    .node-card--selected {
      border-color: var(--primary);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 25%, transparent), var(--shadow-md);
    }
    .node-card--invalid {
      border-color: var(--destructive);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--destructive) 22%, transparent), var(--shadow-md);
    }
    .node-card--drop-target {
      border-color: var(--success);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--success) 30%, transparent), var(--shadow-md);
    }
    .node-card--drop-invalid {
      border-color: var(--destructive);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--destructive) 28%, transparent), var(--shadow-md);
      cursor: not-allowed;
    }
    .node-input-anchor {
      fill: var(--card);
      stroke: var(--muted-foreground);
      stroke-width: 1.5;
    }
    .node-input-anchor--active { fill: var(--success); stroke: var(--success); }

    /* ----- Control-flow body region frames ----- */
    .region-frame {
      fill: color-mix(in oklch, var(--primary) 5%, transparent);
      stroke: color-mix(in oklch, var(--primary) 40%, var(--border));
      stroke-width: 1.5;
      stroke-dasharray: 6 4;
    }
    .region--warn .region-frame {
      fill: color-mix(in oklch, var(--warning) 6%, transparent);
      stroke: color-mix(in oklch, var(--warning) 45%, var(--border));
    }
    .region-label {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.03em;
      text-transform: uppercase;
      color: color-mix(in oklch, var(--primary) 75%, var(--foreground));
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .region--warn .region-label { color: color-mix(in oklch, var(--warning) 80%, var(--foreground)); }
    /* Drop-zone highlight while dragging a palette node over a body frame. */
    .region--drop .region-frame {
      fill: color-mix(in oklch, var(--primary) 12%, transparent);
      stroke: var(--primary);
      stroke-dasharray: none;
      stroke-width: 2;
    }
    .region-banner {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      margin-bottom: 12px;
      padding: 7px 10px;
      border-radius: 8px;
      font-size: 11.5px;
      background: color-mix(in oklch, var(--primary) 7%, var(--card));
      border: 1px solid color-mix(in oklch, var(--primary) 22%, var(--border));
      color: var(--foreground);
    }
    .region-banner span { display: inline-flex; align-items: center; gap: 5px; }
    .control-help {
      margin-bottom: 14px;
      padding: 9px 11px;
      border-radius: 8px;
      background: color-mix(in oklch, var(--primary) 4%, var(--card));
      border: 1px solid var(--border);
      font-size: 11.5px;
      line-height: 1.5;
    }
    .control-help-head {
      display: inline-flex; align-items: center; gap: 5px;
      font-weight: 700; font-size: 11px; text-transform: uppercase; letter-spacing: 0.03em;
      color: var(--muted-foreground); margin-bottom: 6px;
    }
    .control-help-row { display: flex; gap: 7px; margin-bottom: 3px; }
    .control-help-row code {
      flex-shrink: 0; min-width: 62px;
      font-family: var(--uk-font-monospace, monospace); font-size: 10.5px; font-weight: 700;
      color: color-mix(in oklch, var(--primary) 80%, var(--foreground));
    }
    .control-help-row span { color: var(--muted-foreground); }
    .control-help-note { margin-top: 6px; color: var(--muted-foreground); }

    /* Control-flow nesting chips on a trace step. */
    .nest-row { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 5px; }
    .nest-chip {
      display: inline-flex;
      align-items: center;
      gap: 3px;
      padding: 1px 6px;
      border-radius: 10px;
      font-size: 10px;
      font-weight: 600;
      font-family: var(--uk-font-monospace, monospace);
      background: var(--muted);
      color: var(--muted-foreground);
    }
    .nest-chip--region {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: color-mix(in oklch, var(--primary) 80%, var(--foreground));
    }
    .nest-chip--caught {
      background: color-mix(in oklch, var(--warning) 18%, transparent);
      color: color-mix(in oklch, var(--warning) 85%, var(--foreground));
    }

    /* ----- Validation panel (docked in the inspector) ----- */
    .validation-panel {
      margin: 10px 12px;
      border: 1px solid var(--destructive);
      border-radius: 10px;
      background: color-mix(in oklch, var(--destructive) 5%, var(--card));
      font-size: 12px;
      overflow: hidden;
    }
    .validation-panel-head {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 9px 12px;
      color: var(--destructive);
      border-bottom: 1px solid var(--border);
      background: var(--card);
    }
    .validation-panel--ok { border-color: var(--success); background: color-mix(in oklch, var(--success) 6%, var(--card)); }
    .validation-panel-head--ok { color: var(--success); border-bottom: none; }
    .validation-panel-close {
      margin-left: auto;
      border: none;
      background: transparent;
      cursor: pointer;
      color: var(--muted-foreground);
      display: inline-flex;
      padding: 0;
    }
    .validation-panel-close:hover { color: var(--foreground); }
    .validation-list { list-style: none; margin: 0; padding: 6px 0; }
    .validation-list li {
      display: flex;
      flex-wrap: wrap;
      align-items: baseline;
      gap: 6px;
      padding: 6px 12px;
      border-top: 1px solid color-mix(in oklch, var(--border) 60%, transparent);
    }
    .validation-list li:first-child { border-top: none; }
    .validation-list code {
      font-size: 11px;
      color: var(--destructive);
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
      padding: 1px 5px;
      border-radius: 4px;
    }
    .validation-loc { color: var(--muted-foreground); font-size: 11px; }
    .validation-msg { flex-basis: 100%; color: var(--foreground); }

    /* ----- Publish popover ----- */
    .publish-popover {
      position: absolute;
      right: 16px;
      top: 64px;
      width: 240px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: var(--shadow-lg, 0 8px 24px rgba(0,0,0,.18));
      padding: 12px;
      z-index: 6;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
    .publish-popover-title {
      font-size: 12px;
      font-weight: 600;
      color: var(--foreground);
    }
    .publish-field { display: flex; flex-direction: column; gap: 3px; }
    .publish-field span { font-size: 11px; color: var(--muted-foreground); }
    .publish-field input {
      border: 1px solid var(--border);
      border-radius: 7px;
      padding: 6px 9px;
      font: inherit;
      font-size: 13px;
      background: var(--background);
      color: var(--foreground);
    }
    .publish-field input:focus { outline: none; border-color: color-mix(in oklch, var(--primary) 45%, var(--border)); }
    .publish-actions { display: flex; justify-content: flex-end; gap: 8px; }
    .node-card-head {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .node-card-head .icon-tile {
      width: 22px; height: 22px;
      border-radius: 6px;
      display: inline-flex; align-items: center; justify-content: center;
    }
    .node-card-kind {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      color: var(--muted-foreground);
    }
    .node-card-label {
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      line-height: 1.2;
      /* Truncate long labels so the card never grows past the foreignObject
         clip rect — labels like "Extract customer_id" or "Supervisor approval"
         would wrap to 2 lines otherwise. Full label still visible on hover
         via title= on the card. */
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .node-card-param {
      font-size: 11px;
      color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
      line-height: 1.3;
      /* Wrap to 2 lines then ellipsis — long expressions were truncated to ~20
         chars at the old 168px width. Full text still on hover via title= on the
         element. */
      display: -webkit-box;
      -webkit-line-clamp: 2;
      line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
      word-break: break-word;
    }

    /* Mini status strip */
    .canvas-strip {
      position: sticky;
      bottom: 0;
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 8px 16px;
      background: color-mix(in oklch, var(--card) 95%, transparent);
      backdrop-filter: blur(6px);
      border-top: 1px solid var(--border);
      font-size: 12px;
      color: var(--muted-foreground);
      z-index: 10;
    }
    .canvas-strip strong { color: var(--foreground); font-weight: 600; }
    .canvas-strip .dot-ok   { color: var(--success); }
    .canvas-strip .dot-warn { color: var(--warning); }

    /* ----- Inspector ----- */
    .inspector {
      border-left: 1px solid var(--border);
      overflow-y: auto;
    }
    .inspector-body { padding: 16px; }
    .inspector-empty {
      padding: 40px 20px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 13px;
    }
    .inspector-empty uk-icon { color: var(--muted-foreground); opacity: .45; }
    .inspector-empty p { margin: 12px 0 0; }

    .inspector .node-card {
      width: auto;
      cursor: default;
      margin-bottom: 16px;
    }
    .inspector .node-card:hover {
      border-color: var(--border);
      box-shadow: var(--shadow-sm);
    }

    .form-section {
      display: flex;
      flex-direction: column;
      gap: 4px;
      margin-bottom: 14px;
    }
    .form-section label {
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      color: var(--muted-foreground);
    }
    .form-section .form-value {
      font-size: 13px;
      color: var(--foreground);
      font-family: var(--uk-font-monospace, monospace);
      background: var(--muted);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 6px 9px;
      word-break: break-all;
    }
    .form-section .form-input {
      font: inherit;
      font-size: 13px;
      color: var(--foreground);
      background: var(--background);
      border: 1px solid var(--border);
      border-radius: 7px;
      padding: 6px 9px;
      width: 100%;
      box-sizing: border-box;
      resize: vertical;
    }
    .form-section .form-input:focus {
      outline: none;
      border-color: color-mix(in oklch, var(--primary) 45%, var(--border));
    }
    .form-section textarea.form-input {
      font-family: var(--uk-font-monospace, monospace);
      line-height: 1.45;
    }
    .form-section .field-hint {
      margin-top: 4px;
      font-size: 11px;
      color: var(--muted-foreground);
    }
    .form-section .field-hint code {
      font-family: var(--uk-font-monospace, monospace);
      background: var(--muted);
      padding: 0 3px;
      border-radius: 3px;
    }
    /* One consistent chevron for every <select> — the raw native arrow differs
       per OS/browser, which read as "each dropdown a different style". */
    .form-section select.form-input {
      appearance: none;
      -webkit-appearance: none;
      -moz-appearance: none;
      padding-right: 26px;
      background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23888' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
      background-repeat: no-repeat;
      background-position: right 8px center;
      cursor: pointer;
    }
    .form-section label { display: flex; align-items: center; justify-content: space-between; }
    .expr-mode {
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--muted-foreground);
      cursor: pointer;
      font-size: 10px;
      text-transform: none;
      letter-spacing: 0;
      padding: 2px 7px;
      border-radius: 999px;
    }
    .expr-mode:hover { color: var(--foreground); border-color: color-mix(in oklch, var(--primary) 35%, var(--border)); }
    /* Variable on its own full-width row, then [op][value] below — a single
       line truncated long dotted vars (customer.tier → "custome") in the
       narrow inspector, worse once a group nests. */
    .cond-builder { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 5px; }
    .cond-builder .cond-lhs { grid-column: 1 / -1; }
    .cond-builder .cond-op { min-width: 64px; padding-right: 24px; }
    .expr-hint { font-size: 10.5px; color: var(--muted-foreground); margin-top: 4px; line-height: 1.4; }
    .expr-hint code {
      background: var(--muted);
      border-radius: 4px;
      padding: 0 3px;
      font-size: 10px;
    }
    .expr-toolbar { display: flex; align-items: center; gap: 8px; margin-top: 5px; }
    .expr-fn-btn {
      display: inline-flex; align-items: center; gap: 4px;
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--muted-foreground);
      cursor: pointer;
      font-size: 10.5px;
      padding: 2px 8px;
      border-radius: 6px;
    }
    .expr-fn-btn:hover { color: var(--foreground); border-color: color-mix(in oklch, var(--primary) 35%, var(--border)); }
    .expr-picker {
      margin-top: 6px;
      border: 1px solid var(--border);
      border-radius: 8px;
      background: var(--card);
      max-height: 220px;
      overflow-y: auto;
      padding: 4px;
    }
    .expr-picker-ns {
      font-size: 9.5px;
      font-weight: 600;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      padding: 5px 6px 2px;
    }
    .expr-picker-item {
      display: block; width: 100%; text-align: left;
      border: 0; background: transparent;
      color: var(--foreground);
      cursor: pointer;
      font-family: var(--font-mono, monospace);
      font-size: 11px;
      padding: 4px 6px;
      border-radius: 5px;
    }
    .expr-picker-item:hover { background: var(--muted); }
    .expr-preview {
      display: flex; align-items: center; gap: 6px;
      margin-top: 6px;
      padding: 5px 7px;
      background: var(--muted);
      border-radius: 6px;
    }
    .expr-preview code {
      flex: 1; min-width: 0;
      font-size: 10.5px;
      color: var(--foreground);
      overflow-wrap: anywhere;
    }
    .expr-copy {
      flex-shrink: 0;
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--muted-foreground);
      cursor: pointer;
      padding: 2px 5px;
      border-radius: 5px;
      display: inline-flex; align-items: center;
    }
    .expr-copy:hover:not([disabled]) { color: var(--foreground); }
    .expr-copy[disabled] { opacity: 0.4; cursor: default; }
    /* Single combobox: the field is the search input; the filtered list is an
       overlay below it (absolute so it doesn't shove the rest of the form). */
    .catalog-combo { position: relative; }
    .catalog-input { padding-right: 26px; }
    .catalog-input--unknown { border-color: var(--destructive, #e5484d); }
    .catalog-clear {
      position: absolute; top: 50%; right: 6px; transform: translateY(-50%);
      border: 0; background: transparent; color: var(--muted-foreground);
      cursor: pointer; padding: 2px; border-radius: 5px;
      display: inline-flex; align-items: center;
    }
    .catalog-clear:hover { color: var(--foreground); }
    .catalog-list {
      position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 20;
      max-height: 220px; overflow-y: auto;
      border: 1px solid var(--border); border-radius: 8px;
      background: var(--card); box-shadow: var(--shadow-md); padding: 4px;
    }
    .catalog-empty { font-size: 11px; color: var(--muted-foreground); padding: 8px; }
    .catalog-item {
      display: flex; flex-direction: column; gap: 1px; width: 100%; text-align: left;
      border: 0; background: transparent; cursor: pointer; padding: 5px 7px; border-radius: 5px;
    }
    .catalog-item:hover, .catalog-item.on { background: var(--muted); }
    .catalog-item-code { font-size: 12px; font-weight: 600; color: var(--foreground); }
    .catalog-item-name { font-size: 10.5px; color: var(--muted-foreground); }
    .cond-group {
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 8px;
      display: flex; flex-direction: column; gap: 6px;
    }
    .cond-group--nested {
      border-left: 2px solid color-mix(in oklch, var(--primary) 40%, var(--border));
      background: color-mix(in oklch, var(--muted) 45%, transparent);
    }
    .cond-group-head { display: flex; align-items: center; gap: 10px; }
    .cond-andor {
      display: inline-flex;
      border: 1px solid var(--border);
      border-radius: 6px;
      overflow: hidden;
    }
    .cond-andor button {
      border: 0; background: var(--card);
      color: var(--muted-foreground);
      cursor: pointer;
      font-size: 10.5px; font-weight: 600;
      padding: 2px 10px;
    }
    .cond-andor button.on { background: var(--primary); color: var(--primary-foreground); }
    .cond-not {
      display: inline-flex; align-items: center; gap: 4px;
      font-size: 10.5px; color: var(--muted-foreground); cursor: pointer;
    }
    .cond-child { display: flex; align-items: flex-start; gap: 6px; }
    .cond-child > :first-child { flex: 1; min-width: 0; }
    .cond-rm {
      flex-shrink: 0;
      border: 1px solid var(--border);
      background: var(--card);
      color: var(--muted-foreground);
      cursor: pointer;
      padding: 4px 5px;
      border-radius: 5px;
      display: inline-flex; align-items: center;
      margin-top: 1px;
    }
    .cond-rm:hover { color: var(--destructive, #e5484d); border-color: color-mix(in oklch, var(--destructive, #e5484d) 35%, var(--border)); }
    .cond-add { display: flex; gap: 6px; }
    .cond-add button {
      border: 1px dashed var(--border);
      background: transparent;
      color: var(--muted-foreground);
      cursor: pointer;
      font-size: 10.5px;
      padding: 3px 9px;
      border-radius: 6px;
    }
    .cond-add button:hover { color: var(--foreground); border-color: color-mix(in oklch, var(--primary) 35%, var(--border)); }
    .cond-truthy {
      display: inline-flex; align-items: center;
      font-size: 10.5px; font-style: italic;
      color: var(--muted-foreground);
      padding: 0 4px;
    }
    .form-section .chip-row {
      display: flex;
      gap: 4px;
      flex-wrap: wrap;
    }
    .form-section .chip-row .chip {
      padding: 2px 8px;
      border-radius: 4px;
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
      font-size: 11px;
      font-weight: 600;
      font-family: var(--uk-font-monospace, monospace);
    }

    /* Code block for script / DSL params. Same dark-ish-but-tokenised
       palette as the trace-viewer JSON block. */
    .code-block {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      line-height: 1.55;
      background: var(--muted);
      color: var(--foreground);
      border: 1px solid var(--border);
      padding: 8px 10px 8px 8px;
      border-radius: 6px;
      white-space: pre;
      overflow-x: auto;
      margin: 0;
    }
    .code-ln {
      display: inline-block;
      width: 24px;
      padding-right: 10px;
      margin-right: 6px;
      border-right: 1px solid var(--border);
      color: var(--muted-foreground);
      text-align: right;
      user-select: none;
      font-variant-numeric: tabular-nums;
      opacity: 0.7;
    }

    /* Output port list (inspector) — explicit "this node produces these cases" */
    .port-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .port-row {
      display: grid;
      grid-template-columns: 10px 1fr auto;
      align-items: center;
      gap: 8px;
      padding: 4px 8px;
      border-radius: 5px;
      background: var(--background);
      border: 1px solid var(--border);
    }
    .port-dot {
      width: 8px; height: 8px; border-radius: 50%;
    }
    .port-row--success .port-dot { background: var(--success); }
    .port-row--error   .port-dot { background: var(--destructive); }
    .port-row--timeout .port-dot { background: var(--warning); }
    .port-row--branch  .port-dot { background: var(--foreground); }
    .port-row--default .port-dot { background: var(--muted-foreground); }
    .port-row-label {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      font-weight: 600;
      color: var(--foreground);
    }
    .port-row-kind {
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      color: var(--muted-foreground);
    }

    /* ======================================================
       SIM MODE
       ====================================================== */

    .toolbar--sim {
      background: linear-gradient(180deg, color-mix(in oklch, var(--primary) 4%, var(--card)), var(--card));
      border-bottom-color: color-mix(in oklch, var(--primary) 35%, var(--border));
    }

    .sim-badge {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 3px 8px;
      border-radius: 4px;
      background: color-mix(in oklch, var(--primary) 14%, transparent);
      color: var(--primary);
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.06em;
    }

    .toolbar-btn--sim-active {
      background: var(--primary);
      color: var(--primary-foreground);
      border-color: var(--primary);
    }
    .toolbar-btn--sim-active:hover {
      background: var(--primary);
      color: var(--primary-foreground);
    }

    /* Scenario badge — read-only. Scenario is owned by the active test
       case (Run a case from the Test cases tab to switch). The old
       Success/Failure toggle invited people to think scenarios were a
       debug-only concept disconnected from the regression suite. */
    .sim-scenario-badge {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 5px 10px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      cursor: help;
    }
    .sim-scenario-badge--success {
      background: color-mix(in oklch, var(--success) 18%, transparent);
      color: var(--success);
    }
    .sim-scenario-badge--fail {
      background: color-mix(in oklch, var(--destructive) 14%, transparent);
      color: var(--destructive);
    }

    /* Playback controls */
    .sim-playback {
      display: inline-flex;
      gap: 4px;
      padding: 3px;
      border: 1px solid var(--border);
      border-radius: 8px;
      background: var(--card);
      box-shadow: var(--shadow-xs);
    }
    .sim-pb-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 30px;
      height: 28px;
      padding: 0 8px;
      border-radius: 6px;
      background: transparent;
      border: none;
      color: var(--muted-foreground);
      cursor: pointer;
      gap: 4px;
      font-size: 12px;
      font-weight: 500;
      transition: background .12s, color .12s;
    }
    .sim-pb-btn:hover:not(:disabled) {
      background: var(--muted);
      color: var(--foreground);
    }
    .sim-pb-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
    /* A light primary-tinted button with a CORAL icon — a white icon on a solid
       --primary background went invisible (uk-icon color issue). Coral-on-tint
       both shows the icon and keeps the step-forward emphasis (user report). */
    .sim-pb-btn--primary {
      background: color-mix(in oklch, var(--primary) 16%, var(--card));
      color: var(--primary);
    }
    .sim-pb-btn--primary:hover:not(:disabled) {
      background: color-mix(in oklch, var(--primary) 26%, var(--card));
      color: var(--primary);
      filter: brightness(1.05);
    }

    .sim-step-counter {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      color: var(--muted-foreground);
      padding: 0 6px;
    }
    .sim-step-counter strong {
      color: var(--foreground);
      font-variant-numeric: tabular-nums;
      font-weight: 700;
    }

    /* Canvas tweaks in sim mode */
    .canvas-wrap--sim {
      background-color: color-mix(in oklch, var(--primary) 2%, var(--background));
    }

    /* Node states layered on top of base node-card */
    .node-card--dim { opacity: 0.45; }
    .node-card--hit { opacity: 1; border-color: color-mix(in oklch, var(--primary) 60%, var(--border)); }
    .node-card--active {
      opacity: 1;
      border-color: var(--primary);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 25%, transparent), var(--shadow-md);
      animation: nodePulse 1.6s ease-in-out infinite;
    }
    .node-card--fail {
      border-color: var(--destructive);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--destructive) 22%, transparent);
    }
    /* Active + fail: the fail ring is more important than the "you are here"
       pulse. Override the pulse keyframe with destructive tint, otherwise
       the failing current step reads as coral instead of red. */
    .node-card--active.node-card--fail { animation-name: nodePulseFail; }
    @keyframes nodePulse {
      0%, 100% { box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 25%, transparent), var(--shadow-md); }
      50%      { box-shadow: 0 0 0 5px color-mix(in oklch, var(--primary) 15%, transparent), var(--shadow-md); }
    }
    @keyframes nodePulseFail {
      0%, 100% { box-shadow: 0 0 0 3px color-mix(in oklch, var(--destructive) 28%, transparent), var(--shadow-md); }
      50%      { box-shadow: 0 0 0 5px color-mix(in oklch, var(--destructive) 16%, transparent), var(--shadow-md); }
    }

    .node-card-head {
      position: relative;
    }
    .node-card-badge {
      position: absolute;
      right: -4px;
      top: -6px;
      width: 16px; height: 16px;
      border-radius: 50%;
      display: inline-flex; align-items: center; justify-content: center;
      border: 1.5px solid var(--card);
    }
    .node-card-badge--ok      { background: var(--success); color: var(--success-foreground); }
    .node-card-badge--fail    { background: var(--destructive); color: var(--destructive-foreground); }
    .node-card-badge--timeout { background: var(--warning); color: var(--warning-foreground); }
    .node-card-badge--skipped { background: var(--muted-foreground); color: var(--card); }

    .node-card-timing {
      font-size: 10px;
      color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
      margin-top: 2px;
    }


    /* Output port chips at the bottom of the node card. Fixed-height strip
       (24px tall) anchored to the card bottom so edges drawn by portY() —
       which assumes anchors lie in [NODE_H-30, NODE_H-10] — meet the chip
       row's actual visual position deterministically (design review MUST). */
    .node-card-ports {
      display: flex;
      gap: 2px;             /* tightened from 3px so SWITCH/CASE 5 chips fit the card width */
      flex-wrap: nowrap;
      /* overflow MUST stay visible: the port chip's :hover outline (offset 1px,
         2px wide) extends ~3px past the chip, and overflow:hidden clipped its
         bottom edge flat — read as "the Done chip lost its bottom border on
         hover". Chips shrink (below) instead of spilling horizontally. */
      overflow: visible;
      align-items: center;
      justify-content: space-around;
      margin-top: auto;
      padding-top: 6px;
      height: 24px;
      box-sizing: border-box;
      border-top: 1px dashed var(--border);
    }
    .node-card-port {
      font-size: 9px;
      font-weight: 700;
      font-family: var(--uk-font-monospace, monospace);
      letter-spacing: 0.02em;
      padding: 1px 5px;
      border-radius: 3px;
      line-height: 1.4;
      flex-shrink: 1;
      min-width: 0;
      max-width: 60px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .node-card-port--success { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .node-card-port--error   { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }
    .node-card-port--timeout { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    /* Branch ports use a neutral tone, not primary coral. Coral was
       indistinguishable from the active-step pulse ring when sim mode
       runs (design review SHOULD). The label itself carries semantics. */
    .node-card-port--branch  { background: var(--muted); color: var(--foreground); }
    .node-card-port--default { background: var(--muted); color: var(--muted-foreground); }

    /* Sim: reservation port chips are clickable to force that branch. */
    .node-card-port--pick { cursor: pointer; transition: outline-color 80ms, box-shadow 80ms; }
    .node-card-port--pick:hover { outline: 1.5px solid currentColor; outline-offset: 1px; }
    .node-card-port--pinned {
      outline: 1.5px solid currentColor;
      outline-offset: 1px;
      box-shadow: 0 0 0 3px color-mix(in oklch, currentColor 22%, transparent);
    }
    /* The chips outgrow the row's clip box when ringed — let them show. */
    .node-card-ports--pick { padding-bottom: 2px; }

    /* Rendered outside .inspector-body, so it carries its own padding + a
       divider to the Step I/O block below. */
    .sim-outcome {
      margin: 0;
      padding: 14px 16px;
      border-bottom: 1px solid var(--border);
    }
    .sim-outcome-hint {
      margin: 0;
      font-size: 11px;
      line-height: 1.5;
      color: var(--muted-foreground);
    }
    .sim-outcome-hint strong { font-weight: 700; }
    .sim-outcome-hint .node-card-port { display: inline; padding: 0 4px; }
    .linkish {
      background: none; border: none; padding: 0;
      color: var(--primary); font: inherit; cursor: pointer; text-decoration: underline;
    }
    .linkish:hover { color: var(--foreground); }

    /* ----- Run panel (bottom) ----- */
    /* Align columns to the body grid (220 / 1fr / 320) so dividers don't jog
       across the seam. Design review caught this. */
    .run-panel {
      display: grid;
      grid-template-columns: 220px 1fr 344px;
      gap: 0;
      flex-shrink: 0;
      background: var(--card);
      border-top: 1px solid color-mix(in oklch, var(--primary) 35%, var(--border));
      box-shadow: 0 -4px 12px -8px color-mix(in oklch, var(--primary) 30%, transparent);
      min-height: 240px;
      max-height: 320px;
    }
    .run-panel-col {
      display: flex;
      flex-direction: column;
      padding: 12px 16px;
      border-right: 1px solid var(--border);
      min-height: 0;
      min-width: 0;
    }
    .run-panel-col:last-child { border-right: none; }
    /* Narrow 220px column: trim side padding so inputs get the width, and let
       the row's own 6px padding carry the inset. */
    .run-panel-col--vars { padding: 12px 8px 12px 12px; }
    .run-panel-col--center {
      background: color-mix(in oklch, var(--primary) 5%, var(--card));
    }

    .run-panel-header {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      margin-bottom: 10px;
    }
    .run-panel-count {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 18px;
      height: 16px;
      padding: 0 5px;
      border-radius: 8px;
      background: var(--muted);
      color: var(--muted-foreground);
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0;
    }

    /* Init vars list — narrow column (220px), so stack key+tag above input. */
    .initvar-list {
      display: flex;
      flex-direction: column;
      gap: 6px;
      overflow-y: auto;
      min-height: 0;
      /* Hide the overlay scrollbar — it floated over the input values in this
         narrow column. Still scrolls (wheel/trackpad/keyboard). */
      scrollbar-width: none;
    }
    .initvar-list::-webkit-scrollbar { width: 0; height: 0; }
    .initvar-row {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 4px 6px;
      border-radius: 6px;
      transition: background .12s;
    }
    .initvar-row:hover { background: var(--muted); }
    .initvar-row-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 4px;
    }
    .initvar-key {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      color: var(--foreground);
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .initvar-input {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      padding: 3px 7px;
      background: var(--background);
      border: 1px solid var(--border);
      border-radius: 4px;
      color: var(--foreground);
      width: 100%;
      box-sizing: border-box;
      min-width: 0;
    }
    .initvar-input:focus {
      outline: none;
      border-color: var(--primary);
      box-shadow: 0 0 0 2px color-mix(in oklch, var(--primary) 18%, transparent);
    }
    .initvar-input[readonly] {
      background: var(--muted);
      color: var(--muted-foreground);
      cursor: not-allowed;
    }
    .initvar-tag {
      font-size: 9px;
      font-weight: 700;
      padding: 2px 6px;
      border-radius: 3px;
      background: var(--muted);
      color: var(--muted-foreground);
      letter-spacing: 0.05em;
      text-transform: uppercase;
      text-align: center;
    }
    .initvar-tag--trigger {
      background: color-mix(in oklch, var(--primary) 14%, transparent);
      color: var(--primary);
    }
    .scripted-row { display: flex; align-items: center; gap: 6px; }
    .scripted-row .initvar-input { flex: 1; }
    .scripted-idx { font-size: 10px; font-weight: 600; color: var(--muted-foreground); min-width: 20px; }
    .initvar-add {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 5px 8px;
      border: 1px dashed var(--border);
      border-radius: 6px;
      background: transparent;
      color: var(--muted-foreground);
      font-size: 11px;
      cursor: pointer;
      margin-top: 4px;
      align-self: flex-start;
    }
    .initvar-add:hover {
      background: var(--muted);
      color: var(--foreground);
      border-color: color-mix(in oklch, var(--primary) 40%, var(--border));
    }

    /* Var bag list */
    .varbag-list {
      display: flex;
      flex-direction: column;
      gap: 3px;
      overflow-y: auto;
      min-height: 0;
    }
    .varbag-row {
      display: grid;
      grid-template-columns: 130px 1fr 58px;
      align-items: center;
      gap: 8px;
      padding: 4px 6px;
      border-radius: 5px;
      font-size: 11px;
      transition: background .12s;
    }
    .varbag-row:hover { background: var(--muted); }
    .varbag-row--just-set {
      background: color-mix(in oklch, var(--primary) 7%, transparent);
      box-shadow: inset 2px 0 0 0 var(--primary);
    }
    .varbag-key {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--foreground);
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .varbag-value {
      font-family: var(--uk-font-monospace, monospace);
      color: var(--muted-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .varbag-source {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 10px;
      color: var(--muted-foreground);
      background: var(--muted);
      padding: 1px 5px;
      border-radius: 3px;
      text-align: center;
    }

    /* ----- Right pane: Var bag / Test cases tabs ----- */
    .run-panel-tabs {
      display: flex;
      align-items: center;
      gap: 4px;
      border-bottom: 1px solid var(--border);
      padding-bottom: 8px;
      margin-bottom: 10px;
    }
    .rp-tab {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      background: transparent;
      border: none;
      padding: 5px 9px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      cursor: pointer;
      transition: background .12s, color .12s;
    }
    .rp-tab:hover { background: var(--muted); color: var(--foreground); }
    .rp-tab--active {
      background: color-mix(in oklch, var(--primary) 12%, transparent);
      color: var(--primary);
    }
    .rp-tab .run-panel-count { font-weight: 700; letter-spacing: 0; }
    .rp-tab-action {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      background: var(--primary);
      color: var(--primary-foreground);
      border: 1px solid var(--primary);
      padding: 5px 10px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 600;
      cursor: pointer;
    }
    .rp-tab-action:hover { filter: brightness(1.05); }

    /* Test case list */
    .testcase-list {
      display: flex;
      flex-direction: column;
      gap: 6px;
      overflow-y: auto;
      min-height: 0;
    }
    .testcase-row {
      display: grid;
      grid-template-columns: 8px 1fr auto auto;
      align-items: center;
      gap: 8px;
      padding: 6px 8px;
      border-radius: 6px;
      background: var(--background);
      border: 1px solid var(--border);
      transition: border-color .12s;
    }
    .testcase-row:hover { border-color: color-mix(in oklch, var(--primary) 35%, var(--border)); }
    .testcase-dot {
      width: 8px; height: 8px; border-radius: 50%;
    }
    .testcase-dot--pass       { background: var(--success); }
    .testcase-dot--fail       { background: var(--destructive); }
    .testcase-dot--never_run  { background: var(--muted-foreground); opacity: .5; }
    .testcase-text { min-width: 0; }
    .testcase-name {
      font-size: 12px;
      font-weight: 600;
      color: var(--foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .testcase-meta {
      font-size: 10px;
      color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .tc-scenario-pill {
      display: inline-flex;
      padding: 1px 5px;
      border-radius: 3px;
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      font-family: inherit;
    }
    .tc-scenario-pill--success { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .tc-scenario-pill--fail    { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }

    .testcase-run, .testcase-del {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 5px 9px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 600;
      cursor: pointer;
      background: var(--card);
      border: 1px solid var(--border);
      color: var(--foreground);
      transition: background .12s, border-color .12s;
    }
    .testcase-run:hover {
      background: var(--primary);
      color: var(--primary-foreground);
      border-color: var(--primary);
    }
    .testcase-del {
      padding: 5px;
      color: var(--muted-foreground);
    }
    .testcase-del:hover {
      background: color-mix(in oklch, var(--destructive) 12%, transparent);
      color: var(--destructive);
      border-color: color-mix(in oklch, var(--destructive) 40%, var(--border));
    }

    .testcase-empty {
      padding: 24px 16px;
      text-align: center;
      color: var(--muted-foreground);
      font-size: 12px;
      line-height: 1.5;
    }
    .testcase-empty uk-icon { opacity: 0.45; }
    .testcase-empty p { margin: 8px 0 0; }

    /* Save-run button in playback toolbar */
    .sim-save-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 12px;
      height: 32px;
      border-radius: 7px;
      background: var(--card);
      border: 1px solid var(--border);
      color: var(--foreground);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      box-shadow: var(--shadow-xs);
      transition: background .12s, border-color .12s;
    }
    .sim-save-btn:hover:not(:disabled) {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 35%, var(--border));
    }
    .sim-save-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* Toast banner inside right pane */
    .rp-toast {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      margin-top: 8px;
      padding: 6px 10px;
      background: color-mix(in oklch, var(--success) 18%, transparent);
      color: var(--success);
      border-radius: 6px;
      font-size: 11px;
      font-weight: 600;
      animation: rpToastIn .25s ease-out;
    }
    @keyframes rpToastIn {
      from { opacity: 0; transform: translateY(4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    /* Now-executing card */
    .sim-runcard {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px;
      box-shadow: var(--shadow-xs);
    }
    .sim-runcard--empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      text-align: center;
      padding: 24px 16px;
      color: var(--muted-foreground);
    }
    .sim-runcard--empty uk-icon { color: color-mix(in oklch, var(--primary) 50%, var(--muted-foreground)); }
    .sim-runcard--empty p { margin: 8px 0 0; font-size: 12px; line-height: 1.5; }
    .sim-runcard--fail {
      border-color: var(--destructive);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--destructive) 18%, transparent);
    }
    /* Paused = warning amber, NOT primary coral. The destructive red ring
       used for --fail is ~3° from primary in hue space, so primary-ringed
       pause was visually identical to a failed step. Warning amber is the
       "human is waiting" semantic and the same tone we use for the
       manual_approval node icon. */
    .sim-runcard--paused {
      border-color: var(--warning);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--warning) 22%, transparent);
    }

    /* Input form for wait_input steps — mirror the paused warning tone. */
    .input-form {
      margin-top: 10px;
      padding: 10px;
      background: color-mix(in oklch, var(--warning) 10%, var(--card));
      border: 1px solid color-mix(in oklch, var(--warning) 35%, var(--border));
      border-radius: 8px;
    }
    /* The bottom step card's SINGLE input for a capture node (panel-down input —
       user decision: keep the node clean, put the input here). */
    .sim-input-control {
      margin-top: 10px;
      padding: 10px;
      background: color-mix(in oklch, var(--primary) 5%, var(--card));
      border: 1px solid color-mix(in oklch, var(--primary) 25%, var(--border));
      border-radius: 8px;
    }
    .sim-input-label {
      display: block;
      font-size: 11px;
      font-weight: 600;
      color: var(--muted-foreground);
      margin-bottom: 5px;
    }
    .sim-input-field {
      width: 100%;
      box-sizing: border-box;
      font-size: 13px;
      font-family: var(--uk-font-monospace, monospace);
      padding: 6px 8px;
      border: 1px solid var(--primary);
      border-radius: 6px;
      background: var(--card);
      color: var(--foreground);
    }
    .sim-input-field:focus { outline: none; box-shadow: 0 0 0 2px color-mix(in oklch, var(--primary) 30%, transparent); }
    .sim-input-row { display: flex; gap: 8px; align-items: stretch; }
    .sim-input-row .sim-input-field { flex: 1; }
    .sim-input-submit {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      white-space: nowrap;
      padding: 0 12px;
      border: none;
      border-radius: 6px;
      background: var(--primary);
      color: var(--primary-foreground);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
    }
    .sim-input-submit:hover:not(:disabled) { background: color-mix(in oklch, var(--primary) 88%, black); }
    .sim-input-submit:disabled { opacity: 0.5; cursor: not-allowed; }
    .sim-input-branch { margin-top: 6px; font-size: 12px; color: var(--muted-foreground); }
    .branch-tag {
      display: inline-block;
      padding: 0 6px;
      border-radius: 4px;
      font-weight: 600;
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
    }
    .branch-tag--captured { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .branch-tag--timeout  { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    .input-form-prompt {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      font-size: 12px;
      color: var(--foreground);
      font-weight: 500;
      margin-bottom: 8px;
      line-height: 1.4;
    }
    .input-form-prompt uk-icon {
      color: var(--warning);
      flex-shrink: 0;
      margin-top: 2px;
    }
    .input-form-input {
      width: 100%;
      box-sizing: border-box;
      padding: 7px 10px;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: var(--background);
      color: var(--foreground);
      font-family: var(--uk-font-monospace, monospace);
      font-size: 13px;
    }
    .input-form-input:focus {
      outline: none;
      border-color: var(--warning);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--warning) 28%, transparent);
    }

    .input-form-choices {
      display: flex;
      gap: 6px;
      flex-wrap: wrap;
    }
    .choice-pill {
      padding: 6px 12px;
      border-radius: 6px;
      background: var(--background);
      border: 1px solid var(--border);
      color: var(--foreground);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      transition: background .12s, border-color .12s;
    }
    .choice-pill:hover {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 35%, var(--border));
    }
    .choice-pill--active {
      background: var(--primary);
      color: var(--primary-foreground);
      border-color: var(--primary);
    }

    .input-form-actions {
      display: flex;
      align-items: center;
      gap: 10px;
      margin-top: 8px;
    }
    .input-form-hint {
      font-size: 11px;
      color: var(--muted-foreground);
      font-style: italic;
      flex: 1;
    }
    .input-form-submit {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 14px;
      height: 32px;
      border-radius: 7px;
      background: var(--primary);
      color: var(--primary-foreground);
      border: 1px solid var(--primary);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      transition: filter .12s;
    }
    .input-form-submit:hover:not(:disabled) { filter: brightness(1.05); }
    .input-form-submit:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
    .sim-runcard-head {
      display: grid;
      grid-template-columns: auto 1fr auto;
      align-items: center;
      gap: 10px;
    }
    .sim-runcard-status {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 3px 8px;
      border-radius: 9999px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .sim-runcard-status--ok      { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .sim-runcard-status--fail    { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }
    .sim-runcard-status--timeout { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    .sim-runcard-status--skipped { background: var(--muted); color: var(--muted-foreground); }

    .sim-runcard-error {
      margin-top: 10px;
      padding: 10px;
      background: color-mix(in oklch, var(--destructive) 7%, transparent);
      border-radius: 6px;
      font-size: 12px;
      color: var(--foreground);
      line-height: 1.45;
    }
    .sim-runcard-fix-row {
      display: flex;
      gap: 6px;
      margin-top: 10px;
    }
    .sim-runcard-fix-row .fix-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 12px;
      height: 32px;
      border-radius: 7px;
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      background: var(--card);
      border: 1px solid var(--border);
      color: var(--foreground);
      transition: background .12s, border-color .12s;
    }
    .sim-runcard-fix-row .fix-btn:hover {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--destructive) 30%, var(--border));
    }
    .sim-runcard-fix-row .fix-btn--retry {
      background: var(--destructive);
      color: var(--destructive-foreground);
      border-color: var(--destructive);
    }
    .sim-runcard-fix-row .fix-btn--retry:hover {
      background: var(--destructive);
      color: var(--destructive-foreground);
      filter: brightness(1.05);
    }

    /* JSON highlight used by sim inspector (mirrors trace-viewer styles) */
    .json-block {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      line-height: 1.55;
      background: var(--muted);
      color: var(--foreground);
      border: 1px solid var(--border);
      padding: 8px 10px;
      border-radius: 6px;
      white-space: pre;
      overflow-x: auto;
      margin: 0;
    }
    .json-key      { color: var(--primary); font-weight: 500; }
    .json-string   { color: var(--success); }
    .json-number   { color: var(--warning); }
    .json-bool     { color: var(--destructive); }
    .json-null     { color: var(--muted-foreground); font-style: italic; }

    .step-tag {
      display: inline-flex;
      padding: 1px 6px;
      border-radius: 4px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .step-tag--ok      { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .step-tag--fail    { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }
    .step-tag--timeout { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    .step-tag--skipped { background: var(--muted); color: var(--muted-foreground); }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'flow-id' }) accessor flowId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  @state() private accessor _selectedNodeId: string | null = null;
  // Copy/paste clipboard: a node's content snapshot (no id/edges) + a paste
  // counter so repeated pastes cascade instead of stacking.
  private _clipboardNode: Omit<FlowNode, 'id'> | null = null;
  private _pasteCount = 0;

  // Palette filter + per-group collapse. A non-empty query force-expands every
  // matching group (collapse state is ignored while searching) so a hit is
  // never hidden behind a folded header.
  @state() private accessor _paletteQuery = '';
  @state() private accessor _collapsedGroups: Set<string> = new Set();

  // Live graph state. Loaded from the flow's opaque `graph` JSONB (GET),
  // mutated by drag, persisted by Save (PATCH). Deep-copied on load so
  // inspector edits never alias the loaded response.
  @state() private accessor _nodes: FlowNode[] = [];
  @state() private accessor _edges: FlowEdge[] = [];
  @state() private accessor _draggedId: string | null = null;
  private _drag: { id: string; startX: number; startY: number; offX: number; offY: number } | null = null;
  private _svgRef: SVGSVGElement | null = null;
  private _pendingClick: { id: string } | null = null;

  // --- Simulator state ---
  @state() private accessor _simMode: 'edit' | 'sim' = 'edit';
  // Index into the active trace. -1 = "ready to run, no step executed yet".
  @state() private accessor _simStep: number = -1;
  @state() private accessor _simScenario: 'success' | 'fail' = 'success';
  // Real trace from POST /simulate. When set, it drives sim playback instead of
  // the mock scenarios.
  @state() private accessor _liveTrace: TraceStep[] | null = null;
  @state() private accessor _simRunning = false;
  @state() private accessor _initVars: InitVar[] = MOCK_INIT_VARS.map(v => ({ ...v }));
  // Per-node scripted reservation result (sim branch control): node id → port.
  // Set by selecting a reservation node in sim and picking its outcome.
  @state() private accessor _simNodeOutcomes: Record<string, 'accepted' | 'timeout' | 'no_candidate'> = {};
  // 3D-4: per-input-node captured value pinned for the simulator (node id →
  // value). Sent as scripted_effect_outputs; empty → that node times out.
  @state() private accessor _simNodeInputs: Record<string, string> = {};
  // Input draft for the wait_input form. Resets each time the sim lands on a
  // wait_input step. Pre-filled with the "expected" value from the trace so
  // the demo can be stepped through without typing each time.
  @state() private accessor _inputDraft: string = '';
  // Captured wait_input choices keyed by step id. Populated by _submitInput
  // (or by replaying a TestCase). On Save, these go into the new TestCase.
  @state() private accessor _capturedInputs: Record<string, string> = {};
  // Right column toggle — show running variable bag OR list of saved tests.
  @state() private accessor _rightTab: 'varbag' | 'tests' = 'varbag';
  // Saved test cases. Seeded from MOCK so the demo has 3 entries to show.
  @state() private accessor _testCases: TestCase[] = MOCK_TEST_CASES.map(t => ({
    ...t,
    init_var_overrides: { ...t.init_var_overrides },
    captured_inputs: { ...t.captured_inputs },
  }));
  // Banner shown briefly after saving a test case (demo only).
  @state() private accessor _saveToast: string | null = null;

  // Loaded draft (null while pending, errored, or in create mode). `_flow`
  // derives the header (name/code/version/enabled) from it.
  @state() private accessor _loaded: Flow | null = null;
  @state() private accessor _saving = false;
  // Transient action feedback: Save result, the 501 "lands in Layer 3" notice,
  // and PATCH conflict/error surfacing.
  // Latest validation result (from Validate, or a 422 publish). null until the
  // user validates; cleared when the graph changes so stale issues don't linger.
  @state() private accessor _validation: FlowValidationResult | null = null;
  // Publish popover: channel + entry-code binding target.
  @state() private accessor _publishOpen = false;
  @state() private accessor _publishChannel = 'voice';
  @state() private accessor _publishEntry = 'main';
  @state() private accessor _publishing = false;

  // Collapsible side panes (small screens). Each collapses to a thin rail.
  @state() private accessor _paletteCollapsed = false;
  @state() private accessor _inspectorCollapsed = false;

  // Viewport pan/zoom (react-flow style). World coords are unchanged; only the
  // <g> transform moves. Default pan gives the origin some margin (no left wall).
  @state() private accessor _zoom = 1;
  @state() private accessor _panX = 48;
  @state() private accessor _panY = 48;
  private _panState: { startX: number; startY: number; ox: number; oy: number } | null = null;
  private _viewportRef: SVGGElement | null = null;

  // In-progress edge connection (port drag). Cursor is in world coords.
  @state() private accessor _edgeDraft: { fromId: string; fromPort: string; portKind: string; cx: number; cy: number } | null = null;
  // Node id currently under the connect cursor (drop-target highlight).
  @state() private accessor _edgeDraftTarget: string | null = null;
  // True when the hovered drop target would be a region-boundary error — paints
  // the target red and the drop is rejected.
  @state() private accessor _edgeDraftInvalid = false;
  // While dragging a palette node over a control node / body frame: which owner
  // + region would receive it, so the canvas highlights the drop zone.
  @state() private accessor _dropHoverOwner: string | null = null;
  @state() private accessor _dropHoverRegion: string | null = null;
  // Selected edge (click to select, Delete to remove).
  @state() private accessor _selectedEdgeId: string | null = null;
  // Node ids whose condition field is in raw "advanced" mode.
  @state() private accessor _exprAdvanced: Set<string> = new Set();
  // Condition-DSL function catalog (fetched once from /v1/meta/expr-functions).
  @state() private accessor _exprCatalog: ExprFunction[] = [];
  private _catalogFetched = false;
  // Which node's Advanced editor has the function picker open.
  @state() private accessor _exprPickerNode: string | null = null;
  // Ephemeral Visual-builder group per node id (derived from the stored DSL).
  @state() private accessor _condGroups: Map<string, Group> = new Map();
  // Catalog reference lists for the inspector's searchable code pickers.
  @state() private accessor _catalogRefs: Record<'skill' | 'queue' | 'adapter', CatalogRef[]> = { skill: [], queue: [], adapter: [] };
  private _catalogRefsFetched = false;
  // Open catalog picker keyed by `${nodeId}:${fieldKey}`, + its search text.
  @state() private accessor _catalogPicker: string | null = null;
  @state() private accessor _catalogQuery = '';

  @state() private accessor _actionToast: string | null = null;
  @state() private accessor _actionTone: 'ok' | 'warn' | 'error' = 'ok';
  // Create-mode draft fields — used only when flowId is empty (first Save POSTs).
  @state() private accessor _codeDraft = '';
  @state() private accessor _nameDraft = '';

  private get _isCreate(): boolean {
    return !this.flowId;
  }

  // Loads the draft graph. Skips the fetch in create mode (no id yet). On
  // success it hydrates `_nodes`/`_edges`/`_loaded` and returns the flow so the
  // render branch can distinguish pending/error/ready.
  private _loadTask = new Task(this, {
    task: async ([client, orgId, flowId], { signal }) => {
      if (!flowId || !client) {
        this._nodes = [];
        this._edges = [];
        this._loaded = null;
        this._resetHistory();
        return null;
      }
      const { data, error } = await (client as ApiClient).GET('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: orgId as string, id: flowId as string } },
        signal,
      } as never);
      if (error) throw error;
      if (signal.aborted) return null;
      const flow = data as Flow;
      // graph is opaque JSONB — guard against drift (missing or non-array
      // nodes/edges) so the canvas .map()/.find() can't crash on render.
      const graph = (flow.graph ?? {}) as { nodes?: unknown; edges?: unknown };
      const nodes = (Array.isArray(graph.nodes) ? structuredClone(graph.nodes) : []) as FlowNode[];
      // Normalize the backend/runtime shape (type/config) to the UI shape
      // (kind/params) — a graph authored elsewhere (or via the API) only carries
      // type/config, and reading node.kind undefined would crash the render.
      for (const n of nodes as Array<FlowNode & { type?: FlowNodeKind; config?: FlowNode['params'] }>) {
        if (n.kind === undefined && n.type !== undefined) n.kind = n.type;
        if (n.params === undefined && n.config !== undefined) n.params = n.config;
        // ALWAYS derive runtime ports — stale outputs (yes/no, case_1) would let
        // the user drag a port the runtime never emits (codex HIGH).
        if (RUNTIME_KINDS.has(n.kind)) n.outputs = outputsForNode(n);
      }
      const rawEdges = (Array.isArray(graph.edges) ? structuredClone(graph.edges) : []) as FlowEdge[];
      // A graph authored via the API/elsewhere may carry edges with no (or
      // duplicate) `id`. Without a stable unique id, selecting one edge sets
      // _selectedEdgeId to undefined and the render's `e.id === _selectedEdgeId`
      // is `undefined === undefined` → TRUE for every edge, so all edges paint
      // selected (coral). Assign a unique id to any edge missing one.
      const seenEdgeIds = new Set<string>();
      for (const e of rawEdges) {
        if (e.from_port === undefined && e.label !== undefined) e.from_port = e.label;
        if (!e.id || seenEdgeIds.has(e.id)) {
          let nid: string;
          do { nid = 'e_' + Math.random().toString(36).slice(2, 8); }
          while (seenEdgeIds.has(nid) || rawEdges.some(x => x.id === nid));
          e.id = nid;
        }
        seenEdgeIds.add(e.id);
      }
      // Backend-authored graphs carry no x/y — lay them out so they don't
      // render at NaN coordinates (Playwright caught this on an API-made flow).
      if (nodes.some(n => !Number.isFinite(n.x) || !Number.isFinite(n.y))) {
        const pos = new Map(autoArrange(nodes, rawEdges).map(p => [p.id, p]));
        for (const n of nodes) {
          const p = pos.get(n.id);
          if (p) { n.x = p.x; n.y = p.y; }
        }
      }
      this._nodes = nodes;
      this._edges = this._pruneEdges(nodes, rawEdges);
      this._recomputeRegions(); // derive membership from the loaded graph (self-consistent)
      this._resetHistory();
      this._loaded = flow;
      this._dirty = false;
      this._selectedNodeId = this._nodes[0]?.id ?? null;
      return flow;
    },
    args: () => [this.client, this.orgId, this.flowId] as const,
  });

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    document.addEventListener('keydown', this._onKeyDown);
  }

  private _wheelBound = false;
  private _fittedOnLoad = false;
  override updated(): void {
    // The canvas SVG only exists once the flow has loaded (the first render is
    // the pending/error state), so bind wheel here, not in firstUpdated — and
    // non-passive so we can preventDefault the page scroll while zooming.
    if (!this._catalogFetched && this.client) {
      this._catalogFetched = true;
      void this._fetchExprCatalog();
    }
    if (!this._catalogRefsFetched && this.client && this.orgId) {
      this._catalogRefsFetched = true;
      void this._fetchCatalogRefs();
    }
    const svg = this._svgEl();
    if (svg && !this._wheelBound) {
      svg.addEventListener('wheel', this._onWheel, { passive: false });
      this._wheelBound = true;
    }
    // Frame the graph once the flow has loaded — otherwise a reload leaves the
    // viewport at its default pan and the saved nodes can sit off-screen.
    if (svg && !this._fittedOnLoad && this._loaded && this._nodes.length > 0) {
      this._fittedOnLoad = true;
      this._fitView();
    }
  }

  private async _fetchExprCatalog(): Promise<void> {
    try {
      const { data } = (await this.client.GET('/v1/meta/expr-functions' as never, {} as never)) as
        { data?: { functions?: ExprFunction[] } };
      this._exprCatalog = data?.functions ?? [];
    } catch {
      this._exprCatalog = []; // picker just stays empty if the fetch fails
    }
  }

  // Catalog reference lists for the inspector's searchable code pickers (skill /
  // queue / adapter) — so a user PICKS an existing code instead of typing one
  // that won't match (and fails validation).
  private async _fetchCatalogRefs(): Promise<void> {
    const load = async (path: string): Promise<CatalogRef[]> => {
      try {
        // limit is capped at 100 by the contract — 200 is rejected (400), which
        // would silently empty the picker. Fetch the max page; client-side search
        // filters within it.
        const { data, error } = (await this.client.GET(path as never, {
          params: { path: { org_id: this.orgId }, query: { limit: 100, include_disabled: false } },
        } as never)) as { data?: { items?: Array<{ code: string; name?: string }> }; error?: unknown };
        if (error) return [];
        return (data?.items ?? []).map(i => ({ code: i.code, name: i.name ?? i.code }));
      } catch {
        return [];
      }
    };
    const [skill, queue, adapter] = await Promise.all([
      load('/v1/orgs/{org_id}/skills'),
      load('/v1/orgs/{org_id}/queues'),
      load('/v1/orgs/{org_id}/adapters'),
    ]);
    this._catalogRefs = { skill, queue, adapter };
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._svgRef?.removeEventListener('wheel', this._onWheel);
    document.removeEventListener('keydown', this._onKeyDown);
    clearTimeout(this._actionToastTimer);
    clearTimeout(this._saveToastTimer);
  }

  private _actionToastTimer?: ReturnType<typeof setTimeout>;
  private _flashAction(message: string, tone: 'ok' | 'warn' | 'error' = 'ok'): void {
    this._actionToast = message;
    this._actionTone = tone;
    clearTimeout(this._actionToastTimer);
    this._actionToastTimer = setTimeout(() => { this._actionToast = null; }, 3200);
  }

  private _saveToastTimer?: ReturnType<typeof setTimeout>;
  private _flashSaveToast(message: string): void {
    this._saveToast = message;
    clearTimeout(this._saveToastTimer);
    this._saveToastTimer = setTimeout(() => { this._saveToast = null; }, 2400);
  }

  // PATCH the graph (or POST a new draft in create mode). The only write that
  // is real in Layer 2 — version is sent for optimistic concurrency; 409 is
  // surfaced as a conflict rather than silently refreshed.
  private async _saveDraft(): Promise<void> {
    if (this._saving) return;
    this._saving = true;
    try {
      const graph = this._serializeGraph();
      if (this._isCreate) {
        const code = this._codeDraft.trim();
        const name = this._nameDraft.trim();
        if (!code || !name) {
          this._flashAction('Code and name are required to create a flow.', 'error');
          return;
        }
        const { data, error } = await this.client.POST('/v1/orgs/{org_id}/flows' as never, {
          params: { path: { org_id: this.orgId } },
          body: { code, name, graph, enabled: true },
        } as never);
        if (error) {
          this._flashAction(this._errText(error, 'Could not create flow.'), 'error');
          return;
        }
        const created = data as Flow;
        this._flashAction(`Created "${created.name}".`, 'ok');
        this._navigate(`/orgs/${this.orgId}/flows/${created.id}`);
        return;
      }
      const current = this._loaded;
      if (!current) return;
      const res = (await this.client.PATCH('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: this.orgId, id: current.id } },
        body: { graph, version: current.version },
      } as never)) as { data?: unknown; error?: unknown; response?: { status?: number } };
      if (res.error) {
        // openapi-fetch puts the HTTP status on `response`, not on the parsed
        // error body — check response.status for the 409 optimistic-lock case.
        this._flashAction(
          res.response?.status === 409
            ? 'Version conflict — this draft changed elsewhere. Reload before saving.'
            : this._errText(res.error, 'Save failed.'),
          'error',
        );
        return;
      }
      this._loaded = res.data as Flow;
      this._validation = null;
      this._dirty = false; // in sync with the server now
      this._flashAction(`Saved draft · v${(res.data as Flow).version}.`, 'ok');
    } finally {
      this._saving = false;
    }
  }

  private _relTime(iso: string): string {
    const t = Date.parse(iso);
    if (Number.isNaN(t)) return '—';
    const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
    if (sec < 45) return 'just now';
    const min = Math.round(sec / 60);
    if (min < 60) return `${min}m ago`;
    const hr = Math.round(min / 60);
    if (hr < 24) return `${hr}h ago`;
    return `${Math.round(hr / 24)}d ago`;
  }

  private _errText(error: unknown, fallback: string): string {
    const r = (error as { reason?: string; message?: string }) ?? {};
    return r.reason ?? r.message ?? fallback;
  }

  // Validate the saved draft against the real backend (POST /validate) and show
  // the issues. Always a 200 with {valid, issues}; 404 only if the draft is gone.
  private async _validateFlow(): Promise<void> {
    if (this._isCreate || !this._loaded) {
      this._flashAction('Save the draft first.', 'warn');
      return;
    }
    // Validate runs server-side on the saved graph — flush pending edits first.
    if (this._dirty) {
      await this._saveDraft();
      if (this._dirty) return; // save failed (conflict/error already surfaced)
    }
    const res = (await this.client.POST('/v1/orgs/{org_id}/flows/{id}/validate' as never, {
      params: { path: { org_id: this.orgId, id: this._loaded.id } },
    } as never)) as { data?: FlowValidationResult; error?: unknown; response?: { status?: number } };
    if (res.error || !res.data) {
      this._flashAction(
        res.response?.status === 404 ? 'Flow not found — reload.' : this._errText(res.error, 'Validate failed.'),
        'error',
      );
      return;
    }
    this._validation = res.data;
    this._flashAction(
      res.data.valid ? 'Valid — no issues.' : `${res.data.issues.length} issue(s) found.`,
      res.data.valid ? 'ok' : 'warn',
    );
  }

  // Publish the draft for the (channel, entry_code) binding. 201 succeeds;
  // 422 returns the validation issues (publish requires a clean graph); 409 is
  // an optimistic conflict (draft or binding changed) — surfaced, not faked.
  private async _publishFlow(): Promise<void> {
    if (this._isCreate || !this._loaded || this._publishing) {
      if (!this._loaded) this._flashAction('Save the draft first.', 'warn');
      return;
    }
    const channel = this._publishChannel.trim();
    const entryCode = this._publishEntry.trim();
    if (!channel || !entryCode) {
      this._flashAction('Channel and entry code are required.', 'error');
      return;
    }
    // Publish compiles the saved graph — flush pending edits first.
    if (this._dirty) {
      await this._saveDraft();
      if (this._dirty) return;
    }
    this._publishing = true;
    try {
      const res = (await this.client.POST('/v1/orgs/{org_id}/flows/{id}/publish' as never, {
        params: { path: { org_id: this.orgId, id: this._loaded.id } },
        body: { channel, entry_code: entryCode, version: this._loaded.version },
      } as never)) as { data?: unknown; error?: unknown; response?: { status?: number } };

      const status = res.response?.status;
      if (status === 422) {
        // openapi-fetch puts the non-2xx body on `error`; it's a FlowValidationResult.
        const vr = res.error as FlowValidationResult;
        this._validation = vr?.issues ? vr : { valid: false, issues: [] };
        this._flashAction(`Publish blocked — ${this._validation.issues.length} issue(s).`, 'warn');
        return;
      }
      if (status === 409) {
        this._flashAction('Publish conflict — draft or binding changed elsewhere. Reload.', 'error');
        return;
      }
      if (res.error || !res.data) {
        this._flashAction(this._errText(res.error, 'Publish failed.'), 'error');
        return;
      }
      const version = (res.data as { version?: { version_number?: number } })?.version?.version_number;
      this._validation = { valid: true, issues: [] };
      this._publishOpen = false;
      this._flashAction(`Published v${version ?? '?'} → ${channel}/${entryCode}.`, 'ok');
    } finally {
      this._publishing = false;
    }
  }

  // Node ids carrying a validation issue, for canvas highlighting.
  private get _issueNodeIds(): Set<string> {
    const s = new Set<string>();
    for (const i of this._validation?.issues ?? []) {
      if (i.node_id) s.add(i.node_id);
    }
    return s;
  }

  // Edge ids carrying a validation issue (e.g. region_boundary_crossing) so the
  // canvas can paint the offending edge red — the panel lists ids the user can't
  // otherwise find.
  private get _issueEdgeIds(): Set<string> {
    const s = new Set<string>();
    for (const i of this._validation?.issues ?? []) {
      if (i.edge_id) s.add(i.edge_id);
    }
    return s;
  }

  private get _activeTrace(): TraceStep[] {
    if (this._liveTrace) return this._liveTrace;
    return this._simScenario === 'fail' ? MOCK_TRACE_STEPS_FAIL : MOCK_TRACE_STEPS;
  }

  private get _currentStep(): TraceStep | null {
    if (this._simStep < 0) return null;
    return this._activeTrace[this._simStep] ?? null;
  }

  // True when sim has landed on a wait_input step that hasn't yet been
  // submitted. We use this to disable the toolbar's Step-forward button so
  // Submit is the only way to advance — otherwise users can silently skip
  // the input by clicking the chevron, defeating the "engine paused
  // waiting for value" model.
  // The on-node value field is the single input place now, so we never hard-pause
  // the playback for a separate bottom submit (user report: one input, not two).
  private get _pausedForInput(): boolean {
    return false;
  }

  private get _hitNodeIds(): Set<string> {
    const ids = new Set<string>();
    if (this._simStep < 0) return ids;
    for (let i = 0; i <= this._simStep && i < this._activeTrace.length; i++) {
      ids.add(this._activeTrace[i]!.node_id);
    }
    return ids;
  }

  private _stepBy(delta: number): void {
    const max = this._activeTrace.length - 1;
    this._simStep = Math.max(-1, Math.min(max, this._simStep + delta));
    const step = this._activeTrace[this._simStep];
    if (step) this._selectedNodeId = step.node_id;
    this._refreshInputDraft();
  }

  // When the sim lands on a wait_input step, prefill the input field with
  // the trace's expected captured value so demo-stepping just works.
  private _refreshInputDraft(): void {
    const step = this._currentStep;
    if (!step || !WAIT_INPUT_KINDS.has(step.node_kind)) {
      this._inputDraft = '';
      return;
    }
    // Prefill the bottom input with the value pinned for this node (so the field
    // reflects what you submitted), falling back to the trace's captured value.
    const pinned = this._simNodeInputs[step.node_id];
    if (pinned !== undefined) {
      this._inputDraft = pinned;
      return;
    }
    const out = step.outputs;
    const v = out['captured'] ?? out['menu_choice'] ?? out['text'] ?? out['signal_payload'] ??
      out['supervisor_choice'] ?? out['captured_value'] ?? '';
    this._inputDraft = String(v);
  }

  // Apply the bottom-panel input value for a capture node, re-run, then ADVANCE
  // to the next step — Submit alone moves the sim forward, no separate Next click
  // (user report).
  private async _submitNodeInput(nodeId: string): Promise<void> {
    await this._setNodeInput(nodeId, this._inputDraft); // re-run + land on this node
    this._stepBy(1); // Submit advances past the input node
  }

  private _submitInput(): void {
    // Capture the submitted value keyed by step id so the run can be saved
    // as a test case later. Real engine would also push _inputDraft into
    // the variable bag under the step's `var` param.
    const step = this._currentStep;
    if (step) {
      this._capturedInputs = { ...this._capturedInputs, [step.id]: this._inputDraft };
    }
    this._stepBy(1);
  }

  // ----- Test case actions -----

  private _saveCurrentAsTestCase(): void {
    // Placeholder UI — v0.2 wire-up should replace window.prompt with an
    // inline name input + form (modal blocks the JS thread + can't be
    // styled). Flagged in code review SHOULD; intentional for preview.
    const name = window.prompt('Name this test case:', `Replay ${new Date().toISOString().slice(11, 19)}`);
    if (!name) return;
    const overrides: Record<string, string> = {};
    for (const v of this._initVars) {
      if (v.source === 'user') overrides[v.key] = v.value;
    }
    const tc: TestCase = {
      id: 'tc_' + Math.random().toString(36).slice(2, 12),
      name,
      description: `Captured at step ${this._simStep + 1} / ${this._activeTrace.length}, scenario ${this._simScenario}`,
      scenario: this._simScenario,
      init_var_overrides: overrides,
      captured_inputs: { ...this._capturedInputs },
      created_at: new Date().toISOString(),
      last_outcome: 'never_run',
    };
    this._testCases = [tc, ...this._testCases];
    this._rightTab = 'tests';
    this._flashSaveToast(`Saved "${name}"`);
  }

  private _runTestCase(tc: TestCase): void {
    // Replay: switch scenario, apply init_var overrides, restore captured
    // inputs, then jump to the end of the trace so the user sees the
    // outcome. (Real engine would actually re-execute and diff.)
    this._simScenario = tc.scenario;
    this._capturedInputs = { ...tc.captured_inputs };
    // Overrides for keys not present in the current _initVars are silently
    // dropped — the v0.2 engine will need to re-introduce missing keys
    // (e.g. if the trigger schema added a new var since the case was saved).
    this._initVars = this._initVars.map(v =>
      tc.init_var_overrides[v.key] !== undefined
        ? { ...v, value: tc.init_var_overrides[v.key]! }
        : v
    );
    // Jump to end so user sees final state. A nicer demo would animate
    // step-by-step replay.
    this._simStep = this._activeTrace.length - 1;
    const last = this._activeTrace[this._simStep];
    if (last) this._selectedNodeId = last.node_id;
    this._refreshInputDraft();
    // Mark the case as just-passed (demo simplification: success ⇒ pass,
    // fail ⇒ fail). Mutate-don't-mutate: replace the entry.
    this._testCases = this._testCases.map(c =>
      c.id === tc.id ? { ...c, last_outcome: tc.scenario === 'success' ? 'pass' : 'fail' } : c
    );
    this._flashSaveToast(`Replayed "${tc.name}"`);
  }

  private _deleteTestCase(id: string): void {
    this._testCases = this._testCases.filter(c => c.id !== id);
  }

  private _restartSim(): void {
    this._simStep = -1;
    this._selectedNodeId = null;
    this._inputDraft = '';
    this._capturedInputs = {};
  }

  private _runAllSim(): void {
    // Instant — playback animation deferred; this just advances to end.
    this._simStep = this._activeTrace.length - 1;
    const last = this._activeTrace[this._simStep];
    if (last) this._selectedNodeId = last.node_id;
    this._refreshInputDraft();
  }

  private _toggleSimMode(): void {
    if (this._simMode === 'sim') {
      this._simMode = 'edit';
      this._liveTrace = null;
      this._restartSim();
    } else {
      void this._runSimulation();
    }
  }

  // Save the draft, run a real deterministic simulation (POST /simulate), and
  // play its trace back over the canvas. 422 surfaces the validation issues.
  private async _runSimulation(landAtEnd = false): Promise<void> {
    if (this._isCreate || !this._loaded) {
      this._flashAction('Save the draft first.', 'warn');
      return;
    }
    if (this._dirty) {
      await this._saveDraft();
      if (this._dirty) return; // save failed — surfaced
    }
    this._simRunning = true;
    try {
      const scripted = Object.entries(this._simNodeOutcomes).map(([node_id, outcome]) => ({ node_id, outcome }));
      const inputs = Object.fromEntries(Object.entries(this._simNodeInputs).filter(([, v]) => v !== ''));
      const res = (await this.client.POST('/v1/orgs/{org_id}/flows/{id}/simulate' as never, {
        params: { path: { org_id: this.orgId, id: this._loaded.id } },
        body: {
          interaction_input: this._simInteractionInput(),
          scripted_reservation_outcomes: scripted.length ? scripted : undefined,
          scripted_effect_outputs: Object.keys(inputs).length ? inputs : undefined,
        },
      } as never)) as {
        data?: components['schemas']['SimulateFlowResponse'];
        error?: unknown;
        response?: { status?: number };
      };
      if (res.error || !res.data) {
        if (res.response?.status === 422) {
          this._flashAction('Graph is invalid — fix the issues, then simulate.', 'error');
        } else {
          this._flashAction(this._errText(res.error, 'Simulate failed.'), 'error');
        }
        return;
      }
      this._liveTrace = this._mapApiTrace(res.data.trace.steps);
      this._simMode = 'sim';
      if (landAtEnd) {
        // Editing an input value re-runs the whole sim; don't yank the user back
        // to "Ready to run" — land on the final step so they see the new result
        // (which branch the value took) without re-stepping (user report).
        this._selectedNodeId = null;
        this._inputDraft = '';
        this._capturedInputs = {};
        this._simStep = Math.max(0, this._liveTrace.length - 1);
      } else {
        this._restartSim();
      }
      const outcome = res.data.trace.outcome ?? 'completed';
      this._flashAction(`Simulation ${outcome} — ${this._liveTrace.length} step(s).`, outcome === 'failed' ? 'warn' : 'ok');
    } finally {
      this._simRunning = false;
    }
  }

  // Pin (or clear) a reservation node's simulated branch and re-run. Driven by
  // the clickable port chips on the node itself — the per-node sim control.
  private async _toggleNodeOutcome(nodeId: string, outcome: string): Promise<void> {
    const next = { ...this._simNodeOutcomes };
    if (next[nodeId] === outcome) delete next[nodeId];
    else next[nodeId] = outcome as 'accepted' | 'timeout' | 'no_candidate';
    this._simNodeOutcomes = next;
    await this._runSimulation(true); // land on the result, don't restart playback
    // _restartSim (inside _runSimulation) nulls the selection; restore it so the
    // sidebar keeps showing this reservation node's outcome status.
    this._selectedNodeId = nodeId;
  }

  // Sidebar companion to the on-node port chips: a slim status line (NOT a
  // sidebar-width select — that overflowed). The chips on the node are the
  // control; this just reflects/clears the current pin.
  private _renderSimOutcomePicker() {
    const id = this._selectedNodeId;
    const node = id ? this._nodes.find(n => n.id === id) : undefined;
    if (!node || node.kind !== 'reservation') return nothing;
    const cur = this._simNodeOutcomes[node.id];
    return html`
      <div class="form-section sim-outcome">
        <label><span>Reservation outcome</span></label>
        ${cur
          ? html`<p class="sim-outcome-hint">
              Forcing <strong class=${'node-card-port node-card-port--' + this._outcomeKind(cur)}>${cur}</strong>.
              <button class="linkish" @click=${() => this._toggleNodeOutcome(node.id, cur)}>Use candidates instead</button>
            </p>`
          : html`<p class="sim-outcome-hint">
              Routing by candidate pool. Click a port chip on the node
              (<strong>accepted</strong> / <strong>timeout</strong> / <strong>no_candidate</strong>) to force a branch.
            </p>`}
      </div>`;
  }

  private _outcomeKind(o: string): string {
    return o === 'accepted' ? 'success' : o === 'timeout' ? 'timeout' : 'error';
  }

  // The interactive-input kinds — the simulator lets you pin a captured value
  // for these so the run takes the "got a value" branch instead of timing out.
  private static readonly _INPUT_KINDS = new Set<FlowNodeKind>([
    'get_dtmf', 'prompt_text', 'wait_signal', 'manual_approval', 'detect_speech', 'csat_survey', 'nps_survey',
  ]);

  // The placeholder for an input/capture node's simulated value — shown both in
  // the sidebar picker and the on-node field. The VALUE drives the branch
  // (captured/received/approved/recognized vs timeout); empty ⇒ the node times
  // out. if_else/switch_case are deterministic and never appear here.
  private static _simInputHint(kind: FlowNodeKind): string {
    switch (kind) {
      case 'manual_approval': return 'approved / rejected';
      case 'csat_survey': return '1–5';
      case 'nps_survey': return '0–10';
      case 'get_dtmf': return 'e.g. 1234';
      case 'detect_speech': return 'spoken text';
      case 'wait_signal': return 'signal';
      default: return 'captured value';
    }
  }

  private async _setNodeInput(nodeId: string, value: string): Promise<void> {
    const next = { ...this._simNodeInputs };
    if (value === '') delete next[nodeId];
    else next[nodeId] = value;
    this._simNodeInputs = next;
    await this._runSimulation(true); // re-run without restarting to "Ready"
    // Land back on THIS input node's step so its value control stays visible and
    // you can see the branch it now takes (panel-down input — user decision).
    const idx = this._activeTrace.findIndex(s => s.node_id === nodeId);
    if (idx >= 0) this._simStep = idx;
    this._selectedNodeId = nodeId;
  }

  // Build the simulation's interaction_input (the var bag) from the editable
  // init vars, coercing each by its declared type. Dotted keys (customer.tier)
  // are passed flat — the backend's expr lookup tries the flat dotted key first.
  private _simInteractionInput(): Record<string, unknown> {
    const out: Record<string, unknown> = {};
    for (const v of this._initVars) {
      const s = v.value;
      switch (v.type) {
        case 'number': {
          const n = Number(s);
          out[v.key] = Number.isFinite(n) ? n : s;
          break;
        }
        case 'boolean':
          out[v.key] = s === 'true';
          break;
        case 'json':
          try { out[v.key] = JSON.parse(s); } catch { out[v.key] = s; }
          break;
        default:
          out[v.key] = s;
      }
    }
    return out;
  }

  // Map the API trace steps to the builder's TraceStep playback shape. The API
  // duration_ms is CPU time; started_at_ms is the running sum for the timeline.
  private _mapApiTrace(steps: components['schemas']['TraceStep'][]): TraceStep[] {
    let elapsed = 0;
    return steps.map((s, i) => {
      const dur = s.duration_ms ?? 0;
      const started = elapsed;
      elapsed += dur;
      const status: StepStatus =
        s.status === 'error' ? 'fail' : s.port === 'timeout' ? 'timeout' : s.status === 'skipped' ? 'skipped' : 'ok';
      return {
        id: `s${i}`,
        node_id: s.node_id,
        node_kind: s.node_kind as FlowNodeKind,
        label: s.node_kind.replace('_', ' '),
        started_at_ms: started,
        duration_ms: dur,
        status,
        inputs: (s.input ?? {}) as Record<string, unknown>,
        outputs: (s.output ?? {}) as Record<string, unknown>,
        note: s.port ? `→ ${s.port}` : undefined,
        region: s.region ?? undefined,
        iteration: s.iteration ?? undefined,
        branch: s.branch ?? undefined,
        caught: s.caught ?? undefined,
      };
    });
  }

  private _escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
  private _highlightJson(value: unknown): string {
    const json = this._escapeHtml(JSON.stringify(value, null, 2));
    return json
      .replace(/"([^"]+)"(\s*:)/g, '<span class="json-key">"$1"</span>$2')
      .replace(/: "([^"]*)"/g, ': <span class="json-string">"$1"</span>')
      .replace(/: (-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
      .replace(/: (true|false)/g, ': <span class="json-bool">$1</span>')
      .replace(/: null/g, ': <span class="json-null">null</span>');
  }

  private _statusIcon(s: StepStatus): string {
    return s === 'ok' ? 'check' : s === 'fail' ? 'x' : s === 'timeout' ? 'clock' : 'minus';
  }

  // ----- Node drag-drop -----

  // Convert one viewport CSS pixel into one SVG unit. The canvas SVG uses
  // viewBox preserveAspectRatio="meet" so px:unit ratio depends on the
  // rendered SVG width vs the viewBox width.
  private _svgPerCssPx(): number {
    const svg = this._svgRef ?? this.shadowRoot?.querySelector('svg.canvas-svg') as SVGSVGElement | null;
    this._svgRef = svg;
    if (!svg) return 1;
    const rect = svg.getBoundingClientRect();
    const vbWidth = svg.viewBox.baseVal.width || NODE_W_PX;
    return rect.width > 0 ? vbWidth / rect.width : 1;
  }

  // Map a viewport point to WORLD coords via the transformed <g>'s screen CTM —
  // correct under any pan/zoom. Falls back to raw coords when getScreenCTM is
  // unavailable (jsdom).
  private _clientToSvg(cx: number, cy: number): { x: number; y: number } {
    const g = this._viewportRef ?? (this.shadowRoot?.querySelector('g.viewport') as SVGGElement | null);
    this._viewportRef = g;
    try {
      const ctm = g?.getScreenCTM?.();
      if (!g || !ctm || typeof DOMPoint === 'undefined') return { x: cx, y: cy };
      const p = new DOMPoint(cx, cy).matrixTransform(ctm.inverse());
      return { x: p.x, y: p.y };
    } catch {
      return { x: cx, y: cy };
    }
  }

  private _svgEl(): SVGSVGElement | null {
    const svg = this._svgRef ?? (this.shadowRoot?.querySelector('svg.canvas-svg') as SVGSVGElement | null);
    this._svgRef = svg;
    return svg;
  }

  // ----- Pan / zoom -----
  private _onCanvasPointerDown = (e: PointerEvent): void => {
    if (e.button !== 0) return;
    const t = e.target as Element;
    // Only pan from the background — node/port pointerdowns stopPropagation.
    if (!t.classList?.contains('canvas-bg') && t.tagName?.toLowerCase() !== 'svg') return;
    (e.currentTarget as Element).setPointerCapture?.(e.pointerId);
    this._panState = { startX: e.clientX, startY: e.clientY, ox: this._panX, oy: this._panY };
    this._selectedNodeId = null;
    this._selectedEdgeId = null;
  };

  private _onCanvasPointerMove = (e: PointerEvent): void => {
    // A port-drag started inside a foreignObject often loses pointer capture
    // once the pointer crosses onto the SVG canvas, so the move/up land here
    // instead of the port. Keep the draft tracking (and finalize on up) here too
    // or the connection line freezes / never clears.
    if (this._edgeDraft) {
      const p = this._clientToSvg(e.clientX, e.clientY);
      this._edgeDraft = { ...this._edgeDraft, cx: p.x, cy: p.y };
      const t = this._nodeAt(p.x, p.y);
      this._setDraftTarget(t);
      return;
    }
    if (!this._panState) return;
    this._panX = this._panState.ox + (e.clientX - this._panState.startX);
    this._panY = this._panState.oy + (e.clientY - this._panState.startY);
  };

  private _onCanvasPointerUp = (e: PointerEvent): void => {
    if (this._edgeDraft) this._finishEdgeDraft(e.clientX, e.clientY);
    if (this._panState) {
      (e.currentTarget as Element).releasePointerCapture?.(e.pointerId);
      this._panState = null;
    }
  };

  private _onWheel = (e: WheelEvent): void => {
    e.preventDefault();
    const svg = this._svgEl();
    const rect = svg?.getBoundingClientRect();
    const cx = rect ? e.clientX - rect.left : 0;
    const cy = rect ? e.clientY - rect.top : 0;
    // Magnitude-based factor so a trackpad's many small deltas don't zoom
    // exponentially (agy HIGH-4).
    this._zoomAround(cx, cy, Math.exp(-e.deltaY / 400));
  };

  private _zoomAround(cx: number, cy: number, factor: number): void {
    const z2 = Math.min(2.5, Math.max(0.25, this._zoom * factor));
    if (z2 === this._zoom) return;
    this._panX = cx - (cx - this._panX) * (z2 / this._zoom);
    this._panY = cy - (cy - this._panY) * (z2 / this._zoom);
    this._zoom = z2;
  }

  private _zoomButton(factor: number): void {
    const svg = this._svgEl();
    const rect = svg?.getBoundingClientRect();
    this._zoomAround(rect ? rect.width / 2 : 0, rect ? rect.height / 2 : 0, factor);
  }

  private _resetView(): void {
    this._zoom = 1;
    this._panX = 48;
    this._panY = 48;
  }

  private _fitView(): void {
    if (this._nodes.length === 0) {
      this._resetView();
      return;
    }
    const minX = Math.min(...this._nodes.map(n => n.x));
    const minY = Math.min(...this._nodes.map(n => n.y));
    const maxX = Math.max(...this._nodes.map(n => n.x + NODE_W_PX));
    const maxY = Math.max(...this._nodes.map(n => n.y + NODE_H_PX));
    const rect = this._svgEl()?.getBoundingClientRect();
    const w = rect?.width ?? 800;
    const h = rect?.height ?? 600;
    const pad = 60;
    const z = Math.min(1.5, Math.max(0.25, Math.min((w - 2 * pad) / (maxX - minX || 1), (h - 2 * pad) / (maxY - minY || 1))));
    this._zoom = z;
    this._panX = (w - (maxX - minX) * z) / 2 - minX * z;
    this._panY = (h - (maxY - minY) * z) / 2 - minY * z;
  }

  private _doAutoArrange(): void {
    if (this._nodes.length === 0) return;
    const byId = new Map(autoArrange(this._nodes, this._edges).map(p => [p.id, p]));
    this._nodes = this._nodes.map(n => {
      const p = byId.get(n.id);
      return p ? { ...n, x: p.x, y: p.y } : n;
    });
    this._markDirty();
    void this.updateComplete.then(() => this._fitView());
  }

  // Drag-and-drop authoring: a palette item is dragged (dataTransfer carries
  // its kind) and dropped on the canvas, which creates a node at the drop point.
  // Persist a graph the runtime can parse: the backend GraphNode reads `type`
  // and `config`, the UI reads `kind` and `params`. We store a superset so a
  // saved graph round-trips in the builder AND validates/publishes on the
  // server without a separate translation step. Edge `label` carries the
  // branch port the runtime resolves on.
  private _serializeGraph(): { nodes: unknown[]; edges: unknown[] } {
    return {
      nodes: this._nodes.map(n => ({ ...n, type: n.kind, config: n.params ?? {} })),
      // The backend compiles the runtime port from `label`, so it MUST equal the
      // port id (from_port). Prefer from_port over any stale display label (codex HIGH).
      edges: this._edges.map(e => ({ ...e, label: e.from_port ?? e.label ?? '' })),
    };
  }

  private _onPaletteDragStart(e: DragEvent, kind: FlowNodeKind): void {
    if (!e.dataTransfer || !RUNTIME_KINDS.has(kind)) {
      e.preventDefault();
      return;
    }
    e.dataTransfer.setData('application/x-or-node', kind);
    e.dataTransfer.effectAllowed = 'copy';
  }

  private _onCanvasDragOver = (e: DragEvent): void => {
    if (e.dataTransfer?.types.includes('application/x-or-node')) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
      const p = this._clientToSvg(e.clientX, e.clientY);
      const ctx = this._bodyDropContext(p.x, p.y);
      const next = ctx?.ownerId ?? null;
      if (next !== this._dropHoverOwner) {
        this._dropHoverOwner = next;
        this._dropHoverRegion = ctx?.region ?? null;
      }
    }
  };

  private _onCanvasDragLeave = (): void => { this._dropHoverOwner = null; this._dropHoverRegion = null; };

  private _onCanvasDrop = (e: DragEvent): void => {
    const kind = e.dataTransfer?.getData('application/x-or-node') as FlowNodeKind;
    if (!kind) return;
    e.preventDefault();
    if (!RUNTIME_KINDS.has(kind)) {
      this._flashAction(`"${kind}" is not a v0.2 runtime node.`, 'warn');
      return;
    }
    const p = this._clientToSvg(e.clientX, e.clientY);
    // Center the card under the cursor (half its size), then 10px world-snap; no
    // Math.max(0) — the canvas pans, so negative coords are fine.
    const x = Math.round((p.x - NODE_W_PX / 2) / 10) * 10;
    const y = Math.round((p.y - NODE_H_PX / 2) / 10) * 10;
    const entry = this._paletteEntry(kind);
    const id = this._genNodeId();
    this._nodes = [
      ...this._nodes,
      { id, kind, label: entry?.label ?? kind, description: entry?.desc ?? '', x, y, params: {}, outputs: outputsForNode({ kind, params: {} }) },
    ];
    // Dropped onto a control node or inside its body frame → add it to that body
    // and auto-wire it (the discoverable way to populate a loop/try/parallel
    // body, vs. hunting for the body port). `end` can't go in a body.
    const ctx = kind === 'end' ? null : this._bodyDropContext(p.x, p.y, id);
    if (ctx) {
      this._wireIntoBody(ctx, id);
      this._flashAction(`Added "${entry?.label ?? kind}" into ${this._regionMeta(ctx.region).label}.`, 'ok');
    } else {
      this._flashAction(`Added "${entry?.label ?? kind}" — Save draft to persist.`, 'ok');
    }
    this._selectedNodeId = id;
    this._dropHoverOwner = null;
    this._dropHoverRegion = null;
    this._recomputeRegions();
    this._markDirty();
  };

  // Which body region a drop point falls in: an existing region frame (bbox), or
  // directly on a control node (seeds its first body). Excludes the just-added
  // node id from the frame bbox. Returns the owner + body port to wire from.
  private _bodyDropContext(x: number, y: number, excludeId = ''): { region: string; ownerId: string; bodyPort: string } | null {
    const PAD = 22, HEAD = 26;
    const groups = new Map<string, FlowNode[]>();
    for (const n of this._nodes) {
      if (n.id === excludeId || !n.region) continue;
      (groups.get(n.region) ?? groups.set(n.region, []).get(n.region)!).push(n);
    }
    for (const [region, members] of groups) {
      const x0 = Math.min(...members.map(m => m.x)) - PAD;
      const y0 = Math.min(...members.map(m => m.y)) - PAD - HEAD;
      const x1 = Math.max(...members.map(m => m.x + NODE_W_PX)) + PAD;
      const y1 = Math.max(...members.map(m => m.y + NODE_H_PX)) + PAD;
      if (x >= x0 && x <= x1 && y >= y0 && y <= y1) {
        const hash = region.indexOf('#');
        const ownerId = hash >= 0 ? region.slice(0, hash) : region;
        const bodyPort = hash >= 0 ? `body:${region.slice(hash + 1)}` : 'body';
        return { region, ownerId, bodyPort };
      }
    }
    const onNode = this._nodeAt(x, y);
    if (onNode && onNode.id !== excludeId && CONTROL_KINDS.has(onNode.kind)) {
      const bodyPort = onNode.kind === 'parallel' ? 'body:0' : 'body';
      return { region: this._regionIdForPort(onNode.id, bodyPort)!, ownerId: onNode.id, bodyPort };
    }
    return null;
  }

  // Wire a freshly-added node into a body: as the entry if the body is empty,
  // else chained off the body's tail (a member with a free, in-region output).
  private _wireIntoBody(ctx: { region: string; ownerId: string; bodyPort: string }, newId: string): void {
    const hasEntry = this._edges.some(e => e.from === ctx.ownerId && (e.from_port ?? e.label) === ctx.bodyPort);
    if (!hasEntry) {
      this._edges = [...this._edges, { id: this._genEdgeId(), from: ctx.ownerId, to: newId, from_port: ctx.bodyPort, label: ctx.bodyPort, branch: 'success' }];
      return;
    }
    const members = this._nodes.filter(n => n.id !== newId && n.region === ctx.region);
    const inRegionSucc = new Set(this._edges.filter(e => members.some(m => m.id === e.from) && members.some(m => m.id === e.to)).map(e => e.from));
    const tails = members.filter(m => !inRegionSucc.has(m.id));
    const tail = tails[tails.length - 1] ?? members[members.length - 1];
    if (!tail) return;
    const port = (outputsForNode(tail)[0]?.id) ?? 'done';
    this._edges = [...this._edges, { id: this._genEdgeId(), from: tail.id, to: newId, from_port: port, label: port, branch: 'success' }];
  }

  // Drop edges that no longer make sense: missing endpoints, a terminal source,
  // or a from_port that isn't one of the source's current outputs (e.g. after a
  // switch_case case was removed). Single-'done' sources accept any edge.
  private _pruneEdges(nodes: FlowNode[], edges: FlowEdge[]): FlowEdge[] {
    const byId = new Map(nodes.map(n => [n.id, n]));
    return edges.filter(e => {
      const from = byId.get(e.from);
      if (!from || !byId.has(e.to)) return false;
      const ports = outputsForNode(from);
      if (ports.length === 0) return false;
      const port = e.from_port ?? e.label ?? 'done';
      return ports.some(o => o.id === port) || (ports.length === 1 && ports[0]!.id === 'done');
    });
  }

  // ----- Edge connecting (port drag) -----
  private _onPortPointerDown(e: PointerEvent, node: FlowNode, o: FlowNodeOutput): void {
    if (e.button !== 0 || this._simMode === 'sim') return;
    e.stopPropagation();
    (e.currentTarget as Element).setPointerCapture(e.pointerId);
    const p = this._clientToSvg(e.clientX, e.clientY);
    this._edgeDraft = { fromId: node.id, fromPort: o.id, portKind: o.kind, cx: p.x, cy: p.y };
  }

  private _onPortPointerMove = (e: PointerEvent): void => {
    if (!this._edgeDraft) return;
    const p = this._clientToSvg(e.clientX, e.clientY);
    this._edgeDraft = { ...this._edgeDraft, cx: p.x, cy: p.y };
    const t = this._nodeAt(p.x, p.y);
    this._setDraftTarget(t);
  };

  private _onPortPointerUp = (e: PointerEvent): void => {
    if (!this._edgeDraft) return;
    (e.currentTarget as Element).releasePointerCapture?.(e.pointerId);
    this._finishEdgeDraft(e.clientX, e.clientY);
  };

  // Single exit for an edge drag: always clears the draft (so a drop on empty
  // canvas can't leave a stuck red line), then connects only if it landed on a
  // different node. Called from both the port and the canvas pointerup.
  // Update the hovered drop target + whether dropping there would cross a region
  // boundary (so the canvas can warn before the drop).
  private _setDraftTarget(t: FlowNode | undefined): void {
    const id = t && t.id !== this._edgeDraft?.fromId ? t.id : null;
    this._edgeDraftTarget = id;
    this._edgeDraftInvalid = id != null && this._draftTargetInvalid(id);
  }

  private _draftTargetInvalid(targetId: string): boolean {
    const d = this._edgeDraft;
    if (!d) return false;
    const fromNode = this._nodes.find(n => n.id === d.fromId);
    const toNode = this._nodes.find(n => n.id === targetId);
    return !!(fromNode && toNode && this._connectionError(fromNode, d.fromPort, toNode));
  }

  private _finishEdgeDraft(clientX: number, clientY: number): void {
    const draft = this._edgeDraft;
    if (!draft) return;
    this._edgeDraft = null;
    this._edgeDraftTarget = null;
    this._edgeDraftInvalid = false;
    const p = this._clientToSvg(clientX, clientY);
    const target = this._nodeAt(p.x, p.y);
    if (!target || target.id === draft.fromId) return;
    this._connectEdge(draft.fromId, draft.fromPort, draft.portKind, target.id);
  }

  // Topmost node whose bounds contain the world point (reverse render order).
  private _nodeAt(x: number, y: number): FlowNode | undefined {
    return [...this._nodes].reverse().find(n =>
      x >= n.x && x <= n.x + NODE_W_PX && y >= n.y && y <= n.y + NODE_H_PX);
  }

  // Why a flow connection from->to would be a region-boundary error, or null if
  // it's allowed. Used to BLOCK the connection at drag time (and to paint the
  // hovered target red) rather than letting the user create an invalid edge that
  // only Validate would catch.
  private _connectionError(fromNode: FlowNode, fromPort: string, toNode: FlowNode): string | null {
    // A body/body:N port defines/moves the body entry — allowed to any non-end.
    if (CONTROL_KINDS.has(fromNode.kind) && OrFlowBuilder._isBodyPort(fromPort)) {
      return toNode.kind === 'end' ? 'An End node can’t go inside a loop / try body.' : null;
    }
    const src = fromNode.region ?? '';
    const tgt = toNode.region ?? '';
    if (src === tgt) return null; // same region (incl. both top level)
    if (src !== '' && toNode.kind === 'end') {
      return 'Can’t exit a loop / try body straight to an End — route out through the control node’s “after” port.';
    }
    if (tgt === '') return null; // a top-level target joins the source's body
    return src === ''
      ? 'Can’t wire into a loop / try body from outside — only the control node’s body port enters it.'
      : 'That node is inside a different body.';
  }

  private _connectEdge(fromId: string, fromPort: string, portKind: string, toId: string): void {
    const branch: FlowEdge['branch'] = portKind === 'timeout' ? 'timeout' : portKind === 'error' ? 'fallback' : 'success';
    const fromNode = this._nodes.find(n => n.id === fromId);
    const toNode = this._nodes.find(n => n.id === toId);
    if (fromNode && toNode) {
      const err = this._connectionError(fromNode, fromPort, toNode);
      if (err) { this._flashAction(err, 'warn'); return; }
    }
    const isLinear = (fromNode ? outputsForNode(fromNode) : []).length <= 1;
    // One edge per (from, port) — compare the NORMALIZED port so a stale
    // label-only edge is still replaced (codex HIGH). Linear nodes: one outgoing.
    const kept = this._edges.filter(ed => {
      if (ed.from !== fromId) return true;
      return isLinear ? false : (ed.from_port ?? ed.label ?? 'done') !== fromPort;
    });
    this._edges = [...kept, { id: this._genEdgeId(), from: fromId, to: toId, from_port: fromPort, label: fromPort, branch }];
    this._recomputeRegions();
    this._markDirty();
  }

  // Region id a control node's body port enters: loops/try use the owner id;
  // parallel's body:N uses "<owner>#<N>" (matches backend compile.go).
  private _regionIdForPort(ownerId: string, port: string): string | null {
    if (port === 'body') return ownerId;
    const m = /^body:(\d+)$/.exec(port);
    return m ? `${ownerId}#${m[1]}` : null;
  }

  private static _isBodyPort(port: string): boolean {
    return port === 'body' || /^body:\d+$/.test(port);
  }

  // Region membership is DERIVED from the graph, never tracked incrementally
  // (which went stale on reconnect/delete/multi-path — cross-AI review). A body
  // edge seeds its target as a region entry; membership then floods forward along
  // FLOW edges (a control node's done/catch continues its OWN region; its body
  // edges are not flow, so a nested body floods only from its own entry). Run
  // after every graph mutation; produces exactly the explicit node.region the
  // backend requires, with no orphans or stale ids.
  private _recomputeRegions(): void {
    const byId = new Map(this._nodes.map(n => [n.id, n]));
    const region: Record<string, string> = {};
    const seeded = new Set<string>();
    const flowAdj = new Map<string, string[]>();
    for (const e of this._edges) {
      const from = byId.get(e.from);
      const port = e.from_port ?? e.label ?? '';
      if (from && CONTROL_KINDS.has(from.kind) && OrFlowBuilder._isBodyPort(port)) {
        const r = this._regionIdForPort(from.id, port);
        if (r && byId.has(e.to)) {
          region[e.to] = r;
          seeded.add(e.to);
        }
        continue; // body edge is a region declaration, not flow
      }
      (flowAdj.get(e.from) ?? flowAdj.set(e.from, []).get(e.from)!).push(e.to);
    }
    const queue = [...seeded];
    while (queue.length) {
      const n = queue.shift()!;
      const rn = region[n];
      if (rn === undefined) continue;
      for (const m of flowAdj.get(n) ?? []) {
        if (seeded.has(m) || region[m] !== undefined) continue; // entry/conflict: keep
        // An end is terminal/top-level and can't live in a region (backend
        // end_in_region) — leave it out so the stray edge reads as a boundary
        // crossing the user must reroute through the control's done port.
        if (byId.get(m)?.kind === 'end') continue;
        region[m] = rn;
        queue.push(m);
      }
    }
    this._nodes = this._nodes.map(n => {
      const next = region[n.id];
      return (n.region ?? '') === (next ?? '') ? n : { ...n, region: next };
    });
  }

  // "Remove from body" = sever the edges that bring this node into its region;
  // the derivation then drops it (and any nodes that only reached the body
  // through it) back to top level.
  private _ejectFromRegion(id: string): void {
    const node = this._nodes.find(n => n.id === id);
    if (!node || !node.region) return;
    const inRegionPred = (e: FlowEdge): boolean => {
      if (e.to !== id) return false;
      const from = this._nodes.find(n => n.id === e.from);
      const port = e.from_port ?? e.label ?? '';
      // a body edge into this node, or a flow edge from a node in the same region
      if (from && CONTROL_KINDS.has(from.kind) && OrFlowBuilder._isBodyPort(port)) return true;
      return (from?.region ?? '') === node.region;
    };
    this._edges = this._edges.filter(e => !inRegionPred(e));
    this._recomputeRegions();
    this._markDirty();
  }

  // _dirty = the in-memory graph differs from the saved draft. Validate/Publish
  // act on the SAVED graph, so they auto-save first when dirty (codex MED).
  private _dirty = false;
  private _markDirty(): void {
    this._validation = null;
    this._dirty = true;
    this._commitHistory();
  }

  // ----- Undo / redo -----
  // Each entry is a committed {nodes, edges} state; _historyAt points at the
  // current one. Structural edits commit via _markDirty; a node drag commits
  // once on pointer-up (not per frame). Restoring sets state directly and must
  // NOT re-commit, so it's guarded by _restoringHistory.
  private _history: { nodes: FlowNode[]; edges: FlowEdge[] }[] = [];
  private _historyAt = -1;
  private _restoringHistory = false;
  private static readonly _HISTORY_CAP = 100;

  private _snapshot(): { nodes: FlowNode[]; edges: FlowEdge[] } {
    return { nodes: structuredClone(this._nodes), edges: structuredClone(this._edges) };
  }

  // Seed the baseline (pre-edit) state so the first edit is undoable.
  private _resetHistory(): void {
    this._history = [this._snapshot()];
    this._historyAt = 0;
  }

  private _commitHistory(): void {
    if (this._restoringHistory) return;
    // Drop any redo branch, then append the new current state.
    this._history = this._history.slice(0, this._historyAt + 1);
    this._history.push(this._snapshot());
    if (this._history.length > OrFlowBuilder._HISTORY_CAP) this._history.shift();
    this._historyAt = this._history.length - 1;
  }

  private get _canUndo(): boolean { return this._historyAt > 0; }
  private get _canRedo(): boolean { return this._historyAt < this._history.length - 1; }

  private _restoreHistory(at: number): void {
    const snap = this._history[at];
    if (!snap) return;
    this._restoringHistory = true;
    this._nodes = structuredClone(snap.nodes);
    this._edges = structuredClone(snap.edges);
    this._historyAt = at;
    this._validation = null;
    this._dirty = true; // the restored state differs from the saved draft
    this._restoringHistory = false;
  }

  private _undo(): void {
    if (!this._canUndo) return;
    this._restoreHistory(this._historyAt - 1);
    this._flashAction('Undo', 'ok');
  }

  private _redo(): void {
    if (!this._canRedo) return;
    this._restoreHistory(this._historyAt + 1);
    this._flashAction('Redo', 'ok');
  }

  private _selectEdge(id: string): void {
    if (!id) return; // a missing id would select every edge (see _renderEdges)
    this._selectedEdgeId = id;
    this._selectedNodeId = null;
  }

  private _deleteEdge(id: string): void {
    this._edges = this._edges.filter(e => e.id !== id);
    if (this._selectedEdgeId === id) this._selectedEdgeId = null;
    this._recomputeRegions(); // a severed body edge drops its (now unreachable) members
    this._markDirty();
  }

  private _deleteNode(id: string): void {
    this._nodes = this._nodes.filter(n => n.id !== id);
    this._edges = this._edges.filter(e => e.from !== id && e.to !== id);
    // Deleting a control node (or any in-body node) re-derives membership: members
    // that only reached the body through the removed node fall back to top level.
    this._recomputeRegions();
    if (this._selectedNodeId === id) this._selectedNodeId = null;
    this._markDirty();
  }

  // Keyboard: Delete/Backspace removes the selected edge/node; Cmd/Ctrl+C copies
  // the selected node; Cmd/Ctrl+V pastes a duplicate. Never steals a shortcut
  // while typing in a config field (text selection / native copy stay intact).
  private _onKeyDown = (e: KeyboardEvent): void => {
    if (this._simMode === 'sim') return;
    const editable = (n: unknown): boolean => {
      const el = n as HTMLElement | null;
      const tag = el?.tagName?.toLowerCase();
      return tag === 'input' || tag === 'textarea' || tag === 'select' || !!el?.isContentEditable;
    };
    const typing = editable(e.composedPath()[0]) || editable(document.activeElement);
    const mod = e.metaKey || e.ctrlKey;

    // Undo/redo. Skip while typing so native input undo/redo stays intact.
    if (mod && (e.key === 'z' || e.key === 'Z')) {
      if (typing) return;
      e.preventDefault();
      if (e.shiftKey) this._redo();
      else this._undo();
      return;
    }
    if (mod && (e.key === 'y' || e.key === 'Y')) {
      if (typing) return;
      e.preventDefault();
      this._redo();
      return;
    }

    if (mod && (e.key === 'c' || e.key === 'C')) {
      if (typing || !this._selectedNodeId) return;
      this._copyNode(this._selectedNodeId);
      e.preventDefault();
      return;
    }
    if (mod && (e.key === 'v' || e.key === 'V')) {
      if (typing || !this._clipboardNode) return;
      this._pasteNode();
      e.preventDefault();
      return;
    }
    if (e.key !== 'Delete' && e.key !== 'Backspace') return;
    // Only ever acts when THIS canvas has a selection (set by interacting with
    // it), so it can't touch the graph from unrelated parts of the page.
    if (!this._selectedEdgeId && !this._selectedNodeId) return;
    if (typing) return;
    e.preventDefault();
    if (this._selectedEdgeId) this._deleteEdge(this._selectedEdgeId);
    else if (this._selectedNodeId) this._deleteNode(this._selectedNodeId);
  };

  // Snapshot the selected node's content (NOT its id/position) for paste.
  private _copyNode(id: string): void {
    const n = this._nodes.find(x => x.id === id);
    if (!n) return;
    this._clipboardNode = structuredClone({
      kind: n.kind, label: n.label, description: n.description,
      x: n.x, y: n.y, params: n.params, outputs: n.outputs,
    });
    this._pasteCount = 0;
    this._flashAction(`Copied "${n.label ?? n.kind}".`, 'ok');
  }

  // Paste a fresh-id clone, offset so repeated pastes cascade. Edges are NOT
  // copied (a duplicated node starts unconnected). The new node is selected.
  private _pasteNode(): void {
    const c = this._clipboardNode;
    if (!c) return;
    this._pasteCount += 1;
    const off = 30 * this._pasteCount;
    const id = this._genNodeId();
    const clone = structuredClone(c) as FlowNode;
    this._nodes = [...this._nodes, { ...clone, id, x: c.x + off, y: c.y + off }];
    this._selectedNodeId = id;
    this._markDirty();
    this._flashAction(`Pasted "${clone.label ?? clone.kind}".`, 'ok');
  }

  private _genEdgeId(): string {
    let id = '';
    do {
      id = 'e_' + Math.random().toString(36).slice(2, 8);
    } while (this._edges.some(e => e.id === id));
    return id;
  }

  // X of a node's output port anchor (mirrors _renderEdges' portX).
  private _portX(node: FlowNode, portId: string): number {
    const outs = node.outputs;
    if (!outs || outs.length <= 1) return node.x + NODE_W_PX / 2;
    const idx = outs.findIndex(o => o.id === portId);
    if (idx < 0) return node.x + NODE_W_PX / 2;
    return node.x + portFracs(outs.length)[idx]! * NODE_W_PX;
  }

  private _renderEdgeDraft() {
    const d = this._edgeDraft;
    if (!d) return nothing;
    const from = this._nodes.find(n => n.id === d.fromId);
    if (!from) return nothing;
    const x1 = this._portX(from, d.fromPort);
    const y1 = from.y + NODE_H_PX;
    const dy = Math.max(40, (d.cy - y1) * 0.5);
    return svg`<path class="edge edge--draft" marker-end="url(#arrow-draft)" d=${`M ${x1} ${y1} C ${x1} ${y1 + dy}, ${d.cx} ${d.cy - dy}, ${d.cx} ${d.cy}`} />`;
  }

  private _paletteEntry(kind: FlowNodeKind): PaletteEntry | undefined {
    for (const g of PALETTE) {
      const hit = g.items.find(i => i.kind === kind);
      if (hit) return hit;
    }
    return undefined;
  }

  private _genNodeId(): string {
    let id = '';
    do {
      id = 'n_' + Math.random().toString(36).slice(2, 8);
    } while (this._nodes.some(n => n.id === id));
    return id;
  }

  private _renderRail(label: string, icon: string, onExpand: () => void) {
    return html`
      <aside class="pane pane-rail">
        <button class="pane-rail-btn" title="Expand ${label}" @click=${onExpand}>
          <uk-icon icon=${icon} height="14" width="14"></uk-icon>
        </button>
        <span class="pane-rail-label">${label}</span>
      </aside>
    `;
  }

  private _onNodePointerDown(e: PointerEvent, node: FlowNode): void {
    // Ignore right-click + middle-click. Also ignore if the click target
    // is the chip row — we only drag from the card body.
    if (e.button !== 0) return;
    // Block drag while sim is running. The card pulses (.node-card--active /
    // .node-card--fail) signal playback state; dragging the pulsing node
    // mid-trace persists a new position that confuses the user about
    // which step the trace is on. (Design review MUST.)
    if (this._simMode === 'sim') return;
    const target = e.target as HTMLElement;
    if (target.closest('.node-card-ports')) return;
    e.stopPropagation(); // don't let the canvas start a pan
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    // Grab offset in SVG units = where on the card the user grabbed, so the
    // node follows the cursor without snapping its origin to the pointer.
    const p = this._clientToSvg(e.clientX, e.clientY);
    this._drag = { id: node.id, startX: e.clientX, startY: e.clientY, offX: p.x - node.x, offY: p.y - node.y };
    this._pendingClick = { id: node.id };
  }

  private _onNodePointerMove(e: PointerEvent): void {
    if (!this._drag) return;
    // 4px threshold before promoting to drag (vs click).
    if (this._draggedId == null && Math.hypot(e.clientX - this._drag.startX, e.clientY - this._drag.startY) < 4) return;
    this._draggedId = this._drag.id;
    this._pendingClick = null;
    const p = this._clientToSvg(e.clientX, e.clientY);
    const newX = Math.round((p.x - this._drag.offX) / 10) * 10;
    const newY = Math.round((p.y - this._drag.offY) / 10) * 10;
    this._nodes = this._nodes.map(n =>
      n.id === this._drag!.id ? { ...n, x: newX, y: newY } : n
    );
    this._dirty = true;
  }

  private _onNodePointerUp(e: PointerEvent): void {
    if (this._drag && (e.currentTarget as HTMLElement).hasPointerCapture(e.pointerId)) {
      (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
    }
    // If we never moved past threshold, treat as a click → select node
    if (this._pendingClick && this._pendingClick.id === this._drag?.id) {
      this._selectedNodeId = this._pendingClick.id;
    }
    // A completed drag (moved past threshold) is one undoable step — commit the
    // post-drag positions once here (the move itself only set _dirty per frame).
    if (this._draggedId != null) this._commitHistory();
    this._drag = null;
    this._draggedId = null;
    this._pendingClick = null;
  }

  /** Set of nodeId:portId pairs we've already warned about — dedupes the
   *  per-render console spam from stale from_port references. */
  private _warnedPorts = new Set<string>();
  private _warnUnknownPort(nodeId: string, portId: string, declared: string[]): void {
    const key = `${nodeId}:${portId}`;
    if (this._warnedPorts.has(key)) return;
    this._warnedPorts.add(key);
    if (typeof console !== 'undefined') {
      console.warn(`[flow-builder] Edge from "${nodeId}" references unknown port "${portId}". Declared: [${declared.join(', ')}]`);
    }
  }

  private get _flow(): { name: string; code: string; version: number; status: string } {
    const f = this._loaded;
    if (!f) {
      return {
        name: this._nameDraft || 'New flow',
        code: this._codeDraft || 'unsaved',
        version: 0,
        status: 'draft',
      };
    }
    return { name: f.name, code: f.code, version: f.version, status: f.enabled ? 'draft' : 'archived' };
  }

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', { detail: { path }, bubbles: true, composed: true }),
    );
  }

  private _toneFor(kind: FlowNodeKind): PaletteEntry['tone'] {
    return KIND_PROPS[kind]?.tone ?? 'neutral';
  }

  private _iconFor(kind: FlowNodeKind): string {
    return KIND_PROPS[kind]?.icon ?? 'circle';
  }

  private _previewParam(node: FlowNode): string {
    const p = node.params ?? {};
    // Runtime kinds: derive the preview from the authoritative config keys
    // (KIND_FIELDS) so it never shows stale names or `undefined`.
    if (RUNTIME_KINDS.has(node.kind)) {
      const parts = (KIND_FIELDS[node.kind] ?? []).map(f => {
        const v = p[f.key];
        if (f.type === 'cases') {
          const arr = Array.isArray(v) ? (v as string[]) : [];
          return arr.length ? `${arr.length} case${arr.length === 1 ? '' : 's'}` : '';
        }
        if (v === undefined || v === '' || v === null) return '';
        return `${f.key}: ${Array.isArray(v) ? (v as string[]).join(', ') : v}`;
      }).filter(Boolean);
      return parts.join('  ·  ');
    }
    switch (node.kind) {
      case 'trigger':      return `channel: ${p['channel']}`;
      case 'set_var':      return `${p['name']} = ${p['value_expr']}`;
      case 'compute':      return String(p['expr'] ?? '');
      case 'http_request': return `${p['method']} ${String(p['url'] ?? '').replace(/^https?:\/\//, '')}`;
      case 'webhook':      return `→ ${String(p['url'] ?? '').replace(/^https?:\/\//, '')}`;
      case 'if_else':      return String(p['expr'] ?? '');
      case 'switch_case': {
        const cases = (p['cases'] as string[] | undefined) ?? [];
        return `${p['switch_on']}  (${cases.length} cases)`;
      }
      case 'loop_for':     return String(p['iter'] ?? '') + (p['max_iter'] ? `  max=${p['max_iter']}` : '');
      case 'loop_while':   return `while ${p['expr']}`;
      case 'parallel':     return `fan-out × ${p['branches'] ?? '?'}`;
      case 'try_catch': {
        const kinds = (p['catch_kinds'] as string[] | undefined) ?? [];
        return `catch ${kinds.join(', ') || '*'}`;
      }
      case 'wait':            return `${p['ms']} ms`;
      case 'get_dtmf':        return `${p['max_digits']} digit${p['max_digits'] === 1 ? '' : 's'}, ${p['timeout_sec']}s`;
      case 'prompt_text':     return String(p['prompt'] ?? '');
      case 'wait_signal':     return `signal: ${p['signal_name'] ?? '*'}`;
      case 'manual_approval': return `gate: ${(p['choices'] as string[] | undefined)?.join(' / ') ?? 'approve'}`;
      case 'script': {
        const lang = String(p['lang'] ?? 'js');
        const lines = String(p['code'] ?? '').split('\n');
        return `${lang} · ${lines.length} line${lines.length === 1 ? '' : 's'}`;
      }
      // Voice
      case 'tts_speak':       return String(p['text'] ?? p['voice'] ?? '');
      case 'play_prompt':     return String(p['file'] ?? '');
      case 'detect_speech':   return `lang: ${p['language'] ?? 'en-US'}, conf ≥${p['min_confidence'] ?? 0.7}`;
      case 'transfer_call':   return `→ ${p['number'] ?? p['queue_code']}`;
      case 'hangup':          return String(p['reason'] ?? 'normal');
      // Chat
      case 'send_message':    return String(p['text'] ?? '');
      case 'quick_replies':   return `${(p['options'] as string[] | undefined)?.length ?? 0} options`;
      case 'typing_indicator':return `${p['ms']}ms`;
      case 'attach_file':     return String(p['filename'] ?? '');
      case 'bot_handoff':     return `→ ${p['queue_code'] ?? 'human'}`;
      // Email
      case 'send_template':   return String(p['template_code'] ?? '');
      // Surveys
      case 'csat_survey':     return `1-5 scale, ${p['after_sec'] ?? 0}s wait`;
      case 'nps_survey':      return `0-10 scale, ${p['after_sec'] ?? 0}s wait`;
      // Agent state
      case 'set_agent_state': return `→ ${p['state']}`;
      case 'wrapup_timer':    return `${p['acw_sec']}s ACW`;
      case 'match_skill': {
        const skills = (p['required_skills'] as string[] | undefined) ?? [];
        return `${skills.length} skill${skills.length === 1 ? '' : 's'}, ≥${p['min_proficiency']}`;
      }
      case 'filter':       return String(p['expr'] ?? '');
      case 'route_queue':  return `→ ${p['queue_code']}  (p=${p['priority']})`;
      case 'reservation':  return `offer ${p['offer_timeout_sec']}s  retry=${p['retry']}`;
      case 'effect':       return `${p['method'] ?? ''} ${String(p['url'] ?? '').replace(/^https?:\/\//, '')}`;
      case 'log':          return `${p['level']}: ${p['message']}`;
      case 'fallback':     return p['after_sec'] ? `after ${p['after_sec']}s → ${p['queue_code']}` : String(p['reason'] ?? '');
      case 'end':          return '';
      default:             return '';
    }
  }

  // Human label + tone for a region id ("<owner>" or "<owner>#<i>"): the owning
  // control node's label, plus the branch index for parallel.
  private _regionMeta(regionId: string): { label: string; tone: string } {
    const hash = regionId.indexOf('#');
    const ownerId = hash >= 0 ? regionId.slice(0, hash) : regionId;
    const branch = hash >= 0 ? regionId.slice(hash + 1) : '';
    const owner = this._nodes.find(n => n.id === ownerId);
    const name = owner?.label ?? owner?.kind ?? 'region';
    const tone = owner?.kind === 'try_catch' ? 'warn' : 'control';
    return { label: branch === '' ? name : `${name} · branch ${branch}`, tone };
  }

  // A tinted frame auto-drawn behind each body region's member nodes (bounding
  // box). Membership is explicit (node.region); the frame is purely visual so it
  // reads as a container without container hit-testing. Sim mode hides them.
  private _renderRegions(nodes: FlowNode[]) {
    if (this._simMode === 'sim') return nothing;
    const groups = new Map<string, FlowNode[]>();
    for (const n of nodes) {
      if (!n.region) continue;
      (groups.get(n.region) ?? groups.set(n.region, []).get(n.region)!).push(n);
    }
    if (groups.size === 0) return nothing;
    const PAD = 22;
    const HEAD = 26;
    const frames = [...groups.entries()].map(([regionId, members]) => {
      const x0 = Math.min(...members.map(m => m.x)) - PAD;
      const y0 = Math.min(...members.map(m => m.y)) - PAD - HEAD;
      const x1 = Math.max(...members.map(m => m.x + NODE_W_PX)) + PAD;
      const y1 = Math.max(...members.map(m => m.y + NODE_H_PX)) + PAD;
      const { label, tone } = this._regionMeta(regionId);
      return svg`
        <g class=${'region region--' + tone + (this._dropHoverRegion === regionId ? ' region--drop' : '')}>
          <rect class="region-frame" x=${x0} y=${y0} width=${x1 - x0} height=${y1 - y0} rx="14"></rect>
          <foreignObject x=${x0 + 10} y=${y0 + 5} width=${Math.max(80, x1 - x0 - 20)} height="20">
            <div xmlns="http://www.w3.org/1999/xhtml" class="region-label">
              <uk-icon icon=${tone === 'warn' ? 'shield' : 'repeat'} height="11" width="11"></uk-icon>
              <span>${label}</span>
            </div>
          </foreignObject>
        </g>`;
    });
    return svg`${frames}`;
  }

  private _renderEdges(nodes: FlowNode[], edges: FlowEdge[]) {
    const byId = new Map(nodes.map(n => [n.id, n]));
    const NODE_W = NODE_W_PX;
    const NODE_H = NODE_H_PX;
    const isSim = this._simMode === 'sim';
    const issueEdges = isSim ? new Set<string>() : this._issueEdgeIds;

    // Vertical layout — ports distribute horizontally along the bottom
    // edge of the card (matching the .node-card-ports chip row that lives
    // at the bottom). portFracs() returns evenly-distributed fractions
    // across the card's WIDTH. Each output exits the bottom at its own x,
    // edge bezier curves down to the top of the target node.
    const portX = (node: FlowNode, portId: string | undefined): number => {
      const outs = node.outputs;
      if (!outs || outs.length === 0) return node.x + NODE_W / 2;
      if (outs.length === 1 || !portId) return node.x + NODE_W / 2;
      const idx = outs.findIndex(o => o.id === portId);
      if (idx < 0) {
        this._warnUnknownPort(node.id, portId, outs.map(o => o.id));
        return node.x + NODE_W / 2;
      }
      return node.x + portFracs(outs.length)[idx]! * NODE_W;
    };

    return svg`
      <defs>
        <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5"
                markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--border)"/>
        </marker>
        <marker id="arrow-success" viewBox="0 0 10 10" refX="9" refY="5"
                markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--success)"/>
        </marker>
        <marker id="arrow-timeout" viewBox="0 0 10 10" refX="9" refY="5"
                markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--destructive)"/>
        </marker>
        <marker id="arrow-draft" viewBox="0 0 10 10" refX="8" refY="5"
                markerWidth="6" markerHeight="6" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--primary)"/>
        </marker>
      </defs>
      ${edges.map((e) => {
        const from = byId.get(e.from);
        const to = byId.get(e.to);
        if (!from || !to) return null;
        // Vertical flow: source exits BOTTOM, target receives at TOP.
        const x1 = portX(from, e.from_port);
        const y1 = from.y + NODE_H;
        const x2 = to.x + NODE_W / 2;
        const y2 = to.y;
        const dy = Math.max(40, (y2 - y1) * 0.5);
        const d = `M ${x1} ${y1} C ${x1} ${y1 + dy}, ${x2} ${y2 - dy}, ${x2} ${y2}`;
        // Stagger label position along the bezier so fan-outs from the same
        // source (e.g. n5 case_1..4 → n6) don't stack pills on top of each
        // other. Each edge in the same fan-out group picks a different t.
        const siblings = edges.filter(other => other.from === e.from && (other.label || other.from_port));
        const siblingIdx = siblings.indexOf(e);
        // Widen the t spread for fan-outs so 4-5 sibling pills don't pile
        // up on the same bezier midline (e.g. n5 case_1..4 + default).
        const t = siblings.length > 1
          ? 0.18 + (siblingIdx / Math.max(1, siblings.length - 1)) * 0.64
          : 0.5;
        const cls = e.branch === 'success' ? 'edge edge--success'
                  : e.branch === 'timeout' ? 'edge edge--timeout'
                  : e.branch === 'fallback' ? 'edge edge--timeout'
                  : 'edge';
        const marker = e.branch === 'success' ? 'url(#arrow-success)'
                     : e.branch === 'timeout' ? 'url(#arrow-timeout)'
                     : e.branch === 'fallback' ? 'url(#arrow-timeout)'
                     : 'url(#arrow)';
        // Label pill positioned along the bezier at parameter t. Linear
        // interpolation between endpoints is a close enough approximation
        // for the pill placement — the actual bezier hugs this line for
        // our gentle control-point offsets.
        const labelX = x1 + (x2 - x1) * t;
        const labelY = y1 + (y2 - y1) * t;
        // Show the source port's friendly DISPLAY label (e.g. "after loop"), not
        // the raw port id ("done"), so a control node's continuation reads
        // distinctly from a plain node's "done".
        const portId = e.from_port ?? e.label ?? '';
        const rawLabel = outputsForNode(from).find(o => o.id === portId)?.label ?? e.label ?? '';
        // Clamp label length so cross-corridor pills can't overflow into the
        // fallback column at x=320 (clearance ~72px from main column right
        // edge). 10 chars → ~82px pill at the current 7px/char heuristic.
        const labelText = rawLabel.length > 10 ? rawLabel.slice(0, 9) + '…' : rawLabel;
        const labelKind = e.branch === 'success' ? 'success'
                       : e.branch === 'timeout' ? 'timeout'
                       : e.branch === 'fallback' ? 'fallback'
                       : 'default';
        const labelW = labelText ? Math.min(110, Math.max(24, labelText.length * 7 + 12)) : 0;
        const labelH = 16;
        // Require a truthy id so a stray undefined id can't make every edge
        // match _selectedEdgeId (undefined === undefined) and paint all selected.
        const selected = !isSim && !!e.id && e.id === this._selectedEdgeId;
        return svg`
          ${isSim ? nothing : svg`<path class="edge-hit" d=${d}
            @click=${(ev: Event) => { ev.stopPropagation(); this._selectEdge(e.id); }}></path>`}
          <path class=${cls + (selected ? ' edge--selected' : '') + (issueEdges.has(e.id) ? ' edge--invalid' : '')} d=${d} marker-end=${marker}></path>
          ${labelText ? svg`
            <rect
              class=${'edge-label-bg edge-label-bg--' + labelKind}
              x=${labelX - labelW / 2}
              y=${labelY - labelH / 2}
              width=${labelW}
              height=${labelH}
              rx="4"
            ></rect>
            <text
              class=${'edge-label edge-label--' + labelKind}
              x=${labelX}
              y=${labelY + 3.5}
              text-anchor="middle"
            >${labelText}</text>
          ` : nothing}
        `;
      })}
    `;
  }

  private _renderNodes(nodes: FlowNode[]) {
    const NODE_W = NODE_W_PX;
    const NODE_H = NODE_H_PX;
    const isSim = this._simMode === 'sim';
    const hitIds = isSim ? this._hitNodeIds : new Set<string>();
    const issueIds = isSim ? new Set<string>() : this._issueNodeIds;
    const currentNodeId = this._currentStep?.node_id ?? null;

    const stepByNode = new Map<string, TraceStep>();
    if (isSim) {
      const trace = this._activeTrace;
      for (let i = 0; i <= this._simStep && i < trace.length; i++) {
        stepByNode.set(trace[i]!.node_id, trace[i]!);
      }
    }

    // Paint the dragged (else selected) node LAST so it sits on top: each node
    // is its own <foreignObject> and SVG paints them in document order, so
    // z-index is inert — a node dragged over a later sibling would otherwise be
    // covered (its bottom ports/"done" chip vanish behind the sibling).
    const topId = this._draggedId ?? this._selectedNodeId;
    const ordered = topId && nodes.some(n => n.id === topId)
      ? [...nodes.filter(n => n.id !== topId), nodes.find(n => n.id === topId)!]
      : nodes;

    return ordered.map(node => {
      const tone = this._toneFor(node.kind);
      const icon = this._iconFor(node.kind);
      const isSelected = node.id === this._selectedNodeId;
      const preview = this._previewParam(node);
      const stepHit = stepByNode.get(node.id);
      const isCurrent = isSim && node.id === currentNodeId;
      const isHit = isSim && hitIds.has(node.id);
      const isDragging = this._draggedId === node.id;

      const cls = [
        'node-card',
        isSim && !isHit ? 'node-card--dim' : '',
        isCurrent ? 'node-card--active' : '',
        isHit && !isCurrent ? 'node-card--hit' : '',
        stepHit?.status === 'fail' ? 'node-card--fail' : '',
        issueIds.has(node.id) ? 'node-card--invalid' : '',
        isSelected && !isSim ? 'node-card--selected' : '',
        node.id === this._edgeDraftTarget ? (this._edgeDraftInvalid ? 'node-card--drop-invalid' : 'node-card--drop-target') : '',
        node.id === this._dropHoverOwner ? 'node-card--drop-target' : '',
        isDragging ? 'is-dragging' : '',
      ].filter(Boolean).join(' ');

      // Nullish (not length>0) so a terminal node's explicit [] stays empty —
      // an `end` must NOT show a fake `done` port (agy HIGH-2).
      const outs = node.outputs ?? [{ id: 'done', label: 'done', kind: 'success' as const }];

      // In sim, a reservation node's port chips become clickable to force that
      // branch (the per-node outcome control — replaces the sidebar select).
      const pinnable = isSim && node.kind === 'reservation';

      // Input anchor (top-centre): the link target. Hidden for the trigger
      // (no inbound) and in sim. Highlights while a connection is dragged over.
      const showInput = !isSim && node.kind !== 'trigger';

      return svg`
        ${showInput ? svg`<circle class=${'node-input-anchor' + (node.id === this._edgeDraftTarget ? ' node-input-anchor--active' : '')}
          cx=${node.x + NODE_W / 2} cy=${node.y} r="5"></circle>` : nothing}
        <foreignObject x=${node.x - 8} y=${node.y - 8} width=${NODE_W + 16} height=${NODE_H + 16} overflow="visible">
          <div xmlns="http://www.w3.org/1999/xhtml" style="padding:8px;box-sizing:border-box">
          <div
            class=${cls}
            title=${node.label}
            role="button"
            aria-label="Node ${node.label}. Drag to reposition."
            aria-pressed=${isSelected ? 'true' : 'false'}
            @pointerdown=${(e: PointerEvent) => this._onNodePointerDown(e, node)}
            @pointermove=${(e: PointerEvent) => this._onNodePointerMove(e)}
            @pointerup=${(e: PointerEvent) => this._onNodePointerUp(e)}
            @pointercancel=${(e: PointerEvent) => this._onNodePointerUp(e)}
            @click=${() => { if (isSim) this._selectedNodeId = node.id; }}
          >
            <div class="node-card-head">
              <span class="icon-tile icon-tile--${tone}">
                <uk-icon icon=${icon} height="12" width="12"></uk-icon>
              </span>
              <span class="node-card-kind">${node.kind.replace('_', ' ')}</span>
              ${stepHit ? html`
                <span class=${'node-card-badge node-card-badge--' + stepHit.status} title=${stepHit.status}>
                  <uk-icon icon=${this._statusIcon(stepHit.status)} height="9" width="9"></uk-icon>
                </span>
              ` : ''}
            </div>
            <div class="node-card-label">${node.label}</div>
            ${stepHit
              ? html`<div class="node-card-timing">+${stepHit.started_at_ms.toFixed(1)}ms · ${stepHit.duration_ms.toFixed(1)}ms</div>`
              : preview ? html`<div class="node-card-param" title=${preview}>${preview}</div>` : ''}
            <div class=${'node-card-ports' + (pinnable ? ' node-card-ports--pick' : '')}
              title=${pinnable ? 'Click a port to force that branch in the simulation' : 'Output cases this node can produce'}>
              ${outs.map(o => {
                const pinned = pinnable && this._simNodeOutcomes[node.id] === o.id;
                const cls = 'node-card-port node-card-port--' + o.kind
                  + (isSim ? '' : ' node-card-port--handle')
                  + (pinnable ? ' node-card-port--pick' : '')
                  + (pinned ? ' node-card-port--pinned' : '');
                if (pinnable) {
                  return html`
                    <span class=${cls}
                      title=${pinned ? `Forcing ${o.label} — click to clear` : `Click to force the ${o.label} branch`}
                      @pointerdown=${(e: PointerEvent) => e.stopPropagation()}
                      @click=${(e: MouseEvent) => { e.stopPropagation(); void this._toggleNodeOutcome(node.id, o.id); }}>${o.label}</span>`;
                }
                return html`
                  <span class=${cls}
                    title=${isSim ? o.label : 'Drag to another node to connect'}
                    @pointerdown=${isSim ? nothing : (e: PointerEvent) => this._onPortPointerDown(e, node, o)}
                    @pointermove=${isSim ? nothing : this._onPortPointerMove}
                    @pointerup=${isSim ? nothing : this._onPortPointerUp}
                    @pointercancel=${isSim ? nothing : this._onPortPointerUp}>${o.label}</span>`;
              })}
            </div>
          </div>
          </div>
        </foreignObject>
      `;
    });
  }

  // In-context explainer for a control node's ports + the body-region rule (the
  // #1 source of confusion: "which done is done?" and boundary-crossing errors).
  private _renderControlHelp(node: FlowNode) {
    if (!CONTROL_KINDS.has(node.kind)) return nothing;
    const rows: { port: string; text: string }[] =
      node.kind === 'try_catch'
        ? [
            { port: 'try', text: 'Drag to the first node of the protected block.' },
            { port: 'on error', text: 'Runs if the try block hits a domain failure.' },
            { port: 'after', text: 'Continues after the block (success OR caught).' },
          ]
        : node.kind === 'parallel'
        ? [
            { port: 'body:0…N', text: 'Each port is one branch — drag it to that branch’s first node.' },
            { port: 'after', text: 'Continues once ALL branches finish.' },
          ]
        : [
            { port: 'loop body', text: 'Drag to the first node that repeats each iteration.' },
            { port: 'after loop', text: 'Continues once the loop ends — this is the exit.' },
          ];
    return html`
      <div class="control-help">
        <div class="control-help-head"><uk-icon icon="info" height="12" width="12"></uk-icon> How this node flows</div>
        ${rows.map(r => html`<div class="control-help-row"><code>${r.port}</code><span>${r.text}</span></div>`)}
        <div class="control-help-note">
          Nodes inside the body connect <strong>only to each other</strong>; the body just ends on its
          last node. To leave, route through <strong>${node.kind === 'try_catch' ? 'after' : 'after loop'}</strong> —
          a body node wired straight to an outside node is a region-boundary error.
        </div>
      </div>`;
  }

  private _renderInspector(node: FlowNode | null) {
    if (!node) {
      return html`
        <div class="inspector-empty">
          <uk-icon icon="mouse-pointer-click" height="32" width="32"></uk-icon>
          <p>Select a node to edit its parameters.</p>
        </div>
      `;
    }

    const tone = this._toneFor(node.kind);
    const icon = this._iconFor(node.kind);

    const paramRows = Object.entries(node.params ?? {}).map(([key, value]) => {
      if (Array.isArray(value)) {
        return html`
          <div class="form-section">
            <label>${key}</label>
            <div class="chip-row">
              ${value.map(v => html`<span class="chip">${String(v)}</span>`)}
            </div>
          </div>
        `;
      }
      // Render multi-line strings (e.g. script code) as a monospace block
      // with line numbers — script / DSL escape hatch.
      const str = String(value);
      if (key === 'code' || (typeof value === 'string' && str.includes('\n'))) {
        const lines = str.split('\n');
        const lastIdx = lines.length - 1;
        return html`
          <div class="form-section">
            <label>${key}</label>
            <pre class="code-block"><code>${lines.map((line, i) => html`<span class="code-ln">${i + 1}</span>${line}${i === lastIdx ? '' : '\n'}`)}</code></pre>
          </div>
        `;
      }
      return html`
        <div class="form-section">
          <label>${key}</label>
          <div class="form-value">${str}</div>
        </div>
      `;
    });

    // Output ports rendered as a list — explicit visibility for "what cases
    // does this node produce?". For nodes without declared outputs we show
    // the implicit single "done" output.
    const outputs = node.outputs ?? [{ id: 'done', label: 'done', kind: 'success' as const }];
    const outputsSection = html`
      <div class="form-section">
        <label>outputs ${outputs.length === 1 ? '' : `· ${outputs.length}`}</label>
        <div class="port-list">
          ${outputs.map(o => html`
            <span class=${'port-row port-row--' + o.kind} title=${o.kind}>
              <span class="port-dot"></span>
              <span class="port-row-label">${o.label}</span>
              <span class="port-row-kind">${o.kind}</span>
            </span>
          `)}
        </div>
      </div>
    `;

    return html`
      <div class="inspector-body">
        <div class="node-card" style="height:auto">
          <div class="node-card-head">
            <span class="icon-tile icon-tile--${tone}">
              <uk-icon icon=${icon} height="14" width="14"></uk-icon>
            </span>
            <span class="node-card-kind">${node.kind.replace('_', ' ')}</span>
          </div>
          <div class="node-card-label">${node.label}</div>
          <div style="font-size:11px;color:var(--muted-foreground);margin-top:4px;line-height:1.4">
            ${node.description}
          </div>
        </div>

        ${node.region
          ? html`<div class="region-banner">
              <span><uk-icon icon="git-branch" height="12" width="12"></uk-icon> In body of <strong>${this._regionMeta(node.region).label}</strong></span>
              <button class="linkish" @click=${() => this._ejectFromRegion(node.id)}>Remove from body</button>
            </div>`
          : nothing}

        ${this._renderControlHelp(node)}

        ${RUNTIME_KINDS.has(node.kind)
          ? ((KIND_FIELDS[node.kind] ?? []).length
              ? (KIND_FIELDS[node.kind] ?? []).map(f => this._renderField(node, f))
              : html`<div style="font-size:12px;color:var(--muted-foreground);font-style:italic;margin-bottom:14px">No parameters.</div>`)
          : html`<div style="font-size:12px;color:var(--muted-foreground);font-style:italic;margin-bottom:14px">
              Not a v0.2 runtime node — read-only.</div>
            ${paramRows}`}

        ${outputsSection}
      </div>
    `;
  }

  // Split "lhs OP rhs" into parts, or null if it can't round-trip to basic
  // (multiple operators, etc.) — caller then forces advanced mode. A bare
  // variable (no operator) is basic-representable with an empty rhs.
  private _parseCondition(expr: string): { lhs: string; op: string; rhs: string } | null {
    const s = expr.trim();
    if (s === '') return { lhs: '', op: '==', rhs: '' };
    for (const op of EXPR_OPS) {
      const i = s.indexOf(op);
      if (i < 0) continue;
      const lhs = s.slice(0, i).trim();
      const rhs = s.slice(i + op.length).trim();
      // Reject if either side still holds an operator (won't round-trip).
      if (EXPR_OPS.some(o => rhs.includes(o)) || EXPR_OPS.some(o => lhs.includes(o))) return null;
      return { lhs, op, rhs };
    }
    return EXPR_OPS.some(o => s.includes(o)) ? null : { lhs: s, op: '==', rhs: '' };
  }

  private _composeCondition(c: { lhs: string; op: string; rhs: string }): string {
    const lhs = c.lhs.trim();
    const rhs = c.rhs.trim();
    if (!lhs) return '';
    return rhs === '' ? lhs : `${lhs} ${c.op} ${rhs}`;
  }

  private _setCondition(id: string, key: string, patch: Partial<{ lhs: string; op: string; rhs: string }>): void {
    const node = this._nodes.find(n => n.id === id);
    const cur = this._parseCondition(String(node?.params?.[key] ?? '')) ?? { lhs: '', op: '==', rhs: '' };
    this._updateNodeParam(id, key, this._composeCondition({ ...cur, ...patch }));
  }

  private _toggleExprMode(id: string): void {
    const next = new Set(this._exprAdvanced);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    this._exprAdvanced = next;
    // Drop the cached Visual model so re-entering Visual re-parses the DSL the
    // user may have just hand-edited in Advanced.
    const groups = new Map(this._condGroups);
    groups.delete(id);
    this._condGroups = groups;
  }

  // Variable names offered in condition/expr autocomplete: the init vars PLUS
  // every variable a node in the graph defines (set_var, compute, loop item/index,
  // try_catch error). Graph-wide (not scope-aware) — a suggestion list, not a
  // guarantee the var is in scope at that node.
  private _varSuggestions(): string[] {
    const out = new Set<string>(MOCK_INIT_VARS.map(v => v.key));
    const add = (v: unknown, fallback?: string) => {
      if (typeof v === 'string' && v) out.add(v);
      else if (fallback) out.add(fallback);
    };
    for (const n of this._nodes) {
      const p = n.params ?? {};
      switch (n.kind) {
        case 'set_var': add(p['name']); break;
        case 'compute': add(p['var']); break;
        case 'loop_for': add(p['item_var'], 'item'); add(p['index_var'], 'index'); break;
        case 'try_catch': add(p['error_var'], 'error'); break;
        // http_request (and any response-capture node) defines its save_as var.
        default: add(p['save_as']); break;
      }
    }
    return [...out];
  }

  // ----- Condition field: Visual AND/OR builder ⇄ Advanced DSL -----

  private _renderConditionField(node: FlowNode, f: FieldDef) {
    const exprStr = String(node.params?.[f.key] ?? '');
    const parseable = dslToGroup(exprStr) !== null; // simple subset → Visual OK
    const advanced = this._exprAdvanced.has(node.id) || !parseable;
    return html`
      <div class="form-section">
        <label>
          <span>${f.label}</span>
          <button class="expr-mode" ?disabled=${advanced && !parseable}
            title=${advanced && !parseable ? 'Uses functions — edit as text' : ''}
            @click=${() => this._toggleExprMode(node.id)}>
            ${advanced ? 'Visual' : 'Advanced'}
          </button>
        </label>
        ${advanced ? this._renderAdvancedExpr(node, f, exprStr) : this._renderVisualExpr(node, f)}
      </div>`;
  }

  private _renderAdvancedExpr(node: FlowNode, f: FieldDef, exprStr: string) {
    return html`
      <textarea id=${'expr-ta-' + node.id} class="form-input" rows="2"
        placeholder="customer.tier == gold AND num.abs(score) > 3"
        .value=${exprStr}
        @change=${(e: Event) => this._updateNodeParam(node.id, f.key, (e.target as HTMLTextAreaElement).value.trim())}></textarea>
      <div class="expr-toolbar">
        <button class="expr-fn-btn"
          @click=${(e: Event) => { e.stopPropagation(); this._exprPickerNode = this._exprPickerNode === node.id ? null : node.id; }}>
          <uk-icon icon="function-square" height="12" width="12"></uk-icon> fn
        </button>
        <span class="expr-hint">AND/OR/NOT · ( ) · == != &lt; &gt; &lt;= &gt;= · ns.fn(…) · dotted vars · "quoted"</span>
      </div>
      ${this._exprPickerNode === node.id ? this._renderExprPicker(node, f) : nothing}`;
  }

  private _renderExprPicker(node: FlowNode, f: FieldDef) {
    if (this._exprCatalog.length === 0) {
      return html`<div class="expr-picker"><div class="expr-hint" style="padding:8px">No functions available.</div></div>`;
    }
    const byNs = new Map<string, ExprFunction[]>();
    for (const fn of this._exprCatalog) {
      (byNs.get(fn.ns) ?? byNs.set(fn.ns, []).get(fn.ns)!).push(fn);
    }
    return html`
      <div class="expr-picker">
        ${[...byNs.entries()].map(([ns, fns]) => html`
          <div class="expr-picker-ns">${ns}</div>
          ${fns.map(fn => html`
            <button class="expr-picker-item" title=${fn.summary} @click=${() => this._insertFn(node, f, fn)}>
              ${fn.signature}
            </button>`)}
        `)}
      </div>`;
  }

  private _insertFn(node: FlowNode, f: FieldDef, fn: ExprFunction): void {
    const insert = `${fn.ns}.${fn.name}()`;
    const ta = this.shadowRoot?.getElementById('expr-ta-' + node.id) as HTMLTextAreaElement | null;
    // Read the live textarea, not node.params — the textarea only commits on
    // change/blur, so unblurred typing would otherwise be clobbered on insert.
    const cur = ta ? ta.value : String(node.params?.[f.key] ?? '');
    let next: string;
    let caret: number;
    if (ta && ta.selectionStart != null) {
      const s = ta.selectionStart;
      next = cur.slice(0, s) + insert + cur.slice(ta.selectionEnd ?? s);
      caret = s + insert.length - 1; // place cursor inside the ()
    } else {
      next = cur + insert;
      caret = next.length - 1;
    }
    this._updateNodeParam(node.id, f.key, next);
    this._exprPickerNode = null;
    void this.updateComplete.then(() => {
      const t = this.shadowRoot?.getElementById('expr-ta-' + node.id) as HTMLTextAreaElement | null;
      if (t) { t.focus(); t.setSelectionRange(caret, caret); }
    });
  }

  private _condGroupFor(node: FlowNode, key: string): Group {
    let g = this._condGroups.get(node.id);
    if (!g) {
      g = dslToGroup(String(node.params?.[key] ?? '')) ?? newGroup('AND');
      this._condGroups.set(node.id, g);
    }
    return g;
  }

  private _touchGroup(node: FlowNode, f: FieldDef): void {
    this._condGroups = new Map(this._condGroups); // new ref → re-render
    this._updateNodeParam(node.id, f.key, groupToDsl(this._condGroups.get(node.id)!));
  }

  private _renderVisualExpr(node: FlowNode, f: FieldDef) {
    const group = this._condGroupFor(node, f.key);
    const dsl = groupToDsl(group);
    return html`
      ${this._renderGroup(node, f, group, 0)}
      <datalist id="or-var-list">${this._varSuggestions().map(s => html`<option value=${s}></option>`)}</datalist>
      <div class="expr-preview">
        <code>${dsl || '(empty — add a condition)'}</code>
        <button class="expr-copy" title="Copy DSL" ?disabled=${!dsl}
          @click=${() => navigator.clipboard?.writeText(dsl)}><uk-icon icon="copy" height="12" width="12"></uk-icon></button>
      </div>`;
  }

  private _renderGroup(node: FlowNode, f: FieldDef, group: Group, depth: number): unknown {
    return html`
      <div class=${'cond-group' + (depth > 0 ? ' cond-group--nested' : '')}>
        <div class="cond-group-head">
          <div class="cond-andor">
            <button class=${group.op === 'AND' ? 'on' : ''} @click=${() => { group.op = 'AND'; this._touchGroup(node, f); }}>AND</button>
            <button class=${group.op === 'OR' ? 'on' : ''} @click=${() => { group.op = 'OR'; this._touchGroup(node, f); }}>OR</button>
          </div>
          ${depth > 0 ? html`<label class="cond-not"><input type="checkbox" .checked=${!!group.not}
            @change=${(e: Event) => { group.not = (e.target as HTMLInputElement).checked; this._touchGroup(node, f); }}> NOT</label>` : nothing}
        </div>
        ${group.children.map((ch: CondNode, i: number) => html`
          <div class="cond-child">
            ${ch.kind === 'group' ? this._renderGroup(node, f, ch, depth + 1) : this._renderCmpRow(node, f, ch)}
            <button class="cond-rm" title="Remove" @click=${() => { group.children.splice(i, 1); this._touchGroup(node, f); }}>
              <uk-icon icon="x" height="12" width="12"></uk-icon>
            </button>
          </div>`)}
        <div class="cond-add">
          <button @click=${() => { group.children.push(newComparison()); this._touchGroup(node, f); }}>+ condition</button>
          <button @click=${() => { group.children.push(newGroup('AND')); this._touchGroup(node, f); }}>+ group</button>
        </div>
      </div>`;
  }

  private _renderCmpRow(node: FlowNode, f: FieldDef, c: Comparison) {
    return html`
      <div class="cond-builder">
        <input class="form-input cond-lhs" list="or-var-list" placeholder="variable" .value=${c.lhs}
          @input=${(e: Event) => { c.lhs = (e.target as HTMLInputElement).value; this._touchGroup(node, f); }}>
        <select class="form-input cond-op" @change=${(e: Event) => {
          const val = (e.target as HTMLSelectElement).value;
          if (val === 'truthy') { c.mode = 'truthy'; } else { c.mode = 'cmp'; c.op = val as Comparison['op']; }
          this._touchGroup(node, f);
        }}>
          <option value="truthy" ?selected=${c.mode === 'truthy'}>is set</option>
          ${COND_OPS.map(o => html`<option value=${o} ?selected=${c.mode === 'cmp' && c.op === o}>${o}</option>`)}
        </select>
        ${c.mode === 'truthy'
          ? html`<span class="cond-truthy">truthy</span>`
          : html`<input class="form-input" placeholder="value" .value=${c.rhs}
              @input=${(e: Event) => { c.rhs = (e.target as HTMLInputElement).value; this._touchGroup(node, f); }}>`}
      </div>`;
  }

  private _renderField(node: FlowNode, f: FieldDef) {
    const v = node.params?.[f.key];
    if (f.type === 'condition') {
      return this._renderConditionField(node, f);
    }
    if (f.type === 'cases') {
      const arr = Array.isArray(v) ? (v as string[]) : [];
      return html`
        <div class="form-section">
          <label>${f.label}</label>
          <textarea class="form-input" rows="3" placeholder="one case value per line"
            .value=${arr.join('\n')}
            @change=${(e: Event) => {
              const lines = (e.target as HTMLTextAreaElement).value.split('\n').map(s => s.trim()).filter(Boolean);
              this._updateNodeParam(node.id, 'cases', [...new Set(lines)]);
            }}></textarea>
        </div>`;
    }
    if (f.type === 'select') {
      return html`
        <div class="form-section">
          <label>${f.label}</label>
          <select class="form-input"
            @change=${(e: Event) => this._updateNodeParam(node.id, f.key, (e.target as HTMLSelectElement).value)}>
            <option value="" ?selected=${v === undefined || v === ''}>—</option>
            ${(f.options ?? []).map(o => html`<option value=${o} ?selected=${v === o}>${o}</option>`)}
          </select>
        </div>`;
    }
    if (f.type === 'catalog') {
      return this._renderCatalogField(node, f);
    }
    if (f.type === 'expr') {
      // A value expression (not a boolean) — the Advanced DSL editor + function
      // autocomplete, without the AND/OR Visual builder.
      return html`
        <div class="form-section">
          <label>${f.label}</label>
          ${this._renderAdvancedExpr(node, f, String(v ?? ''))}
          ${f.hint ? html`<div class="field-hint">${f.hint}</div>` : nothing}
        </div>`;
    }
    if (f.type === 'textarea') {
      return html`
        <div class="form-section">
          <label>${f.label}</label>
          <textarea class="form-input" rows="4" spellcheck="false"
            placeholder=${f.placeholder ?? ''}
            .value=${v === undefined || v === null ? '' : String(v)}
            @input=${(e: Event) => {
              const raw = (e.target as HTMLTextAreaElement).value;
              this._updateNodeParam(node.id, f.key, raw === '' ? undefined : raw);
            }}></textarea>
          ${f.hint ? html`<div class="field-hint">${f.hint}</div>` : nothing}
        </div>`;
    }
    return html`
      <div class="form-section">
        <label>${f.label}</label>
        <input class="form-input" type=${f.type === 'number' ? 'number' : 'text'}
          placeholder=${f.placeholder ?? ''}
          .value=${v === undefined || v === null ? '' : String(v)}
          @input=${(e: Event) => {
            const raw = (e.target as HTMLInputElement).value;
            this._updateNodeParam(node.id, f.key, f.type === 'number' ? (raw === '' ? undefined : Number(raw)) : raw);
          }}>
        ${f.hint ? html`<div class="field-hint">${f.hint}</div>` : nothing}
      </div>`;
  }

  // Single search-as-you-type combobox for a catalog reference: the field IS the
  // search input (type to filter the list below), so picking an existing
  // skill/queue/adapter code is one control — no separate trigger + search box.
  private _renderCatalogField(node: FlowNode, f: FieldDef) {
    const source = f.source!;
    const list = this._catalogRefs[source];
    const current = node.params?.[f.key] === undefined ? '' : String(node.params?.[f.key]);
    const key = `${node.id}:${f.key}`;
    const open = this._catalogPicker === key;
    const selected = list.find(r => r.code === current);
    const known = current === '' || !!selected;
    const display = current ? (selected ? `${selected.code} — ${selected.name}` : current) : '';
    const q = this._catalogQuery.trim().toLowerCase();
    const filtered = open && q
      ? list.filter(r => r.code.toLowerCase().includes(q) || r.name.toLowerCase().includes(q))
      : list;
    const pick = (code: string | undefined) => {
      this._updateNodeParam(node.id, f.key, code);
      this._catalogPicker = null;
      this._catalogQuery = '';
    };
    return html`
      <div class="form-section">
        <label>${f.label}</label>
        <div class="catalog-combo">
          <input
            class=${'form-input catalog-input' + (known ? '' : ' catalog-input--unknown')}
            type="text"
            placeholder=${open || !current ? `Search ${source}s…` : ''}
            title=${known ? '' : 'Not in the catalog — pick a valid one'}
            .value=${open ? this._catalogQuery : display}
            @focus=${() => { this._catalogPicker = key; this._catalogQuery = ''; }}
            @input=${(e: Event) => { this._catalogPicker = key; this._catalogQuery = (e.target as HTMLInputElement).value; }}
            @blur=${() => { setTimeout(() => { if (this._catalogPicker === key) this._catalogPicker = null; }, 120); }}>
          ${current && !open
            ? html`<button class="catalog-clear" title="Clear"
                @mousedown=${(e: Event) => { e.preventDefault(); pick(undefined); }}>
                <uk-icon icon="x" height="12" width="12"></uk-icon></button>`
            : nothing}
          ${open ? html`
            <div class="catalog-list">
              ${list.length === 0
                ? html`<div class="catalog-empty">No ${source}s in the catalog.</div>`
                : filtered.length === 0
                  ? html`<div class="catalog-empty">No match.</div>`
                  : filtered.map(r => html`
                      <button class=${'catalog-item' + (r.code === current ? ' on' : '')}
                        @mousedown=${(e: Event) => { e.preventDefault(); pick(r.code); }}>
                        <span class="catalog-item-code">${r.code}</span>
                        <span class="catalog-item-name">${r.name}</span>
                      </button>`)}
            </div>` : nothing}
        </div>
      </div>`;
  }

  private _updateNodeParam(id: string, key: string, value: unknown): void {
    this._nodes = this._nodes.map(n => {
      if (n.id !== id) return n;
      const params = { ...(n.params ?? {}) } as Record<string, unknown>;
      if (value === undefined) delete params[key];
      else params[key] = value;
      const next = { ...n, params: params as FlowNode['params'] };
      if (n.kind === 'switch_case' && key === 'cases') next.outputs = outputsForNode(next);
      if (n.kind === 'parallel' && key === 'branches') next.outputs = outputsForNode(next);
      return next;
    });
    if (key === 'cases' || key === 'branches') {
      // Reducing branches drops the now-missing body:N ports + their edges; the
      // re-derivation then drops nodes orphaned out of a region that's gone.
      this._edges = this._pruneEdges(this._nodes, this._edges);
      this._recomputeRegions();
    }
    this._markDirty();
  }

  override render() {
    return this._loadTask.render({
      pending: () => html`<div class="builder-status">Loading flow…</div>`,
      error: (err) => html`
        <div class="builder-status builder-status--error">
          <strong>Failed to load flow.</strong>
          <span>${this._errText(err, String(err))}</span>
          <button class="toolbar-btn" @click=${() => this._loadTask.run()}>Retry</button>
        </div>
      `,
      complete: () => this._renderBuilder(),
    });
  }

  private _renderBuilder() {
    const flow = this._flow;
    const nodes = this._nodes;
    const edges = this._edges;
    const isSim = this._simMode === 'sim';
    const selected = nodes.find(n => n.id === this._selectedNodeId) ?? null;

    return html`
      ${this._actionToast
        ? html`<div class="action-toast action-toast--${this._actionTone}" role="status">${this._actionToast}</div>`
        : nothing}
      <div class=${isSim ? 'toolbar toolbar--sim' : 'toolbar'}>
        <button class="toolbar-back" title="Back to flows" @click=${() => this._navigate(`/orgs/${this.orgId}/flows`)}>
          <uk-icon icon="arrow-left" height="16" width="16"></uk-icon>
        </button>
        ${this._isCreate
          ? html`
              <div class="toolbar-title toolbar-title--create">
                <input class="uk-input uk-form-small" placeholder="flow_code" .value=${this._codeDraft}
                  @input=${(e: Event) => { this._codeDraft = (e.target as HTMLInputElement).value; }} aria-label="Flow code" />
                <input class="uk-input uk-form-small" placeholder="Flow name" .value=${this._nameDraft}
                  @input=${(e: Event) => { this._nameDraft = (e.target as HTMLInputElement).value; }} aria-label="Flow name" />
              </div>
            `
          : html`
              <div class="toolbar-title">
                <strong>${flow.name}</strong>
                <span class="meta">${flow.code}  ·  v${flow.version}</span>
              </div>
            `}
        <span class="status-pill">${flow.status}</span>
        ${isSim ? html`<span class="sim-badge"><uk-icon icon="play-circle" height="11" width="11"></uk-icon> SIM</span>` : nothing}

        <div class="toolbar-spacer"></div>

        ${isSim
          ? this._renderSimControls()
          : html`
              <button class="toolbar-btn toolbar-btn--icon" title="Undo (⌘Z)" ?disabled=${!this._canUndo}
                @click=${() => this._undo()}>
                <uk-icon icon="undo-2" height="14" width="14"></uk-icon>
              </button>
              <button class="toolbar-btn toolbar-btn--icon" title="Redo (⌘⇧Z)" ?disabled=${!this._canRedo}
                @click=${() => this._redo()}>
                <uk-icon icon="redo-2" height="14" width="14"></uk-icon>
              </button>
              <button class="toolbar-btn" @click=${this._validateFlow}>
                <uk-icon icon="check-circle" height="14" width="14"></uk-icon>
                Validate
              </button>
              <button class="toolbar-btn" title="Auto-arrange nodes" ?disabled=${this._nodes.length === 0}
                @click=${() => this._doAutoArrange()}>
                <uk-icon icon="layout-grid" height="14" width="14"></uk-icon>
                Arrange
              </button>
              <button class="toolbar-btn" title="Compare draft against the published version (Layer 3)"
                @click=${() => this._flashAction('Diff vs published lands in Layer 3.', 'warn')}>
                <uk-icon icon="history" height="14" width="14"></uk-icon>
                Diff vs published
              </button>
              <button class="toolbar-btn" ?disabled=${this._saving} @click=${this._saveDraft}>
                <uk-icon icon="save" height="14" width="14"></uk-icon>
                ${this._saving ? 'Saving…' : this._isCreate ? 'Create flow' : 'Save draft'}
              </button>
            `}

        <button
          class=${isSim ? 'toolbar-btn toolbar-btn--sim-active' : 'toolbar-btn'}
          @click=${this._toggleSimMode}
          title=${isSim ? 'Exit simulator' : 'Run on the canvas'}
        >
          <uk-icon icon=${isSim ? 'x' : 'play'} height="14" width="14"></uk-icon>
          ${isSim ? 'Exit sim' : 'Simulate'}
        </button>
        ${isSim ? nothing : html`
          <button class="toolbar-btn toolbar-btn--primary" @click=${() => { this._publishOpen = !this._publishOpen; }}>
            <uk-icon icon="rocket" height="14" width="14"></uk-icon>
            Publish
          </button>
        `}
      </div>

      ${this._publishOpen && !isSim ? this._renderPublishPopover() : nothing}

      <div class="body" style=${`grid-template-columns: ${this._paletteCollapsed ? '34px' : '220px'} 1fr ${this._inspectorCollapsed ? '34px' : '344px'}`}>
        ${this._paletteCollapsed
          ? this._renderRail('Nodes', 'chevrons-right', () => { this._paletteCollapsed = false; })
          : this._renderPalette(isSim)}

        <main class=${isSim ? 'pane canvas-wrap canvas-wrap--sim' : 'pane canvas-wrap'}
          style=${`background-position:${this._panX}px ${this._panY}px;background-size:${24 * this._zoom}px ${24 * this._zoom}px`}
          @dragover=${isSim ? nothing : this._onCanvasDragOver}
          @dragleave=${isSim ? nothing : this._onCanvasDragLeave}
          @drop=${isSim ? nothing : this._onCanvasDrop}>
          <svg
            class=${'canvas-svg' + (this._panState ? ' is-panning' : '')}
            xmlns="http://www.w3.org/2000/svg"
            @pointerdown=${this._onCanvasPointerDown}
            @pointermove=${this._onCanvasPointerMove}
            @pointerup=${this._onCanvasPointerUp}
            @pointercancel=${this._onCanvasPointerUp}
          >
            <rect class="canvas-bg" x="0" y="0" width="100%" height="100%" fill="transparent"></rect>
            <g class="viewport" transform=${`translate(${this._panX} ${this._panY}) scale(${this._zoom})`}>
              ${this._renderRegions(nodes)}
              ${this._renderEdges(nodes, edges)}
              ${this._renderNodes(nodes)}
              ${this._edgeDraft ? this._renderEdgeDraft() : nothing}
            </g>
          </svg>
          ${this._renderZoomControls()}
          <div class="canvas-strip">
            ${isSim ? html`
              <span><span class="dot-ok">●</span> Step <strong>${this._simStep < 0 ? 'ready' : (this._simStep + 1) + ' / ' + this._activeTrace.length}</strong></span>
              <span>Trace <strong>${this._currentStep?.id ?? '—'}</strong></span>
              <span>Scenario <strong>${this._simScenario === 'fail' ? 'failure path' : 'success path'}</strong></span>
            ` : this._validation
              ? (this._validation.valid
                  ? html`<span><span class="dot-ok">●</span> Validated — <strong>no issues</strong></span>`
                  : html`<span><span class="dot-warn">●</span> <strong>${this._validation.issues.length}</strong> validation ${this._validation.issues.length === 1 ? 'issue' : 'issues'}</span>`)
              : this._nodes.length === 0
              ? html`<span><span class="dot-warn">●</span> Empty — drag a node from the palette to start</span>`
              : html`
                <span><strong>${this._nodes.length}</strong> ${this._nodes.length === 1 ? 'node' : 'nodes'}</span>
                <span><strong>${this._edges.length}</strong> ${this._edges.length === 1 ? 'edge' : 'edges'}</span>
              `}
            <div style="flex:1"></div>
            <span>Zoom: <strong>${Math.round(this._zoom * 100)}%</strong></span>
            ${isSim || !this._loaded?.updated_at ? nothing
              : html`<span>Last saved <strong>${this._relTime(this._loaded.updated_at)}</strong></span>`}
          </div>
        </main>

        ${this._inspectorCollapsed && !isSim
          ? this._renderRail('Inspector', 'chevrons-left', () => { this._inspectorCollapsed = false; })
          : html`
            <aside class="pane inspector">
              <div class="pane-header">
                <span>${isSim && this._currentStep ? 'Step I/O' : 'Inspector'}</span>
                ${isSim ? nothing : html`
                  <button class="pane-collapse" title="Collapse inspector" @click=${() => { this._inspectorCollapsed = true; }}>
                    <uk-icon icon="chevrons-right" height="14" width="14"></uk-icon>
                  </button>`}
              </div>
              ${!isSim && this._validation ? this._renderValidationPanel() : nothing}
              ${isSim ? this._renderSimOutcomePicker() : nothing}
              ${isSim && this._currentStep
                ? this._renderSimInspector(this._currentStep)
                : this._renderInspector(selected)}
            </aside>
          `}
      </div>

      ${isSim ? this._renderSimRunPanel() : nothing}
    `;
  }

  private _toggleGroup(label: string): void {
    const next = new Set(this._collapsedGroups);
    if (next.has(label)) next.delete(label);
    else next.add(label);
    this._collapsedGroups = next;
  }

  private _renderPalette(isSim: boolean) {
    const q = this._paletteQuery.trim().toLowerCase();
    const searching = q.length > 0;
    const groups = PALETTE
      .map(group => {
        const items = searching
          ? group.items.filter(p =>
              p.label.toLowerCase().includes(q) ||
              p.desc.toLowerCase().includes(q) ||
              p.kind.toLowerCase().includes(q))
          : group.items;
        // Runtime-supported nodes float to the top of each group; the "Soon"
        // placeholders sink below so the draggable set reads first.
        const sorted = [...items].sort(
          (a, b) => Number(RUNTIME_KINDS.has(b.kind)) - Number(RUNTIME_KINDS.has(a.kind)),
        );
        return { group, items: sorted };
      })
      .filter(g => g.items.length > 0);

    return html`
      <aside class="pane palette">
        <div class="pane-header">
          <span>${isSim ? 'Nodes (read-only)' : 'Nodes'}</span>
          ${isSim ? nothing : html`
            <button class="pane-collapse" title="Collapse palette" @click=${() => { this._paletteCollapsed = true; }}>
              <uk-icon icon="chevrons-left" height="14" width="14"></uk-icon>
            </button>`}
        </div>
        <div class="palette-search">
          <div class="palette-search-box">
            <uk-icon icon="search" height="14" width="14"></uk-icon>
            <input
              class="palette-search-input"
              type="text"
              placeholder="Search nodes…"
              aria-label="Search nodes"
              .value=${this._paletteQuery}
              @input=${(e: Event) => { this._paletteQuery = (e.target as HTMLInputElement).value; }}
            />
            ${searching ? html`
              <button class="palette-search-clear" title="Clear search" aria-label="Clear search"
                @click=${() => { this._paletteQuery = ''; }}>
                <uk-icon icon="x" height="14" width="14"></uk-icon>
              </button>
            ` : nothing}
          </div>
        </div>
        <div class="palette-list">
          ${groups.map(({ group, items }) => {
            const collapsed = !searching && this._collapsedGroups.has(group.label);
            return html`
              <button class="palette-group-label" aria-expanded=${!collapsed}
                @click=${() => this._toggleGroup(group.label)}>
                <uk-icon class="chev" icon=${collapsed ? 'chevron-right' : 'chevron-down'} height="12" width="12"></uk-icon>
                <span>${group.label}</span>
                <span class="grp-count">${items.length}</span>
              </button>
              ${collapsed ? nothing : items.map(p => {
                const supported = RUNTIME_KINDS.has(p.kind);
                const draggable = !isSim && supported;
                return html`
                  <div class="palette-item ${draggable ? '' : 'palette-item--disabled'}"
                    title=${isSim ? p.desc : supported ? 'Drag onto the canvas to add' : 'Not supported by the v0.2 runtime yet'}
                    draggable=${draggable}
                    @dragstart=${(e: DragEvent) => this._onPaletteDragStart(e, p.kind)}>
                    <span class="icon-tile icon-tile--${p.tone}">
                      <uk-icon icon=${p.icon} height="14" width="14"></uk-icon>
                    </span>
                    <span class="palette-item-text">
                      <span class="palette-item-label">${p.label}</span>
                      <span class="palette-item-desc">${p.desc}</span>
                    </span>
                    ${supported ? nothing : html`<span class="palette-soon" title="Planned — the v0.2 runtime doesn't execute this node yet">Soon</span>`}
                  </div>
                `;
              })}
            `;
          })}
          ${groups.length === 0 ? html`
            <div class="palette-empty">No nodes match “${this._paletteQuery}”.</div>
          ` : nothing}
        </div>
      </aside>
    `;
  }

  private _renderZoomControls() {
    if (this._simMode === 'sim') return nothing;
    return html`
      <div class="zoom-controls">
        <button title="Zoom out" @click=${() => this._zoomButton(1 / 1.2)}><uk-icon icon="minus" height="14" width="14"></uk-icon></button>
        <span class="zoom-pct">${Math.round(this._zoom * 100)}%</span>
        <button title="Zoom in" @click=${() => this._zoomButton(1.2)}><uk-icon icon="plus" height="14" width="14"></uk-icon></button>
        <button title="Fit to content" @click=${() => this._fitView()}><uk-icon icon="maximize" height="14" width="14"></uk-icon></button>
        <button title="Reset view" @click=${() => this._resetView()}><uk-icon icon="locate-fixed" height="14" width="14"></uk-icon></button>
      </div>
    `;
  }

  private _renderPublishPopover() {
    return html`
      <div class="publish-popover" role="dialog" aria-label="Publish flow">
        <div class="publish-popover-title">Publish to entry point</div>
        <label class="publish-field">
          <span>Channel</span>
          <input type="text" .value=${this._publishChannel}
            @input=${(e: Event) => { this._publishChannel = (e.target as HTMLInputElement).value; }} />
        </label>
        <label class="publish-field">
          <span>Entry code</span>
          <input type="text" .value=${this._publishEntry}
            @input=${(e: Event) => { this._publishEntry = (e.target as HTMLInputElement).value; }} />
        </label>
        <div class="publish-actions">
          <button class="toolbar-btn" @click=${() => { this._publishOpen = false; }}>Cancel</button>
          <button class="toolbar-btn toolbar-btn--primary" ?disabled=${this._publishing} @click=${this._publishFlow}>
            ${this._publishing ? 'Publishing…' : 'Publish'}
          </button>
        </div>
      </div>
    `;
  }

  private _renderValidationPanel() {
    const issues = this._validation?.issues ?? [];
    if (this._validation?.valid) {
      return html`
        <div class="validation-panel validation-panel--ok" role="status">
          <div class="validation-panel-head validation-panel-head--ok">
            <uk-icon icon="check-circle" height="13" width="13"></uk-icon>
            Validated — no issues
            <button class="validation-panel-close" title="Dismiss" @click=${() => { this._validation = null; }}>
              <uk-icon icon="x" height="13" width="13"></uk-icon>
            </button>
          </div>
        </div>
      `;
    }
    return html`
      <div class="validation-panel" role="status">
        <div class="validation-panel-head">
          <uk-icon icon="alert-triangle" height="13" width="13"></uk-icon>
          <strong>${issues.length}</strong> validation ${issues.length === 1 ? 'issue' : 'issues'}
          <button class="validation-panel-close" title="Dismiss" @click=${() => { this._validation = null; }}>
            <uk-icon icon="x" height="13" width="13"></uk-icon>
          </button>
        </div>
        <ul class="validation-list">
          ${issues.map(i => html`
            <li>
              <code>${i.code}</code>
              ${i.node_id ? html`<span class="validation-loc">node ${i.node_id}${i.field ? '.' + i.field : ''}</span>` : nothing}
              ${i.edge_id ? html`<span class="validation-loc">edge ${i.edge_id}</span>` : nothing}
              <span class="validation-msg">${i.message}</span>
            </li>
          `)}
        </ul>
      </div>
    `;
  }

  // ----- Sim toolbar controls -----
  private _renderSimControls() {
    const max = this._activeTrace.length - 1;
    const atStart = this._simStep < 0;
    const atEnd = this._simStep >= max;
    const paused = this._pausedForInput;
    return html`
      <span
        class=${'sim-scenario-badge sim-scenario-badge--' + (this._simScenario === 'fail' ? 'fail' : 'success')}
        title="Scenario is set by the active test case. Switch via Test cases tab → Run."
        aria-label=${(this._simScenario === 'fail' ? 'Failure' : 'Success') + ' path. Read-only — scenario is set by the active test case. To switch scenarios, run a test case from the Test cases tab.'}
      >
        <uk-icon icon=${this._simScenario === 'fail' ? 'circle-x' : 'circle-check'} height="11" width="11"></uk-icon>
        ${this._simScenario === 'fail' ? 'failure path' : 'success path'}
      </span>

      <div class="sim-playback" role="group" aria-label="Playback controls">
        <button class="sim-pb-btn" title="Re-run with current init vars + outcomes" ?disabled=${this._simRunning} @click=${() => void this._runSimulation()}>
          <uk-icon icon="rotate-ccw" height="14" width="14"></uk-icon>
        </button>
        <button class="sim-pb-btn" title="Step back" ?disabled=${atStart} @click=${() => this._stepBy(-1)}>
          <uk-icon icon="chevron-left" height="14" width="14"></uk-icon>
        </button>
        <button
          class="sim-pb-btn sim-pb-btn--primary"
          title=${paused ? 'Use Submit & continue below — sim is paused waiting for input' : atEnd ? 'Restart' : 'Step forward'}
          ?disabled=${paused}
          @click=${() => { if (atEnd) this._restartSim(); else this._stepBy(1); }}
        >
          <uk-icon icon=${atEnd ? 'rotate-ccw' : 'play'} height="14" width="14"></uk-icon>
        </button>
        <button class="sim-pb-btn" title="Run to end" ?disabled=${atEnd || paused} @click=${this._runAllSim}>
          <uk-icon icon="fast-forward" height="14" width="14"></uk-icon>
        </button>
      </div>

      <span class="sim-step-counter">
        Step
        ${this._simStep < 0
          ? html`<strong>ready</strong>`
          : html`<strong>${this._simStep + 1}</strong> <span style="color:var(--muted-foreground)">/ ${this._activeTrace.length}</span>`}
      </span>

      <button
        class="sim-save-btn"
        title="Save current run (init vars + captured inputs) as a regression test case"
        @click=${this._saveCurrentAsTestCase}
        ?disabled=${this._simStep < 0}
      >
        <uk-icon icon="bookmark-plus" height="13" width="13"></uk-icon>
        Save run
      </button>
    `;
  }

  // ----- Sim run panel (bottom) -----
  private _renderSimRunPanel() {
    const bag = computeVarBag(this._simStep, this._activeTrace, this._initVars);
    const currentStepId = this._currentStep?.id ?? 'init';
    return html`
      <section class="run-panel" aria-label="Simulator run panel">
        <div class="run-panel-col run-panel-col--vars">
          <div class="run-panel-header">
            <uk-icon icon="zap" height="13" width="13"></uk-icon>
            Init vars
            <span class="run-panel-count">${this._initVars.length}</span>
          </div>
          <div class="initvar-list">
            ${this._initVars.map((v, i) => html`
              <div class="initvar-row">
                <div class="initvar-row-head">
                  <span class="initvar-key">${v.key}</span>
                  <span class=${v.source === 'trigger' ? 'initvar-tag initvar-tag--trigger' : 'initvar-tag'}>${v.source}</span>
                </div>
                <input
                  class="initvar-input"
                  .value=${v.value}
                  title="Edit and press Enter (or click away) to re-simulate with this value"
                  @input=${(e: Event) => {
                    const val = (e.target as HTMLInputElement).value;
                    this._initVars = this._initVars.map((x, j) => (j === i ? { ...x, value: val } : x));
                  }}
                  @keydown=${(e: KeyboardEvent) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); }}
                  @change=${() => void this._runSimulation(true)}
                />
              </div>
            `)}
          </div>
        </div>

        <div class="run-panel-col run-panel-col--center">
          ${this._renderSimStepCard()}
        </div>

        <div class="run-panel-col">
          <div class="run-panel-tabs" role="tablist" aria-label="Right panel">
            <button
              class=${this._rightTab === 'varbag' ? 'rp-tab rp-tab--active' : 'rp-tab'}
              @click=${() => { this._rightTab = 'varbag'; }}
              role="tab"
            >
              <uk-icon icon="database" height="12" width="12"></uk-icon>
              Variable bag
              <span class="run-panel-count">${bag.length}</span>
            </button>
            <button
              class=${this._rightTab === 'tests' ? 'rp-tab rp-tab--active' : 'rp-tab'}
              @click=${() => { this._rightTab = 'tests'; }}
              role="tab"
            >
              <uk-icon icon="flask-conical" height="12" width="12"></uk-icon>
              Test cases
              <span class="run-panel-count">${this._testCases.length}</span>
            </button>
            <div style="flex:1"></div>
            ${this._rightTab === 'varbag' ? html`
              <span style="font-size:10px;color:var(--muted-foreground);font-weight:500;letter-spacing:0;text-transform:none">
                after step ${this._simStep < 0 ? '—' : (this._simStep + 1) + ' / ' + this._activeTrace.length}
              </span>
            ` : html`
              <button class="rp-tab-action" title="Save current run as a new test case" @click=${this._saveCurrentAsTestCase}>
                <uk-icon icon="bookmark-plus" height="12" width="12"></uk-icon>
                Save run
              </button>
            `}
          </div>

          ${this._rightTab === 'varbag'
            ? html`
                <div class="varbag-list">
                  ${bag.map(v => html`
                    <div class=${v.set_at_step === currentStepId ? 'varbag-row varbag-row--just-set' : 'varbag-row'}>
                      <span class="varbag-key">${v.key}</span>
                      <span
                        class="varbag-value"
                        title=${typeof v.value === 'string' ? v.value : JSON.stringify(v.value)}
                      >${typeof v.value === 'string' ? v.value : JSON.stringify(v.value)}</span>
                      <span class="varbag-source" title="Set at ${v.set_at_node_label}">
                        ${v.set_at_step === 'init' ? 'init' : v.set_at_step}
                      </span>
                    </div>
                  `)}
                </div>
              `
            : this._renderTestCases()}

          ${this._saveToast ? html`
            <div class="rp-toast">
              <uk-icon icon="check-circle" height="12" width="12"></uk-icon>
              ${this._saveToast}
            </div>
          ` : nothing}
        </div>
      </section>
    `;
  }

  // ----- Test cases panel -----

  private _renderTestCases() {
    if (this._testCases.length === 0) {
      return html`
        <div class="testcase-empty">
          <uk-icon icon="flask-conical" height="22" width="22"></uk-icon>
          <p>No saved test cases yet. Step through a run, then <strong>Save run</strong> to capture this scenario for regression replay.</p>
        </div>
      `;
    }
    return html`
      <div class="testcase-list">
        ${this._testCases.map(tc => html`
          <div class="testcase-row">
            <span class=${'testcase-dot testcase-dot--' + tc.last_outcome} title=${tc.last_outcome}></span>
            <div class="testcase-text">
              <div class="testcase-name" title=${tc.description}>${tc.name}</div>
              <div class="testcase-meta">
                <span class=${'tc-scenario-pill tc-scenario-pill--' + tc.scenario}>${tc.scenario}</span>
                · ${Object.keys(tc.captured_inputs).length} input${Object.keys(tc.captured_inputs).length === 1 ? '' : 's'}
                · ${Object.keys(tc.init_var_overrides).length} override${Object.keys(tc.init_var_overrides).length === 1 ? '' : 's'}
                · ${tc.last_outcome === 'never_run' ? 'never run' : tc.last_outcome}
              </div>
            </div>
            <button
              class="testcase-run"
              title="Replay this case — applies init vars + captured inputs + jumps to end"
              @click=${() => this._runTestCase(tc)}
            >
              <uk-icon icon="play" height="11" width="11"></uk-icon>
              Run
            </button>
            <button
              class="testcase-del"
              title="Delete test case"
              @click=${() => this._deleteTestCase(tc.id)}
            >
              <uk-icon icon="trash-2" height="11" width="11"></uk-icon>
            </button>
          </div>
        `)}
      </div>
    `;
  }

  // Control-flow context chips for a trace step: which body region it ran in,
  // the loop iteration / parallel branch, and whether a try_catch caught it.
  private _renderStepNesting(step: TraceStep) {
    const chips = [];
    if (step.region) {
      chips.push(html`<span class="nest-chip nest-chip--region">
        <uk-icon icon="git-branch" height="9" width="9"></uk-icon>${this._regionMeta(step.region).label}</span>`);
    }
    if (step.iteration != null) {
      chips.push(html`<span class="nest-chip"><uk-icon icon="repeat" height="9" width="9"></uk-icon>iter ${step.iteration}</span>`);
    }
    if (step.branch != null) {
      chips.push(html`<span class="nest-chip"><uk-icon icon="rows-3" height="9" width="9"></uk-icon>branch ${step.branch}</span>`);
    }
    if (step.caught) {
      chips.push(html`<span class="nest-chip nest-chip--caught"><uk-icon icon="shield-check" height="9" width="9"></uk-icon>caught</span>`);
    }
    return chips.length ? html`<div class="nest-row">${chips}</div>` : nothing;
  }

  private _renderSimStepCard() {
    const step = this._currentStep;
    if (!step) {
      return html`
        <div class="run-panel-header"><uk-icon icon="rocket-launch" height="13" width="13"></uk-icon> Ready to run</div>
        <div class="sim-runcard sim-runcard--empty">
          <uk-icon icon="play-circle" height="28" width="28"></uk-icon>
          <p>Press <strong>▶ Step forward</strong> to execute the next node, or <strong>⏭ Run to end</strong> for the whole flow.</p>
        </div>
      `;
    }
    const tone = this._toneFor(step.node_kind);
    const icon = this._iconFor(step.node_kind);
    // Capture nodes get their single value input here in the bottom step card.
    const isInputNode = WAIT_INPUT_KINDS.has(step.node_kind);
    return html`
      <div class="run-panel-header">
        <uk-icon icon="activity" height="13" width="13"></uk-icon>
        Now executing
      </div>
      <div class=${step.status === 'fail' && step.caught ? 'sim-runcard sim-runcard--paused'
        : step.status === 'fail' ? 'sim-runcard sim-runcard--fail'
        : 'sim-runcard'}>
        <div class="sim-runcard-head">
          <span class="icon-tile icon-tile--${tone}">
            <uk-icon icon=${icon} height="12" width="12"></uk-icon>
          </span>
          <div>
            <div style="font-size:13px;font-weight:600;color:var(--foreground)">${step.label}</div>
            <div style="font-size:11px;color:var(--muted-foreground);font-family:var(--uk-font-monospace, monospace)">
              ${step.node_kind.replace('_', ' ')} · +${step.started_at_ms.toFixed(1)}ms · ${step.duration_ms.toFixed(1)}ms
            </div>
            ${this._renderStepNesting(step)}
          </div>
          <span class=${'sim-runcard-status sim-runcard-status--' + step.status}>
            <uk-icon icon=${this._statusIcon(step.status)} height="11" width="11"></uk-icon>
            ${step.status}
          </span>
        </div>
        ${isInputNode ? html`
          <div class="sim-input-control">
            <label class="sim-input-label">Captured value</label>
            <div class="sim-input-row">
              <input class="sim-input-field" type="text"
                placeholder=${OrFlowBuilder._simInputHint(step.node_kind as FlowNodeKind)}
                .value=${this._inputDraft}
                @input=${(e: Event) => { this._inputDraft = (e.target as HTMLInputElement).value; }}
                @keydown=${(e: KeyboardEvent) => { if (e.key === 'Enter') void this._submitNodeInput(step.node_id); }}>
              <button class="sim-input-submit" ?disabled=${this._simRunning}
                @click=${() => void this._submitNodeInput(step.node_id)}>
                <uk-icon icon="corner-down-left" height="13" width="13"></uk-icon> Submit
              </button>
            </div>
            <div class="sim-input-branch">
              ${(this._simNodeInputs[step.node_id] ?? '') !== ''
                ? html`Takes the <span class="branch-tag branch-tag--captured">captured</span> branch.`
                : html`Empty → <span class="branch-tag branch-tag--timeout">timeout</span> branch. Type a value + Submit for <strong>captured</strong>.`}
            </div>
          </div>` : nothing}
        ${step.status === 'fail' && step.caught ? html`
          <div class="sim-runcard-error sim-runcard-error--caught">
            <div style="font-weight:600;font-size:12px;color:color-mix(in oklch, var(--warning) 85%, var(--foreground))">
              <uk-icon icon="shield-check" height="12" width="12"></uk-icon>
              Caught by try / catch — flow continued via the catch path.
            </div>
            <div style="margin-top:4px;color:var(--muted-foreground)">
              ${String((step.outputs as Record<string, unknown>)['error'] ?? step.note ?? 'domain failure')}
            </div>
          </div>
        ` : step.status === 'fail' ? html`
          <div class="sim-runcard-error">
            <div style="font-weight:600;color:var(--destructive);font-size:12px">
              <uk-icon icon="alert-triangle" height="12" width="12"></uk-icon>
              ${String((step.outputs as Record<string, unknown>)['error'] ?? 'Step failed')}
            </div>
            ${step.note ? html`<div style="margin-top:4px;color:var(--muted-foreground)">${step.note}</div>` : ''}
            <div class="sim-runcard-fix-row">
              <button class="fix-btn" title="Open node params in inspector">
                <uk-icon icon="settings-2" height="13" width="13"></uk-icon>
                Edit node params
              </button>
              <button class="fix-btn fix-btn--retry" title="Re-run from this step with current params">
                <uk-icon icon="refresh-ccw" height="13" width="13"></uk-icon>
                Retry from here
              </button>
            </div>
          </div>
        ` : nothing}
      </div>
    `;
  }

  // Input form for wait_input steps. Shown inside the NOW EXECUTING card
  // when the sim has paused waiting for a value (DTMF digit, free text,
  // supervisor choice, external signal). User types or accepts pre-fill
  // then clicks "Submit & continue" to advance.
  private _renderInputForm(step: TraceStep) {
    const p = step.inputs as Record<string, unknown>;
    const prompt = String(p['prompt'] ?? p['signal_name'] ?? 'Provide value to continue');
    const isMultiChoice = step.node_kind === 'manual_approval';
    const choices = (step.outputs as Record<string, unknown>)['choices'] as string[] | undefined ??
      ((step.inputs as Record<string, unknown>)['choices'] as string[] | undefined) ??
      ['retry', 'drop'];
    const placeholderHint = step.node_kind === 'get_dtmf' ? 'Digits…'
      : step.node_kind === 'prompt_text' ? 'Type response…'
      : step.node_kind === 'wait_signal' ? 'Signal payload (JSON or raw)…'
      : 'Choice…';

    return html`
      <div class="input-form">
        <div class="input-form-prompt">
          <uk-icon icon="message-square-quote" height="12" width="12"></uk-icon>
          ${prompt}
        </div>
        ${isMultiChoice
          ? html`
              <div class="input-form-choices" role="radiogroup">
                ${choices.map(c => html`
                  <button
                    class=${this._inputDraft === c ? 'choice-pill choice-pill--active' : 'choice-pill'}
                    @click=${() => { this._inputDraft = c; }}
                  >${c}</button>
                `)}
              </div>
            `
          : html`
              <input
                class="input-form-input"
                .value=${this._inputDraft}
                placeholder=${placeholderHint}
                @input=${(e: Event) => { this._inputDraft = (e.target as HTMLInputElement).value; }}
                @keydown=${(e: KeyboardEvent) => { if (e.key === 'Enter') this._submitInput(); }}
              />
            `}
        <div class="input-form-actions">
          <span class="input-form-hint">
            ${step.node_kind === 'get_dtmf' ? `expects ${p['max_digits']} digit${p['max_digits'] === 1 ? '' : 's'}, ${p['timeout_sec']}s timeout` :
              step.node_kind === 'manual_approval' ? 'supervisor gate' :
              step.node_kind === 'wait_signal' ? 'external signal' :
              'free text'}
          </span>
          <button
            class="input-form-submit"
            ?disabled=${!this._inputDraft.trim()}
            @click=${this._submitInput}
            title=${this._inputDraft.trim() ? 'Submit + advance to next step' : 'Provide a value first'}
          >
            <uk-icon icon="check" height="13" width="13"></uk-icon>
            Submit &amp; continue
          </button>
        </div>
      </div>
    `;
  }

  // Inspector when stepping a trace — shows the current step's I/O instead of edit form.
  private _renderSimInspector(step: TraceStep) {
    return html`
      <div class="inspector-body">
        <div class="node-card" style="margin-bottom:12px">
          <div class="node-card-head">
            <span class="icon-tile icon-tile--${this._toneFor(step.node_kind)}">
              <uk-icon icon=${this._iconFor(step.node_kind)} height="14" width="14"></uk-icon>
            </span>
            <span class="node-card-kind">${step.node_kind.replace('_', ' ')}</span>
          </div>
          <div class="node-card-label">${step.label}</div>
          <div style="font-size:11px;color:var(--muted-foreground);font-family:var(--uk-font-monospace, monospace);margin-top:4px">
            +${step.started_at_ms.toFixed(1)}ms  ·  ${step.duration_ms.toFixed(1)}ms  ·  <span class=${'step-tag step-tag--' + step.status}>${step.status}</span>
          </div>
        </div>

        <div class="form-section">
          <label>
            <uk-icon icon="arrow-down-circle" height="11" width="11"></uk-icon>
            Input  <span style="font-weight:500;color:var(--muted-foreground);letter-spacing:0;text-transform:none">${Object.keys(step.inputs).length} field${Object.keys(step.inputs).length === 1 ? '' : 's'}</span>
          </label>
          <pre class="json-block" .innerHTML=${this._highlightJson(step.inputs)}></pre>
        </div>

        <div class="form-section">
          <label>
            <uk-icon icon="arrow-up-circle" height="11" width="11"></uk-icon>
            Output  <span style="font-weight:500;color:var(--muted-foreground);letter-spacing:0;text-transform:none">${Object.keys(step.outputs).length} field${Object.keys(step.outputs).length === 1 ? '' : 's'}</span>
          </label>
          <pre class="json-block" .innerHTML=${this._highlightJson(step.outputs)}></pre>
        </div>

        ${step.note ? html`
          <div style="font-size:11px;color:var(--muted-foreground);font-style:italic;border-top:1px solid var(--border);padding-top:10px">
            <uk-icon icon="info" height="11" width="11"></uk-icon>
            ${step.note}
          </div>
        ` : nothing}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-flow-builder': OrFlowBuilder;
  }
}
