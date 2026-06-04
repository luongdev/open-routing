// v0.2 Layer 2 — API-backed trace viewer (read-only). Binds to
// GET /v1/orgs/{org_id}/traces/{id}, which is a not-implemented stub until
// Layer 3 (the runtime that records traces). So in Layer 2 it attempts the real
// fetch, reports the stub honestly via a banner, and keeps the rich sample
// trace (MOCK_*) visible as a PREVIEW — deterministic replay over a mini
// flow-graph (step list · graph · step I/O). The Trace->viewer mapping for live
// data lands with the runtime in Layer 3.

import { LitElement, html, css, svg, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';
import type { ApiClient } from '../../api/client.js';
import {
  MOCK_FLOWS,
  MOCK_FLOW_GRAPH,
  MOCK_TRACE_STEPS,
  MOCK_SIM_OUTCOME,
  MOCK_SIM_INPUT,
  type FlowNode,
  type FlowNodeKind,
  type TraceStep,
  type StepStatus,
} from '../shell/playground-mock-data.js';

// FlowNodeKind has grown well past trace-viewer's original 8; declare as
// Partial so any new kind that ever lands in a trace gets a safe fallback
// (`circle` / `neutral`) instead of an empty icon. The accessor helpers
// below apply the defaults.
type Tone = 'neutral' | 'primary' | 'warning' | 'destructive' | 'success';

const KIND_ICON: Partial<Record<FlowNodeKind, string>> = {
  // Original 8
  trigger: 'play', match_skill: 'target', filter: 'filter',
  route_queue: 'list', reservation: 'user-check', effect: 'zap',
  fallback: 'shield-alert', end: 'circle-check',
  // Control flow
  if_else: 'git-fork', switch_case: 'split', loop_for: 'repeat',
  loop_while: 'repeat-2', parallel: 'rows-3', try_catch: 'shield',
  wait: 'clock',
  // User input
  get_dtmf: 'phone-call', prompt_text: 'message-square',
  wait_signal: 'satellite', manual_approval: 'gavel',
  // Data + integration
  set_var: 'pencil-line', compute: 'sigma',
  http_request: 'globe', webhook: 'send',
  // Channel-specific
  tts_speak: 'volume-2', play_prompt: 'megaphone',
  detect_speech: 'mic', transfer_call: 'phone-forwarded', hangup: 'phone-off',
  send_message: 'message-circle', quick_replies: 'mouse-pointer-2',
  typing_indicator: 'more-horizontal', attach_file: 'paperclip',
  bot_handoff: 'bot', send_template: 'mail',
  csat_survey: 'star', nps_survey: 'gauge',
  set_agent_state: 'user-cog', wrapup_timer: 'timer',
  log: 'file-text',
};

const KIND_TONE: Partial<Record<FlowNodeKind, Tone>> = {
  trigger: 'primary', match_skill: 'neutral', filter: 'neutral',
  route_queue: 'success', reservation: 'primary', effect: 'warning',
  fallback: 'destructive', end: 'neutral',
  if_else: 'neutral', switch_case: 'neutral', loop_for: 'neutral',
  loop_while: 'neutral', parallel: 'neutral', try_catch: 'warning',
  wait: 'neutral',
  get_dtmf: 'primary', prompt_text: 'primary',
  wait_signal: 'primary', manual_approval: 'warning',
  set_var: 'primary', compute: 'primary',
  http_request: 'primary', webhook: 'primary',
  tts_speak: 'primary', play_prompt: 'primary', detect_speech: 'primary',
  transfer_call: 'primary', hangup: 'destructive',
  send_message: 'primary', quick_replies: 'primary', typing_indicator: 'neutral',
  attach_file: 'neutral', bot_handoff: 'warning', send_template: 'primary',
  csat_survey: 'warning', nps_survey: 'warning',
  set_agent_state: 'neutral', wrapup_timer: 'neutral',
  log: 'neutral',
};

const iconFor = (k: FlowNodeKind): string => KIND_ICON[k] ?? 'circle';
const toneFor = (k: FlowNodeKind): Tone => KIND_TONE[k] ?? 'neutral';

@customElement('or-trace-viewer')
export class OrTraceViewer extends LitElement {
  static override styles = css`
    :host {
      display: flex;
      flex-direction: column;
      min-height: 680px;
      height: calc(100vh - 160px);
      background: var(--background);
    }

    .trace-banner {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 20px;
      font-size: 12px;
      font-weight: 500;
      color: var(--warning);
      background: color-mix(in oklch, var(--warning) 12%, transparent);
      border-bottom: 1px solid color-mix(in oklch, var(--warning) 30%, transparent);
    }

    .trace-header {
      display: flex; align-items: center; gap: 12px;
      padding: 14px 20px;
      background: var(--card);
      border-bottom: 1px solid var(--border);
    }
    .trace-id {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 12px;
      padding: 3px 8px;
      border-radius: 6px;
      background: var(--muted);
      color: var(--muted-foreground);
    }
    .trace-header-title strong {
      font-size: 16px; font-weight: 700; color: var(--foreground);
      display: block;
    }
    .trace-header-title .meta {
      font-size: 12px; color: var(--muted-foreground);
    }
    .header-spacer { flex: 1; }

    .summary-pills { display: flex; gap: 6px; }
    .summary-pill {
      display: inline-flex; align-items: center; gap: 4px;
      padding: 4px 10px;
      border-radius: 9999px;
      background: var(--card);
      border: 1px solid var(--border);
      font-size: 11px;
      color: var(--foreground);
    }
    .summary-pill uk-icon { color: var(--muted-foreground); }

    .body {
      display: grid;
      grid-template-columns: 280px 1fr 340px;
      flex: 1;
      min-height: 0;
    }

    .pane {
      display: flex;
      flex-direction: column;
      min-height: 0;
      overflow-y: auto;
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

    /* ----- Step list (left) ----- */
    .step-list {
      border-right: 1px solid var(--border);
    }
    .step-row {
      display: grid;
      grid-template-columns: 22px 1fr 18px;
      align-items: center;
      gap: 10px;
      padding: 10px 14px;
      cursor: pointer;
      border-bottom: 1px solid var(--border);
      transition: background .12s;
      position: relative;
    }
    .step-row:hover { background: var(--muted); }
    .step-row--active {
      background: color-mix(in oklch, var(--primary) 8%, transparent);
    }
    .step-row--active::before {
      content: '';
      position: absolute;
      left: 0; top: 0; bottom: 0;
      width: 3px;
      background: var(--primary);
    }

    .icon-tile {
      width: 22px; height: 22px; border-radius: 6px;
      display: inline-flex; align-items: center; justify-content: center;
    }
    .icon-tile--neutral     { background: var(--muted); color: var(--muted-foreground); }
    .icon-tile--primary     { background: color-mix(in oklch, var(--primary) 18%, transparent); color: var(--primary); }
    .icon-tile--success     { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .icon-tile--warning     { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }
    .icon-tile--destructive { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }

    .step-row-text {
      min-width: 0;
    }
    .step-row-name {
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      display: block;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .step-row-meta {
      font-size: 11px;
      color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
    }

    .step-row-status {
      width: 18px; height: 18px;
      border-radius: 50%;
      display: inline-flex; align-items: center; justify-content: center;
    }
    .step-row-status--ok      { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }
    .step-row-status--skipped { background: var(--muted); color: var(--muted-foreground); }
    .step-row-caught {
      margin-left: 6px;
      padding: 0 5px;
      border-radius: 8px;
      font-size: 9px;
      font-weight: 700;
      vertical-align: middle;
      background: color-mix(in oklch, var(--warning) 18%, transparent);
      color: color-mix(in oklch, var(--warning) 85%, var(--foreground));
    }
    .step-row-status--fail    { background: color-mix(in oklch, var(--destructive) 14%, transparent); color: var(--destructive); }
    .step-row-status--timeout { background: color-mix(in oklch, var(--warning) 22%, transparent); color: var(--warning); }

    /* ----- Graph (center) ----- */
    .graph-wrap {
      position: relative;
      overflow: auto;
      /* See flow-builder.ts: prevents SVG's min-width from inflating the grid track. */
      min-width: 0;
      background:
        radial-gradient(circle, color-mix(in oklch, var(--muted-foreground) 18%, transparent) 1px, transparent 1px);
      background-size: 24px 24px;
      background-color: var(--background);
    }
    .graph-svg {
      display: block;
      width: 100%;
      min-width: 1260px;
      height: 100%;
      min-height: 520px;
    }
    .edge { fill: none; stroke: var(--border); stroke-width: 1.5; }
    .edge--success { stroke: color-mix(in oklch, var(--success) 80%, transparent); }
    .edge--timeout { stroke: color-mix(in oklch, var(--destructive) 70%, transparent); stroke-dasharray: 6 4; }
    .edge--hit { stroke: var(--primary); stroke-width: 2.5; }
    .edge-label {
      font-size: 11px; font-weight: 600;
      fill: var(--muted-foreground);
      paint-order: stroke;
      stroke: var(--background); stroke-width: 4; stroke-linejoin: round;
    }
    .edge-label--success { fill: var(--success); }
    .edge-label--timeout { fill: var(--destructive); }

    .node-card {
      width: 152px;
      padding: 10px 12px;
      border-radius: 10px;
      background: var(--card);
      border: 1.5px solid var(--border);
      box-shadow: var(--shadow-sm);
      transition: border-color .12s, box-shadow .12s;
      box-sizing: border-box;
      cursor: pointer;
      opacity: 0.4;
    }
    .node-card--hit { opacity: 1; border-color: color-mix(in oklch, var(--primary) 70%, var(--border)); }
    .node-card--active {
      opacity: 1;
      border-color: var(--primary);
      box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 30%, transparent), var(--shadow-md);
    }
    .node-card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
    .node-card-kind {
      font-size: 10px; font-weight: 700; letter-spacing: 0.05em;
      text-transform: uppercase; color: var(--muted-foreground);
    }
    .node-card-label {
      font-size: 13px; font-weight: 600; color: var(--foreground);
    }
    .node-card-timing {
      font-size: 10px; color: var(--muted-foreground);
      font-family: var(--uk-font-monospace, monospace);
      margin-top: 2px;
    }

    /* ----- I/O (right) ----- */
    .io {
      border-left: 1px solid var(--border);
    }
    .io-body { padding: 16px; }
    .io-section { margin-bottom: 16px; }
    .io-section-header {
      display: flex; align-items: center; gap: 6px;
      margin-bottom: 8px;
    }
    .io-section-header .badge {
      font-size: 10px; font-weight: 700;
      letter-spacing: 0.05em; text-transform: uppercase;
      padding: 2px 7px; border-radius: 4px;
      color: var(--muted-foreground);
      background: var(--muted);
    }
    .io-section-header .badge--in  { background: color-mix(in oklch, var(--primary) 14%, transparent); color: var(--primary); }
    .io-section-header .badge--out { background: color-mix(in oklch, var(--success) 18%, transparent); color: var(--success); }

    .json-block {
      font-family: var(--uk-font-monospace, monospace);
      font-size: 11px;
      line-height: 1.55;
      background: var(--muted);
      color: var(--foreground);
      border: 1px solid var(--border);
      padding: 10px 12px;
      border-radius: 8px;
      white-space: pre;
      overflow-x: auto;
    }

    .json-key      { color: var(--primary); font-weight: 500; }
    .json-string   { color: var(--success); }
    .json-number   { color: var(--warning); }
    .json-bool     { color: var(--destructive); }
    .json-null     { color: var(--muted-foreground); font-style: italic; }
  `;

  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: String, attribute: 'trace-id' }) accessor traceId = MOCK_SIM_OUTCOME.trace_id;
  @property({ type: Object }) accessor client!: ApiClient;

  @state() private accessor _selectedStepIndex = 4;

  // Probe the real endpoint so the contract wiring is exercised, returning the
  // live Trace or null. The Layer-1 stub answers 500 { reason: 'not_implemented' }
  // (or a future 501), so the task value is null and the render falls back to the
  // sample preview + banner. `client` is in the deps so a late-arriving client
  // (set in a separate update than orgId) reruns the probe. Live Trace->viewer
  // mapping lands in Layer 3.
  private _loadTask = new Task(this, {
    task: async ([client, orgId, traceId], { signal }) => {
      if (!client || !traceId) return null;
      const res = (await (client as ApiClient).GET('/v1/orgs/{org_id}/traces/{id}' as never, {
        params: { path: { org_id: orgId as string, id: traceId as string } },
        signal,
      } as never)) as { data?: unknown };
      return res.data ?? null;
    },
    args: () => [this.client, this.orgId, this.traceId] as const,
  });

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  private _statusIcon(s: StepStatus): string {
    return s === 'ok' ? 'check' : s === 'fail' ? 'x' : s === 'timeout' ? 'clock' : 'minus';
  }

  private _highlightJson(value: unknown): string {
    // HTML-escape FIRST so any user-supplied string in the JSON (v0.2 when real
    // traces are wired) can't break out of the highlight spans. Today this
    // viewer only consumes static MOCK_TRACE_STEPS, but the moment a live
    // trace flows through, un-escaped <, >, & would render as raw HTML.
    const raw = JSON.stringify(value, null, 2);
    const json = raw.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return json
      .replace(/"([^"]+)"(\s*:)/g, '<span class="json-key">"$1"</span>$2')
      .replace(/: "([^"]*)"/g, ': <span class="json-string">"$1"</span>')
      .replace(/: (-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
      .replace(/: (true|false)/g, ': <span class="json-bool">$1</span>')
      .replace(/: null/g, ': <span class="json-null">null</span>');
  }

  private _renderGraph(activeNodeId: string, hitNodeIds: Set<string>) {
    const nodes = MOCK_FLOW_GRAPH.nodes;
    const edges = MOCK_FLOW_GRAPH.edges;
    const NODE_W = 152;
    const NODE_H = 70;
    const byId = new Map(nodes.map(n => [n.id, n]));

    const hitSteps = MOCK_TRACE_STEPS.slice(0, this._selectedStepIndex + 1);
    const hitEdgeSet = new Set<string>();
    for (let i = 0; i < hitSteps.length - 1; i++) {
      const a = hitSteps[i]!.node_id;
      const b = hitSteps[i + 1]!.node_id;
      const e = edges.find(x => x.from === a && x.to === b);
      if (e) hitEdgeSet.add(e.id);
    }

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
        <marker id="arrow-hit" viewBox="0 0 10 10" refX="9" refY="5"
                markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="var(--primary)"/>
        </marker>
      </defs>
      ${edges.map(e => {
        const from = byId.get(e.from); const to = byId.get(e.to);
        if (!from || !to) return null;
        const x1 = from.x + NODE_W, y1 = from.y + NODE_H / 2;
        const x2 = to.x, y2 = to.y + NODE_H / 2;
        const dx = Math.max(40, (x2 - x1) * 0.5);
        const d = `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
        const hit = hitEdgeSet.has(e.id);
        const cls = hit ? 'edge edge--hit'
          : e.branch === 'success' ? 'edge edge--success'
          : e.branch === 'timeout' ? 'edge edge--timeout' : 'edge';
        const marker = hit ? 'url(#arrow-hit)'
          : e.branch === 'success' ? 'url(#arrow-success)' : 'url(#arrow)';
        return svg`<path class=${cls} d=${d} marker-end=${marker}></path>`;
      })}
      ${nodes.map((node: FlowNode) => {
        const tone = toneFor(node.kind);
        const icon = iconFor(node.kind);
        const isActive = node.id === activeNodeId;
        const isHit = hitNodeIds.has(node.id);
        const cls = isActive ? 'node-card node-card--active'
                  : isHit ? 'node-card node-card--hit'
                  : 'node-card';
        const step = MOCK_TRACE_STEPS.find(s => s.node_id === node.id);
        return svg`
          <foreignObject x=${node.x} y=${node.y} width=${NODE_W} height=${NODE_H}>
            <div xmlns="http://www.w3.org/1999/xhtml" class=${cls}
                 @click=${() => {
                   const idx = MOCK_TRACE_STEPS.findIndex(s => s.node_id === node.id);
                   if (idx >= 0) this._selectedStepIndex = idx;
                 }}>
              <div class="node-card-head">
                <span class="icon-tile icon-tile--${tone}">
                  <uk-icon icon=${icon} height="12" width="12"></uk-icon>
                </span>
                <span class="node-card-kind">${node.kind.replace('_', ' ')}</span>
              </div>
              <div class="node-card-label">${node.label}</div>
              ${step ? html`<div class="node-card-timing">+${step.started_at_ms.toFixed(1)}ms · ${step.duration_ms.toFixed(1)}ms</div>` : ''}
            </div>
          </foreignObject>
        `;
      })}
    `;
  }

  override render() {
    const flow = MOCK_FLOWS[0]!;
    const steps = MOCK_TRACE_STEPS;
    const selectedStep = steps[this._selectedStepIndex] ?? steps[0]!;
    const hitNodeIds = new Set(steps.slice(0, this._selectedStepIndex + 1).map(s => s.node_id));
    // Until the runtime records traces (Layer 3) the probe yields no live trace,
    // so we render the sample preview and say so. A real Trace hides the banner.
    const live = this._loadTask.value;

    return html`
      ${live ? nothing : html`
        <div class="trace-banner" role="status">
          <uk-icon icon="info" height="13" width="13"></uk-icon>
          Sample trace — live trace data (GET /traces/{id}) lands in Layer 3.
        </div>
      `}
      <div class="trace-header">
        <uk-icon icon="git-branch" height="22" width="22" style="color:var(--primary)"></uk-icon>
        <div class="trace-header-title">
          <strong>Trace viewer</strong>
          <span class="meta">${flow.name} · v${flow.version}</span>
        </div>

        <div class="header-spacer"></div>

        <span class="trace-id">${this.traceId}</span>
        <div class="summary-pills">
          <span class="summary-pill">
            <uk-icon icon="phone" height="11" width="11"></uk-icon>
            ${MOCK_SIM_INPUT.channel}
          </span>
          <span class="summary-pill">
            <uk-icon icon="award" height="11" width="11"></uk-icon>
            ${MOCK_SIM_INPUT.customer_tier}
          </span>
          <span class="summary-pill">
            <uk-icon icon="clock" height="11" width="11"></uk-icon>
            ${MOCK_SIM_OUTCOME.total_latency_ms.toFixed(1)}ms
          </span>
          <span class="summary-pill" style="background:color-mix(in oklch, var(--success) 18%, transparent);color:var(--success);border-color:transparent">
            <uk-icon icon="check" height="11" width="11"></uk-icon>
            ${MOCK_SIM_OUTCOME.outcome}
          </span>
        </div>
      </div>

      <div class="body">
        <aside class="pane step-list">
          <div class="pane-header">Steps  ·  ${steps.length}</div>
          ${steps.map((s: TraceStep, i: number) => {
            const isActive = i === this._selectedStepIndex;
            return html`
              <div
                class=${isActive ? 'step-row step-row--active' : 'step-row'}
                @click=${() => { this._selectedStepIndex = i; }}
              >
                <span class="icon-tile icon-tile--${toneFor(s.node_kind)}">
                  <uk-icon icon=${iconFor(s.node_kind)} height="11" width="11"></uk-icon>
                </span>
                <div class="step-row-text">
                  <span class="step-row-name">${s.label}${s.caught ? html`<span class="step-row-caught" title="Caught by try/catch">caught</span>` : ''}</span>
                  <span class="step-row-meta">+${s.started_at_ms.toFixed(1)}ms · ${s.duration_ms.toFixed(1)}ms${
                    s.iteration != null ? ` · iter ${s.iteration}` : ''}${s.branch != null ? ` · branch ${s.branch}` : ''}</span>
                </div>
                <span class="step-row-status step-row-status--${s.status}">
                  <uk-icon icon=${this._statusIcon(s.status)} height="10" width="10"></uk-icon>
                </span>
              </div>
            `;
          })}
        </aside>

        <main class="pane graph-wrap">
          <svg
            class="graph-svg"
            viewBox="0 0 1260 520"
            xmlns="http://www.w3.org/2000/svg"
          >
            ${this._renderGraph(selectedStep.node_id, hitNodeIds)}
          </svg>
        </main>

        <aside class="pane io">
          <div class="pane-header">Step I/O</div>
          <div class="io-body">
            <div style="margin-bottom:14px">
              <div style="font-size:13px;font-weight:600;color:var(--foreground);margin-bottom:2px">${selectedStep.label}</div>
              <div style="font-size:11px;color:var(--muted-foreground);font-family:var(--uk-font-monospace, monospace)">${selectedStep.node_kind.replace('_', ' ')} · ${selectedStep.duration_ms.toFixed(1)}ms · ${selectedStep.status}</div>
              ${selectedStep.note ? html`<div style="font-size:12px;color:var(--muted-foreground);font-style:italic;margin-top:6px">${selectedStep.note}</div>` : nothing}
            </div>

            <div class="io-section">
              <div class="io-section-header">
                <span class="badge badge--in">Input</span>
                <span style="font-size:11px;color:var(--muted-foreground)">${Object.keys(selectedStep.inputs).length} field${Object.keys(selectedStep.inputs).length === 1 ? '' : 's'}</span>
              </div>
              <pre class="json-block" .innerHTML=${this._highlightJson(selectedStep.inputs)}></pre>
            </div>

            <div class="io-section">
              <div class="io-section-header">
                <span class="badge badge--out">Output</span>
                <span style="font-size:11px;color:var(--muted-foreground)">${Object.keys(selectedStep.outputs).length} field${Object.keys(selectedStep.outputs).length === 1 ? '' : 's'}</span>
              </div>
              <pre class="json-block" .innerHTML=${this._highlightJson(selectedStep.outputs)}></pre>
            </div>
          </div>
        </aside>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-trace-viewer': OrTraceViewer;
  }
}
