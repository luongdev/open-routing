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
import type { components } from '../../api/generated.js';
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
  type TraceStep,
  type StepStatus,
  type InitVar,
  type TestCase,
} from '../shell/playground-mock-data.js';

type Flow = components['schemas']['Flow'];

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

// Fraction (0..1) of card WIDTH where output port `idx` sits along the
// bottom edge. Matches CSS `justify-content: space-around` on
// .node-card-ports — equal half-gaps at left/right, even spacing between.
// Consumed by portX() (edge anchoring) — single source of truth.
const NODE_W_PX = 168;
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
      grid-template-columns: 220px 1fr 320px;
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
      padding: 14px 16px 8px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      border-bottom: 1px solid var(--border);
    }

    /* ----- Palette ----- */
    .palette {
      border-right: 1px solid var(--border);
      overflow-y: auto;
    }
    .palette-list {
      padding: 8px;
      display: flex;
      flex-direction: column;
      gap: 2px;
    }
    .palette-group-label {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      color: var(--muted-foreground);
      padding: 10px 10px 4px;
      margin-top: 4px;
    }
    .palette-group-label:first-child { margin-top: 0; padding-top: 4px; }
    .palette-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 9px 10px;
      border-radius: 8px;
      border: 1px solid transparent;
      cursor: pointer;
      transition: background .12s, border-color .12s;
      background: transparent;
      text-align: left;
    }
    .palette-item:hover {
      background: var(--muted);
      border-color: color-mix(in oklch, var(--primary) 25%, var(--border));
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
    .canvas-wrap {
      position: relative;
      overflow: auto;
      /* Prevent the SVG's min-width from inflating this grid track and
         pushing the inspector off-screen. The SVG scrolls inside this pane
         instead. */
      min-width: 0;
      background:
        radial-gradient(circle, color-mix(in oklch, var(--muted-foreground) 18%, transparent) 1px, transparent 1px);
      background-size: 24px 24px;
      background-color: var(--background);
    }
    .canvas-svg {
      display: block;
      width: 100%;
      min-width: 560px;
      height: auto;
      min-height: 2720px;
    }

    .edge {
      fill: none;
      stroke: var(--border);
      stroke-width: 1.5;
    }
    .edge--success { stroke: color-mix(in oklch, var(--success) 80%, transparent); }
    .edge--timeout { stroke: color-mix(in oklch, var(--destructive) 70%, transparent); stroke-dasharray: 6 4; }
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

    .node-card {
      display: flex;
      flex-direction: column;
      gap: 4px;
      width: 168px;
      height: 116px;
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
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
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
    .sim-pb-btn--primary {
      background: var(--primary);
      color: var(--primary-foreground);
    }
    .sim-pb-btn--primary:hover:not(:disabled) {
      background: var(--primary);
      color: var(--primary-foreground);
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
      gap: 2px;             /* tightened from 3px so SWITCH/CASE 5 chips fit at 168px card width */
      flex-wrap: nowrap;
      overflow: hidden;
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
      flex-shrink: 0;
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

    /* ----- Run panel (bottom) ----- */
    /* Align columns to the body grid (220 / 1fr / 320) so dividers don't jog
       across the seam. Design review caught this. */
    .run-panel {
      display: grid;
      grid-template-columns: 220px 1fr 320px;
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
    }
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

  // Live graph state. Loaded from the flow's opaque `graph` JSONB (GET),
  // mutated by drag, persisted by Save (PATCH). Deep-copied on load so
  // inspector edits never alias the loaded response.
  @state() private accessor _nodes: FlowNode[] = [];
  @state() private accessor _edges: FlowEdge[] = [];
  @state() private accessor _draggedId: string | null = null;
  private _drag: { id: string; startX: number; startY: number; ox: number; oy: number } | null = null;
  private _svgRef: SVGSVGElement | null = null;
  private _pendingClick: { id: string } | null = null;

  // --- Simulator state ---
  @state() private accessor _simMode: 'edit' | 'sim' = 'edit';
  // Index into the active trace. -1 = "ready to run, no step executed yet".
  @state() private accessor _simStep: number = -1;
  @state() private accessor _simScenario: 'success' | 'fail' = 'success';
  @state() private accessor _initVars: InitVar[] = MOCK_INIT_VARS.map(v => ({ ...v }));
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
    task: async ([orgId, flowId], { signal }) => {
      if (!flowId) {
        this._nodes = [];
        this._edges = [];
        this._loaded = null;
        return null;
      }
      const { data, error } = await this.client.GET('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: orgId as string, id: flowId as string } },
        signal,
      } as never);
      if (error) throw error;
      if (signal.aborted) return null;
      const flow = data as Flow;
      const graph = (flow.graph ?? {}) as { nodes?: FlowNode[]; edges?: FlowEdge[] };
      this._nodes = structuredClone(graph.nodes ?? []) as FlowNode[];
      this._edges = structuredClone(graph.edges ?? []) as FlowEdge[];
      this._loaded = flow;
      this._selectedNodeId = this._nodes[0]?.id ?? null;
      return flow;
    },
    args: () => [this.orgId, this.flowId] as const,
  });

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    clearTimeout(this._actionToastTimer);
  }

  private _actionToastTimer?: ReturnType<typeof setTimeout>;
  private _flashAction(message: string, tone: 'ok' | 'warn' | 'error' = 'ok'): void {
    this._actionToast = message;
    this._actionTone = tone;
    clearTimeout(this._actionToastTimer);
    this._actionToastTimer = setTimeout(() => { this._actionToast = null; }, 3200);
  }

  // PATCH the graph (or POST a new draft in create mode). The only write that
  // is real in Layer 2 — version is sent for optimistic concurrency; 409 is
  // surfaced as a conflict rather than silently refreshed.
  private async _saveDraft(): Promise<void> {
    if (this._saving) return;
    this._saving = true;
    try {
      const graph = { nodes: this._nodes, edges: this._edges };
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
      const { data, error } = await this.client.PATCH('/v1/orgs/{org_id}/flows/{id}' as never, {
        params: { path: { org_id: this.orgId, id: current.id } },
        body: { graph, version: current.version },
      } as never);
      if (error) {
        const status = (error as { status?: number }).status;
        this._flashAction(
          status === 409
            ? 'Version conflict — this draft changed elsewhere. Reload before saving.'
            : this._errText(error, 'Save failed.'),
          'error',
        );
        return;
      }
      this._loaded = data as Flow;
      this._flashAction(`Saved draft · v${(data as Flow).version}.`, 'ok');
    } finally {
      this._saving = false;
    }
  }

  private _errText(error: unknown, fallback: string): string {
    const r = (error as { reason?: string; message?: string }) ?? {};
    return r.reason ?? r.message ?? fallback;
  }

  // Validate / Publish / Rollback are 501 stubs until Layer 3. Call the real
  // endpoint anyway so the contract wiring is exercised, and report the 501
  // honestly instead of faking a result.
  private async _runStubAction(
    op: 'validate' | 'publish' | 'rollback',
    label: string,
  ): Promise<void> {
    if (this._isCreate || !this._loaded) {
      this._flashAction('Save the draft first.', 'warn');
      return;
    }
    const id = this._loaded.id;
    const path =
      op === 'validate'
        ? '/v1/orgs/{org_id}/flows/{id}/validate'
        : op === 'publish'
        ? '/v1/orgs/{org_id}/flows/{id}/publish'
        : '/v1/orgs/{org_id}/flows/{id}/rollback';
    const body =
      op === 'validate'
        ? undefined
        : op === 'publish'
        ? { channel: 'voice', entry_code: 'main', version: this._loaded.version }
        : { channel: 'voice', entry_code: 'main', to_version_number: 1 };
    const { error, response } = await this.client.POST(path as never, {
      params: { path: { org_id: this.orgId, id } },
      ...(body ? { body } : {}),
    } as never);
    if (response?.status === 501) {
      this._flashAction(`${label} lands in Layer 3 (runtime not wired yet).`, 'warn');
      return;
    }
    if (error) {
      this._flashAction(this._errText(error, `${label} failed.`), 'error');
      return;
    }
    this._flashAction(`${label} OK.`, 'ok');
  }

  private get _activeTrace(): TraceStep[] {
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
  private get _pausedForInput(): boolean {
    const step = this._currentStep;
    return !!step && WAIT_INPUT_KINDS.has(step.node_kind);
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
    // Try common output keys for captured value. Falls back to empty.
    // String() also coerces numeric DTMF values cleanly.
    const out = step.outputs;
    const v = out['menu_choice'] ?? out['text'] ?? out['signal_payload'] ??
      out['supervisor_choice'] ?? out['captured_value'] ?? '';
    this._inputDraft = String(v);
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
    this._saveToast = `Saved "${name}"`;
    setTimeout(() => { if (this._saveToast === `Saved "${name}"`) this._saveToast = null; }, 2400);
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
    this._saveToast = `Replayed "${tc.name}"`;
    setTimeout(() => { if (this._saveToast === `Replayed "${tc.name}"`) this._saveToast = null; }, 2400);
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
      this._restartSim();
    } else {
      this._simMode = 'sim';
      this._flashAction('Sample trace preview — live simulation (POST /simulate) lands in Layer 3.', 'warn');
    }
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
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    this._drag = { id: node.id, startX: e.clientX, startY: e.clientY, ox: node.x, oy: node.y };
    this._pendingClick = { id: node.id };
  }

  private _onNodePointerMove(e: PointerEvent): void {
    if (!this._drag) return;
    const dxPx = e.clientX - this._drag.startX;
    const dyPx = e.clientY - this._drag.startY;
    // 4px threshold before promoting to drag (vs click)
    if (this._draggedId == null && Math.hypot(dxPx, dyPx) < 4) return;
    const ratio = this._svgPerCssPx();
    const dx = dxPx * ratio;
    const dy = dyPx * ratio;
    this._draggedId = this._drag.id;
    this._pendingClick = null;  // movement promoted past click threshold
    const newX = Math.max(0, Math.round((this._drag.ox + dx) / 10) * 10);
    const newY = Math.max(0, Math.round((this._drag.oy + dy) / 10) * 10);
    this._nodes = this._nodes.map(n =>
      n.id === this._drag!.id ? { ...n, x: newX, y: newY } : n
    );
  }

  private _onNodePointerUp(e: PointerEvent): void {
    if (this._drag && (e.currentTarget as HTMLElement).hasPointerCapture(e.pointerId)) {
      (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
    }
    // If we never moved past threshold, treat as a click → select node
    if (this._pendingClick && this._pendingClick.id === this._drag?.id) {
      this._selectedNodeId = this._pendingClick.id;
    }
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

  private _renderEdges(nodes: FlowNode[], edges: FlowEdge[]) {
    const byId = new Map(nodes.map(n => [n.id, n]));
    const NODE_W = 168;
    const NODE_H = 116;

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
        const rawLabel = e.label ?? '';
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
        return svg`
          <path class=${cls} d=${d} marker-end=${marker}></path>
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
    const NODE_W = 168;
    const NODE_H = 116;
    const isSim = this._simMode === 'sim';
    const hitIds = isSim ? this._hitNodeIds : new Set<string>();
    const currentNodeId = this._currentStep?.node_id ?? null;

    const stepByNode = new Map<string, TraceStep>();
    if (isSim) {
      const trace = this._activeTrace;
      for (let i = 0; i <= this._simStep && i < trace.length; i++) {
        stepByNode.set(trace[i]!.node_id, trace[i]!);
      }
    }

    return nodes.map(node => {
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
        isSelected && !isSim ? 'node-card--selected' : '',
        isDragging ? 'is-dragging' : '',
      ].filter(Boolean).join(' ');

      const outs = (node.outputs && node.outputs.length > 0)
        ? node.outputs
        : [{ id: 'done', label: 'done', kind: 'success' as const }];

      return svg`
        <foreignObject x=${node.x} y=${node.y} width=${NODE_W} height=${NODE_H} overflow="visible">
          <div
            xmlns="http://www.w3.org/1999/xhtml"
            class=${cls}
            title=${node.label}
            role="button"
            aria-label="Node ${node.label}. Drag to reposition."
            aria-pressed=${isSelected ? 'true' : 'false'}
            @pointerdown=${(e: PointerEvent) => this._onNodePointerDown(e, node)}
            @pointermove=${(e: PointerEvent) => this._onNodePointerMove(e)}
            @pointerup=${(e: PointerEvent) => this._onNodePointerUp(e)}
            @pointercancel=${(e: PointerEvent) => this._onNodePointerUp(e)}
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
            <div class="node-card-ports" title="Output cases this node can produce">
              ${outs.map(o => html`
                <span class=${'node-card-port node-card-port--' + o.kind}>${o.label}</span>
              `)}
            </div>
          </div>
        </foreignObject>
      `;
    });
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

        ${paramRows.length === 0
          ? html`<div style="font-size:12px;color:var(--muted-foreground);font-style:italic;margin-bottom:14px">No parameters.</div>`
          : paramRows}

        ${outputsSection}
      </div>
    `;
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
              <button class="toolbar-btn" @click=${() => this._runStubAction('validate', 'Validate')}>
                <uk-icon icon="check-circle" height="14" width="14"></uk-icon>
                Validate
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
          <button class="toolbar-btn toolbar-btn--primary" @click=${() => this._runStubAction('publish', 'Publish')}>
            <uk-icon icon="rocket" height="14" width="14"></uk-icon>
            Publish
          </button>
        `}
      </div>

      <div class="body">
        <aside class="pane palette">
          <div class="pane-header">${isSim ? 'Nodes (read-only)' : 'Nodes'}</div>
          <div class="palette-list">
            ${PALETTE.map(group => html`
              <div class="palette-group-label">${group.label}</div>
              ${group.items.map(p => html`
                <button class="palette-item" title=${p.desc} ?disabled=${isSim}
                  @click=${() => this._flashAction(`Adding "${p.label}" nodes lands in Layer 3.`, 'warn')}>
                  <span class="icon-tile icon-tile--${p.tone}">
                    <uk-icon icon=${p.icon} height="14" width="14"></uk-icon>
                  </span>
                  <span class="palette-item-text">
                    <span class="palette-item-label">${p.label}</span>
                    <span class="palette-item-desc">${p.desc}</span>
                  </span>
                </button>
              `)}
            `)}
          </div>
        </aside>

        <main class=${isSim ? 'pane canvas-wrap canvas-wrap--sim' : 'pane canvas-wrap'}>
          <svg
            class="canvas-svg"
            viewBox="0 0 560 2720"
            xmlns="http://www.w3.org/2000/svg"
          >
            ${this._renderEdges(nodes, edges)}
            ${this._renderNodes(nodes)}
          </svg>
          <div class="canvas-strip">
            ${isSim ? html`
              <span><span class="dot-ok">●</span> Step <strong>${this._simStep < 0 ? 'ready' : (this._simStep + 1) + ' / ' + this._activeTrace.length}</strong></span>
              <span>Trace <strong>${this._currentStep?.id ?? '—'}</strong></span>
              <span>Scenario <strong>${this._simScenario === 'fail' ? 'failure path' : 'success path'}</strong></span>
            ` : html`
              <span><span class="dot-ok">●</span> <strong>0</strong> errors</span>
              <span><span class="dot-warn">●</span> <strong>1</strong> warning <em style="color:var(--muted-foreground)">— effect 'Notify CRM' has no retry policy</em></span>
            `}
            <div style="flex:1"></div>
            <span>Zoom: <strong>100%</strong></span>
            ${isSim ? nothing : html`<span>Last saved <strong>2m ago</strong></span>`}
          </div>
        </main>

        <aside class="pane inspector">
          <div class="pane-header">${isSim && this._currentStep ? 'Step I/O' : 'Inspector'}</div>
          ${isSim && this._currentStep
            ? this._renderSimInspector(this._currentStep)
            : this._renderInspector(selected)}
        </aside>
      </div>

      ${isSim ? this._renderSimRunPanel() : nothing}
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
        <button class="sim-pb-btn" title="Restart" @click=${this._restartSim}>
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
          <uk-icon icon=${atEnd ? 'rotate-ccw' : 'chevron-right'} height="14" width="14"></uk-icon>
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
    const bag = computeVarBag(this._simStep, this._activeTrace);
    const currentStepId = this._currentStep?.id ?? 'init';
    return html`
      <section class="run-panel" aria-label="Simulator run panel">
        <div class="run-panel-col">
          <div class="run-panel-header">
            <uk-icon icon="zap" height="13" width="13"></uk-icon>
            Init vars
            <span class="run-panel-count">${this._initVars.length}</span>
          </div>
          <div class="initvar-list">
            ${this._initVars.map(v => html`
              <div class="initvar-row">
                <div class="initvar-row-head">
                  <span class="initvar-key">${v.key}</span>
                  <span class=${v.source === 'trigger' ? 'initvar-tag initvar-tag--trigger' : 'initvar-tag'}>${v.source}</span>
                </div>
                <input
                  class="initvar-input"
                  .value=${v.value}
                  readonly
                  title=${v.source === 'trigger' ? 'Derived from trigger node (read-only in v0.2 contract)' : 'User-set — editable in the v0.2 wire-up'}
                />
              </div>
            `)}
            <button class="initvar-add" title="Add a custom variable">
              <uk-icon icon="plus" height="12" width="12"></uk-icon>
              Add variable
            </button>
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
    const isWaitInput = WAIT_INPUT_KINDS.has(step.node_kind);
    return html`
      <div class="run-panel-header">
        <uk-icon icon=${isWaitInput ? 'pause-circle' : 'activity'} height="13" width="13"></uk-icon>
        ${isWaitInput ? 'Paused — input required' : 'Now executing'}
      </div>
      <div class=${step.status === 'fail' ? 'sim-runcard sim-runcard--fail' : isWaitInput ? 'sim-runcard sim-runcard--paused' : 'sim-runcard'}>
        <div class="sim-runcard-head">
          <span class="icon-tile icon-tile--${tone}">
            <uk-icon icon=${icon} height="12" width="12"></uk-icon>
          </span>
          <div>
            <div style="font-size:13px;font-weight:600;color:var(--foreground)">${step.label}</div>
            <div style="font-size:11px;color:var(--muted-foreground);font-family:var(--uk-font-monospace, monospace)">
              ${step.node_kind.replace('_', ' ')} · +${step.started_at_ms.toFixed(1)}ms · ${step.duration_ms.toFixed(1)}ms
            </div>
          </div>
          <span class=${'sim-runcard-status sim-runcard-status--' + step.status}>
            <uk-icon icon=${this._statusIcon(step.status)} height="11" width="11"></uk-icon>
            ${step.status}
          </span>
        </div>
        ${isWaitInput ? this._renderInputForm(step) : nothing}
        ${step.status === 'fail' ? html`
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
