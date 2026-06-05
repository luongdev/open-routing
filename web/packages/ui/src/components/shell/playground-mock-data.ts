import type { components } from '../../api/generated.js';

export const MOCK_ORG_ID = '01919e5c-1234-7890-abcd-1234567890ab';

type Agent       = components['schemas']['Agent'];
type Skill       = components['schemas']['Skill'];
type Queue       = components['schemas']['Queue'];
type Channel     = components['schemas']['Channel'];
type Adapter     = components['schemas']['Adapter'];
type BreakReason = components['schemas']['BreakReason'];
type AgentState  = components['schemas']['AgentState'];

export const MOCK_SKILLS: Skill[] = [
  { id: '01919f00-0001-7000-8000-100000000001', org_id: MOCK_ORG_ID, code: 'skill_voice_tier1',    name: 'Voice Tier 1',    skill_type: 'voice',    description: 'Handle inbound voice calls',    enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-10T00:00:00Z' },
  { id: '01919f00-0001-7000-8000-100000000002', org_id: MOCK_ORG_ID, code: 'skill_voice_tier2',    name: 'Voice Tier 2',    skill_type: 'voice',    description: 'Handle escalated voice calls', enabled: true,  version: 2, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-12T00:00:00Z' },
  { id: '01919f00-0001-7000-8000-100000000003', org_id: MOCK_ORG_ID, code: 'skill_chat_support',   name: 'Chat Support',    skill_type: 'chat',     description: 'Handle live chat queues',      enabled: true,  version: 1, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-02T00:00:00Z' },
  { id: '01919f00-0001-7000-8000-100000000004', org_id: MOCK_ORG_ID, code: 'skill_billing',        name: 'Billing',         skill_type: 'support',  description: 'Billing disputes and refunds', enabled: true,  version: 1, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-03T00:00:00Z' },
  { id: '01919f00-0001-7000-8000-100000000005', org_id: MOCK_ORG_ID, code: 'skill_tech_support',   name: 'Tech Support',    skill_type: 'technical',description: 'Software troubleshooting',      enabled: true,  version: 1, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
  { id: '01919f00-0001-7000-8000-100000000006', org_id: MOCK_ORG_ID, code: 'skill_lang_vi',        name: 'Vietnamese',      skill_type: 'language', description: 'Vietnamese language support',  enabled: false, version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z' },
];

export const MOCK_QUEUES: Queue[] = [
  { id: '01919f00-0002-7000-8000-200000000001', org_id: MOCK_ORG_ID, code: 'queue_billing',   name: 'Billing',       channel_types: ['voice', 'chat'], priority: 5,  acw_sec: 60,  enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-01T00:00:00Z' },
  { id: '01919f00-0002-7000-8000-200000000002', org_id: MOCK_ORG_ID, code: 'queue_tech',      name: 'Tech Support',  channel_types: ['voice'],         priority: 7,  acw_sec: 120, enabled: true,  version: 1, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-02T00:00:00Z' },
  { id: '01919f00-0002-7000-8000-200000000003', org_id: MOCK_ORG_ID, code: 'queue_chat_gen',  name: 'General Chat',  channel_types: ['chat'],          priority: 3,  acw_sec: 30,  enabled: true,  version: 2, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-15T00:00:00Z' },
  { id: '01919f00-0002-7000-8000-200000000004', org_id: MOCK_ORG_ID, code: 'queue_email',     name: 'Email',         channel_types: ['email'],         priority: 2,  acw_sec: 300, enabled: true,  version: 1, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
  { id: '01919f00-0002-7000-8000-200000000005', org_id: MOCK_ORG_ID, code: 'queue_vip',       name: 'VIP',           channel_types: ['voice', 'chat'], priority: 10, acw_sec: 90,  enabled: true,  version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z' },
  { id: '01919f00-0002-7000-8000-200000000006', org_id: MOCK_ORG_ID, code: 'queue_overflow',  name: 'Overflow',      channel_types: ['voice'],         priority: 1,  acw_sec: 45,  enabled: false, version: 1, created_at: '2026-05-06T00:00:00Z', updated_at: '2026-05-06T00:00:00Z' },
];

export const MOCK_AGENTS: Agent[] = [
  {
    id: '01919f00-0003-7000-8000-300000000001', org_id: MOCK_ORG_ID,
    code: 'emp_0001', external_id: 'HR-EMP-0001', name: 'Alice Nguyen', email: 'alice@example.com',
    enabled: true, version: 3, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-18T00:00:00Z',
    skills: [
      { skill_id: '01919f00-0001-7000-8000-100000000001', name: 'Voice Tier 1', proficiency: 9 },
      { skill_id: '01919f00-0001-7000-8000-100000000004', name: 'Billing',      proficiency: 7 },
    ],
  },
  {
    id: '01919f00-0003-7000-8000-300000000002', org_id: MOCK_ORG_ID,
    code: 'emp_0002', external_id: 'HR-EMP-0002', name: 'Bao Tran', email: 'bao@example.com',
    enabled: true, version: 2, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-16T00:00:00Z',
    skills: [
      { skill_id: '01919f00-0001-7000-8000-100000000002', name: 'Voice Tier 2', proficiency: 8 },
      { skill_id: '01919f00-0001-7000-8000-100000000005', name: 'Tech Support', proficiency: 10 },
    ],
  },
  {
    id: '01919f00-0003-7000-8000-300000000003', org_id: MOCK_ORG_ID,
    code: 'emp_0003', external_id: null, name: 'Camille Durand', email: 'camille@example.com',
    enabled: true, version: 1, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-03T00:00:00Z',
    skills: [
      { skill_id: '01919f00-0001-7000-8000-100000000003', name: 'Chat Support', proficiency: 8 },
    ],
  },
  {
    id: '01919f00-0003-7000-8000-300000000004', org_id: MOCK_ORG_ID,
    code: 'emp_0004', external_id: 'HR-EMP-0004', name: 'David Kim', email: 'david@example.com',
    enabled: true, version: 2, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-14T00:00:00Z',
    skills: [
      { skill_id: '01919f00-0001-7000-8000-100000000001', name: 'Voice Tier 1', proficiency: 6 },
      { skill_id: '01919f00-0001-7000-8000-100000000004', name: 'Billing',      proficiency: 9 },
      { skill_id: '01919f00-0001-7000-8000-100000000003', name: 'Chat Support', proficiency: 7 },
    ],
  },
  {
    id: '01919f00-0003-7000-8000-300000000005', org_id: MOCK_ORG_ID,
    code: 'emp_0005', external_id: 'HR-EMP-0005', name: 'Elena Popova', email: 'elena@example.com',
    enabled: false, version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z',
    skills: [],
  },
  {
    id: '01919f00-0003-7000-8000-300000000006', org_id: MOCK_ORG_ID,
    code: 'emp_0006', external_id: null, name: 'Femi Adeyemi', email: 'femi@example.com',
    enabled: true, version: 1, created_at: '2026-05-06T00:00:00Z', updated_at: '2026-05-06T00:00:00Z',
    skills: [
      { skill_id: '01919f00-0001-7000-8000-100000000005', name: 'Tech Support', proficiency: 8 },
    ],
  },
];

export const MOCK_CHANNELS: Channel[] = [
  { id: '01919f00-0004-7000-8000-400000000001', org_id: MOCK_ORG_ID, code: 'channel_voice_primary', name: 'Voice Primary',  channel_type: 'voice', default_queue_id: '01919f00-0002-7000-8000-200000000001', enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-01T00:00:00Z' },
  { id: '01919f00-0004-7000-8000-400000000002', org_id: MOCK_ORG_ID, code: 'channel_voice_ivr',     name: 'Voice IVR',      channel_type: 'voice', default_queue_id: '01919f00-0002-7000-8000-200000000002', enabled: true,  version: 1, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-02T00:00:00Z' },
  { id: '01919f00-0004-7000-8000-400000000003', org_id: MOCK_ORG_ID, code: 'channel_chat_web',      name: 'Web Chat',       channel_type: 'chat',  default_queue_id: '01919f00-0002-7000-8000-200000000003', enabled: true,  version: 2, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-17T00:00:00Z' },
  { id: '01919f00-0004-7000-8000-400000000004', org_id: MOCK_ORG_ID, code: 'channel_chat_mobile',   name: 'Mobile Chat',    channel_type: 'chat',  default_queue_id: null,                                     enabled: true,  version: 1, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
  { id: '01919f00-0004-7000-8000-400000000005', org_id: MOCK_ORG_ID, code: 'channel_email_support', name: 'Support Email',  channel_type: 'email', default_queue_id: '01919f00-0002-7000-8000-200000000004', enabled: false, version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z' },
];

export const MOCK_ADAPTERS: Adapter[] = [
  { id: '01919f00-0005-7000-8000-500000000001', org_id: MOCK_ORG_ID, code: 'adapter_freeswitch_dc1', name: 'FreeSWITCH DC1',  adapter_type: 'freeswitch', config: { host: 'sip-dc1.example.com', port: 5060 }, enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-01T00:00:00Z' },
  { id: '01919f00-0005-7000-8000-500000000002', org_id: MOCK_ORG_ID, code: 'adapter_livekit_prod',   name: 'LiveKit Prod',    adapter_type: 'livekit',    config: { url: 'wss://lk.example.com', api_key: 'demo' }, enabled: true,  version: 2, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-16T00:00:00Z' },
  { id: '01919f00-0005-7000-8000-500000000003', org_id: MOCK_ORG_ID, code: 'adapter_twilio_main',    name: 'Twilio Main',     adapter_type: 'twilio',     config: { account_sid: 'ACdemo123' }, enabled: true,  version: 1, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-03T00:00:00Z' },
  { id: '01919f00-0005-7000-8000-500000000004', org_id: MOCK_ORG_ID, code: 'adapter_freeswitch_dc2', name: 'FreeSWITCH DC2',  adapter_type: 'freeswitch', config: { host: 'sip-dc2.example.com', port: 5060 }, enabled: false, version: 1, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
  { id: '01919f00-0005-7000-8000-500000000005', org_id: MOCK_ORG_ID, code: 'adapter_genesys_cloud',  name: 'Genesys Cloud',   adapter_type: 'genesys',    config: { region: 'us-east-1' }, enabled: true,  version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z' },
];

export const MOCK_BREAK_REASONS: BreakReason[] = [
  { id: '01919f00-0006-7000-8000-600000000001', org_id: MOCK_ORG_ID, code: 'break_lunch',      name: 'Lunch',          routable: false, display_order: 1, enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-01T00:00:00Z' },
  { id: '01919f00-0006-7000-8000-600000000002', org_id: MOCK_ORG_ID, code: 'break_short',      name: 'Short Break',    routable: true,  display_order: 2, enabled: true,  version: 1, created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-01T00:00:00Z' },
  { id: '01919f00-0006-7000-8000-600000000003', org_id: MOCK_ORG_ID, code: 'break_training',   name: 'Training',       routable: false, display_order: 3, enabled: true,  version: 1, created_at: '2026-05-02T00:00:00Z', updated_at: '2026-05-02T00:00:00Z' },
  { id: '01919f00-0006-7000-8000-600000000004', org_id: MOCK_ORG_ID, code: 'break_personal',   name: 'Personal',       routable: false, display_order: 4, enabled: true,  version: 2, created_at: '2026-05-03T00:00:00Z', updated_at: '2026-05-15T00:00:00Z' },
  { id: '01919f00-0006-7000-8000-600000000005', org_id: MOCK_ORG_ID, code: 'break_wellness',   name: 'Wellness',       routable: false, display_order: 5, enabled: false, version: 1, created_at: '2026-05-04T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
  { id: '01919f00-0006-7000-8000-600000000006', org_id: MOCK_ORG_ID, code: 'break_tech_issue', name: 'Tech Issue',     routable: false, display_order: 6, enabled: true,  version: 1, created_at: '2026-05-05T00:00:00Z', updated_at: '2026-05-05T00:00:00Z' },
];

export const MOCK_AGENT_STATES: AgentState[] = [
  { agent_id: '01919f00-0003-7000-8000-300000000001', org_id: MOCK_ORG_ID, status: 'Ready',    engaged_channel: null, break_reason_id: null, post_interaction_state: null, wrapup_until: null, state_version: 5, updated_at: '2026-05-19T08:00:00Z' },
  { agent_id: '01919f00-0003-7000-8000-300000000002', org_id: MOCK_ORG_ID, status: 'Engaged',  engaged_channel: 'voice', break_reason_id: null, post_interaction_state: 'ready', wrapup_until: null, state_version: 12, updated_at: '2026-05-19T08:32:00Z' },
  { agent_id: '01919f00-0003-7000-8000-300000000003', org_id: MOCK_ORG_ID, status: 'WrapUp',   engaged_channel: null, break_reason_id: null, post_interaction_state: 'not_ready', wrapup_until: '2026-05-19T08:35:00Z', state_version: 8, updated_at: '2026-05-19T08:33:00Z' },
  { agent_id: '01919f00-0003-7000-8000-300000000004', org_id: MOCK_ORG_ID, status: 'Break',    engaged_channel: null, break_reason_id: '01919f00-0006-7000-8000-600000000001', post_interaction_state: null, wrapup_until: null, state_version: 3, updated_at: '2026-05-19T08:10:00Z' },
  { agent_id: '01919f00-0003-7000-8000-300000000005', org_id: MOCK_ORG_ID, status: 'Offline',  engaged_channel: null, break_reason_id: null, post_interaction_state: null, wrapup_until: null, state_version: 1, updated_at: '2026-05-18T17:00:00Z' },
  { agent_id: '01919f00-0003-7000-8000-300000000006', org_id: MOCK_ORG_ID, status: 'NotReady', engaged_channel: null, break_reason_id: null, post_interaction_state: null, wrapup_until: null, state_version: 2, updated_at: '2026-05-19T07:45:00Z' },
];

export const MOCK_IMPORT_JOB = {
  id: '01919f00-0009-7000-8000-900000000001',
  org_id: MOCK_ORG_ID,
  entity_type: 'agents' as const,
  status: 'completed' as const,
  total_rows: 10,
  succeeded_rows: 7,
  failed_rows: 3,
  errors: [
    { row: 2, code: 'emp_9999', reason: 'duplicate_code', message: 'Code emp_9999 already exists' },
    { row: 5, code: 'emp_0003', reason: 'invalid_body',   message: 'Email is required' },
    { row: 9, code: '',         reason: 'invalid_body',   message: 'Code is required' },
  ],
  created_at: '2026-05-19T08:00:00Z',
};

// ============================================================
// vNext (v0.2+) — flows, simulator, trace viewer mock data.
// Not part of v0.1 contract; consumed by playground previews only.
// ============================================================

export type FlowStatus = 'draft' | 'published' | 'archived';

export interface FlowSummary {
  id: string;
  org_id: string;
  code: string;
  name: string;
  description: string;
  status: FlowStatus;
  version: number;
  channel_types: ReadonlyArray<'voice' | 'chat' | 'email'>;
  last_published_at: string | null;
  last_simulated_at: string | null;
  updated_at: string;
}

export const MOCK_FLOWS: FlowSummary[] = [
  { id: '01919f00-0010-7000-8000-A00000000001', org_id: MOCK_ORG_ID, code: 'voice_inbound_default',  name: 'Voice — Inbound Default',  description: 'Skill-match → tiered queue → VIP overflow', status: 'published', version: 7, channel_types: ['voice'],          last_published_at: '2026-05-18T09:14:00Z', last_simulated_at: '2026-05-19T07:32:00Z', updated_at: '2026-05-18T09:14:00Z' },
  { id: '01919f00-0010-7000-8000-A00000000002', org_id: MOCK_ORG_ID, code: 'chat_triage_v2',         name: 'Chat — Triage v2',         description: 'Tier-1 chat with billing/tech fork',         status: 'draft',     version: 3, channel_types: ['chat'],           last_published_at: '2026-05-12T14:02:00Z', last_simulated_at: '2026-05-19T08:08:00Z', updated_at: '2026-05-19T08:08:00Z' },
  { id: '01919f00-0010-7000-8000-A00000000003', org_id: MOCK_ORG_ID, code: 'email_billing',          name: 'Email — Billing',          description: 'Billing-only intake to acw=300 queue',       status: 'published', version: 2, channel_types: ['email'],          last_published_at: '2026-05-10T11:00:00Z', last_simulated_at: null,                   updated_at: '2026-05-10T11:00:00Z' },
  { id: '01919f00-0010-7000-8000-A00000000004', org_id: MOCK_ORG_ID, code: 'vip_voice_gold',         name: 'VIP — Voice (Gold tier)',  description: 'Gold customers fast-tracked to VIP queue',    status: 'published', version: 4, channel_types: ['voice', 'chat'],  last_published_at: '2026-05-15T16:40:00Z', last_simulated_at: '2026-05-18T19:11:00Z', updated_at: '2026-05-15T16:40:00Z' },
  { id: '01919f00-0010-7000-8000-A00000000005', org_id: MOCK_ORG_ID, code: 'overflow_tech',          name: 'Overflow — Tech',          description: 'Spillover when tech queue saturated',         status: 'draft',     version: 1, channel_types: ['voice'],          last_published_at: null,                   last_simulated_at: '2026-05-19T08:50:00Z', updated_at: '2026-05-19T08:50:00Z' },
  { id: '01919f00-0010-7000-8000-A00000000006', org_id: MOCK_ORG_ID, code: 'legacy_chat_v1',         name: 'Legacy — Chat v1',         description: 'Archived: superseded by chat_triage_v2',     status: 'archived',  version: 9, channel_types: ['chat'],           last_published_at: '2026-04-22T10:00:00Z', last_simulated_at: null,                   updated_at: '2026-05-01T00:00:00Z' },
];

// ----- Flow graph (for builder + trace viewer) -----

// Full workflow primitive taxonomy. Grouped by category so the palette can
// render sections (Triggers / Control flow / User input / Data / Integration
// / Routing / Side effects / End). A real engine would also support:
// child-flows, sub-graphs, compensations, signals — out of scope for the
// v0.2 preview.
export type FlowNodeKind =
  // Triggers (one per flow)
  | 'trigger'
  // Control flow
  | 'if_else'
  | 'switch_case'
  | 'loop_for'
  | 'loop_while'
  | 'parallel'
  | 'try_catch'
  | 'wait'
  | 'end'
  // User input (the runtime PAUSES until a value is provided — caller's
  // DTMF digits, an agent's free-text answer, an external signal, a
  // manual approval gate). The simulator surfaces an input form for
  // these in the run panel.
  | 'get_dtmf'
  | 'prompt_text'
  | 'wait_signal'
  | 'manual_approval'
  // Data ops (vars, expressions)
  | 'set_var'
  | 'compute'
  | 'script'      // inline JS / Lua / Python — DSL escape hatch
  // Integration (sync/async outbound)
  | 'http_request'
  | 'webhook'
  // Routing (domain-specific)
  | 'match_skill'
  | 'filter'
  | 'route_queue'
  | 'reservation'
  | 'fallback'
  // Voice channel
  | 'tts_speak'
  | 'play_prompt'
  | 'detect_speech'
  | 'transfer_call'
  | 'hangup'
  // Chat channel
  | 'send_message'
  | 'quick_replies'
  | 'typing_indicator'
  | 'attach_file'
  | 'bot_handoff'
  // Email channel
  | 'send_template'
  // Surveys (post-interaction, channel-agnostic)
  | 'csat_survey'
  | 'nps_survey'
  // Agent state (extends routing — engine controls agent presence)
  | 'set_agent_state'
  | 'wrapup_timer'
  // Side effects
  | 'effect'
  | 'log';

// Convenience set so the simulator can recognise "this step needs user input
// before it can complete". Keep in sync with FlowNodeKind.
export const WAIT_INPUT_KINDS: ReadonlySet<FlowNodeKind> = new Set([
  'get_dtmf',
  'prompt_text',
  'wait_signal',
  'manual_approval',
]);

// Output ports declared on a node. Engine-relevant nodes (http_request,
// if_else, switch_case, try_catch, reservation, loop_for) have multiple
// named outputs; simple nodes (set_var, log, end) have a single implicit
// 'done' output. Drawing these on the canvas card answers "what cases
// does this node produce?" without forcing the user to read every
// outgoing edge.
export type OutputKind = 'success' | 'error' | 'timeout' | 'branch' | 'default';

export interface FlowNodeOutput {
  id: string;          // e.g. 'ok' | 'error' | 'timeout' | 'case_1' | 'yes' | 'no'
  label: string;
  kind: OutputKind;
}

export interface FlowNode {
  id: string;
  kind: FlowNodeKind;
  label: string;
  description: string;
  x: number;
  y: number;
  // Params support primitives + arrays + nested string-keyed maps (e.g.
  // http_request headers, switch_case cases). Deliberately narrow: more
  // exotic shapes (Date, RegExp, function refs) belong in v0.2 contract.
  params?: Record<string, string | number | boolean | string[] | Record<string, string>>;
  // Optional explicit outputs. If omitted, node has a single 'done'
  // success output (canvas omits the chip row to keep simple nodes clean).
  outputs?: FlowNodeOutput[];
  // 3D-2 control flow: the body region this node belongs to. "" / undefined =
  // top level; "<ownerId>" for a loop/try body; "<ownerId>#<i>" for parallel
  // branch i. Mirrors backend GraphNode.region (json "region").
  region?: string;
}

export interface FlowEdge {
  id: string;
  from: string;
  to: string;
  label?: string;
  branch?: 'success' | 'fallback' | 'timeout';
  // Which output port of `from` this edge originates from. Used by the
  // canvas to vertically offset the edge's anchor against the node's
  // right edge so each output has a visually distinct exit.
  from_port?: string;
}

export interface FlowGraph {
  flow_id: string;
  nodes: FlowNode[];
  edges: FlowEdge[];
}

// Demo graph for voice_inbound_default. Deliberately exercises every node
// family so the palette + canvas demo doesn't read as a 7-step state
// machine: trigger → data (set_var) → integration (http_request) →
// control (if_else, switch_case, loop_for, try_catch, wait) → routing
// (match_skill, route_queue, reservation, fallback) → side effects
// (effect, log) → end. The two horizontal rows are the happy path
// (y=80) and the fallback path (y=240).
export const MOCK_FLOW_GRAPH: FlowGraph = {
  flow_id: MOCK_FLOWS[0]!.id,
  nodes: [
    // VERTICAL layout — nodes stack top-to-bottom. Main column at x=80.
    // Fallback branch offsets right at x=320 from n9.timeout. y-stride is
    // 168 (NODE_H 116 + ~52 inter-card gap for the bezier curve breathing).
    { id: 'n1',  kind: 'trigger',         label: 'On Voice Inbound',     description: 'Trigger from voice channel',                          x: 80, y: 40,   params: { channel: 'voice', source: 'pstn' } },
    { id: 'n1a', kind: 'get_dtmf',        label: 'IVR menu',              description: 'Play prompt + capture DTMF digits (sync — sim waits for input)', x: 80, y: 208, params: { prompt: 'Press 1 for billing, 2 for tech, 3 for sales, 4 for ops', expected: '1', timeout_sec: 8, max_digits: 1, var: 'menu_choice' } },
    { id: 'n2',  kind: 'set_var',         label: 'Extract customer_id',  description: 'Parse caller_id → customer_id (sync)',                x: 80, y: 376, params: { name: 'customer_id', value_expr: 'parse_caller(from)' } },
    { id: 'n3',  kind: 'http_request',    label: 'Lookup CRM',           description: 'GET /crm/customers/{customer_id} (async, 2s timeout)', x: 80, y: 544, params: { method: 'GET', url: 'https://crm.example/customers/{customer_id}', mode: 'async', timeout_ms: 2000, retry: 1, on_error: 'continue' },
      outputs: [
        { id: 'ok',      label: 'ok',      kind: 'success' },
        { id: 'error',   label: 'error',   kind: 'error' },
        { id: 'timeout', label: 'timeout', kind: 'timeout' },
      ] },
    { id: 'n4',  kind: 'if_else',         label: 'Gold tier?',           description: 'Branch on customer.tier == "gold"',                    x: 80, y: 712, params: { expr: 'customer.tier == "gold"' },
      outputs: [
        { id: 'yes',   label: 'yes', kind: 'branch' },
        { id: 'no',    label: 'no',  kind: 'branch' },
      ] },
    { id: 'n5',  kind: 'switch_case',     label: 'Switch on menu',       description: 'Branch on the DTMF choice the caller pressed',        x: 80, y: 880, params: { switch_on: 'menu_choice', cases: ['1', '2', '3', '4'], default: 'continue' },
      outputs: [
        { id: 'case_1',  label: '1',       kind: 'branch' },
        { id: 'case_2',  label: '2',       kind: 'branch' },
        { id: 'case_3',  label: '3',       kind: 'branch' },
        { id: 'case_4',  label: '4',       kind: 'branch' },
        { id: 'default', label: 'default', kind: 'default' },
      ] },
    { id: 'n6',  kind: 'match_skill',     label: 'Match skill',          description: 'Filter candidate agents by skills + proficiency',     x: 80, y: 1048, params: { required_skills: ['skill_voice_tier1', 'skill_billing'], min_proficiency: 7 } },
    { id: 'n7',  kind: 'loop_for',        label: 'For each candidate',   description: 'Try each candidate agent until one accepts',          x: 80, y: 1216, params: { iter: 'agent in candidates', max_iter: 3 },
      outputs: [
        { id: 'body', label: 'body', kind: 'branch' },
        { id: 'done', label: 'done', kind: 'success' },
      ] },
    { id: 'n8',  kind: 'route_queue',     label: 'Route → VIP',          description: 'Enqueue with priority 10',                            x: 80, y: 1384, params: { queue_code: 'queue_vip', priority: 10 } },
    { id: 'n9',  kind: 'reservation',     label: 'Reserve agent',        description: 'Offer + wait for accept (timeout drops to fallback)', x: 80, y: 1552, params: { offer_timeout_sec: 12, retry: 2 },
      outputs: [
        { id: 'accepted', label: 'accepted', kind: 'success' },
        { id: 'rejected', label: 'rejected', kind: 'error' },
        { id: 'timeout',  label: 'timeout',  kind: 'timeout' },
      ] },
    { id: 'n10', kind: 'try_catch',       label: 'Try: Notify CRM',      description: 'Wrap CRM effect — fall to log + continue on error',   x: 80, y: 1720, params: { catch_kinds: ['http_error', 'timeout'] },
      outputs: [
        { id: 'ok',    label: 'ok',    kind: 'success' },
        { id: 'catch', label: 'catch', kind: 'error' },
      ] },
    { id: 'n11', kind: 'effect',          label: 'POST /crm/notify',     description: 'Side-effect: notify CRM of routing outcome (sync)',   x: 80, y: 1888, params: { method: 'POST', url: 'https://crm.example/hooks/route', mode: 'sync', timeout_ms: 1500 } },
    { id: 'n12', kind: 'log',             label: 'Audit log',            description: 'Emit structured audit log for compliance',            x: 80, y: 2056, params: { level: 'info', message: 'routing.completed', fields: ['interaction_id', 'agent_id', 'queue_id', 'latency_ms'] } },
    { id: 'n13', kind: 'end',             label: 'Done',                 description: 'Happy path complete',                                  x: 80, y: 2224 },
    // Fallback branch — offset right at x=320, joins from n9.timeout.
    { id: 'n20', kind: 'fallback',        label: 'Fallback (timeout)',   description: 'Engaged when n9 times out — no agent accepted',       x: 320, y: 1720, params: { after_sec: 30, reason: 'no_agent_accepted' } },
    { id: 'n21', kind: 'manual_approval', label: 'Supervisor approval',  description: 'Pause — supervisor decides retry vs drop',            x: 320, y: 1888, params: { prompt: 'Reservation timed out. Retry on overflow queue?', choices: ['retry', 'drop'], var: 'supervisor_choice' } },
    { id: 'n22', kind: 'route_queue',     label: 'Route → Overflow',     description: 'Lower-priority overflow queue',                       x: 320, y: 2056, params: { queue_code: 'queue_overflow', priority: 1 } },
    { id: 'n22b', kind: 'script',         label: 'Build payload',        description: 'Custom JS — shape the analytics payload before the fallback log', x: 320, y: 2224, params: {
      lang: 'js',
      code: `// Build the analytics payload for the fallback path.\n// Vars in scope: vars.interaction_id, vars.queue_id, vars.reason\nreturn {\n  interaction: vars.interaction_id,\n  fallback_queue: vars.queue_id,\n  reason: vars.reason || 'no_agent_accepted',\n  ts: Date.now(),\n};`,
    } },
    { id: 'n23', kind: 'log',             label: 'Audit log (fallback)', description: 'Mark routing as fallback for analytics',              x: 320, y: 2392, params: { level: 'warn', message: 'routing.fallback', fields: ['interaction_id', 'queue_id', 'reason'] } },
    { id: 'n24', kind: 'end',             label: 'Done (fallback)',      description: 'Fallback path complete',                              x: 320, y: 2560 },
  ],
  edges: [
    { id: 'e1',  from: 'n1',  to: 'n1a' },
    { id: 'e1a', from: 'n1a', to: 'n2',  label: 'digit',         branch: 'success' },
    { id: 'e2',  from: 'n2',  to: 'n3' },
    // Happy-path edges
    { id: 'e3',  from: 'n3',  to: 'n4',  label: 'ok',            branch: 'success', from_port: 'ok' },
    { id: 'e4',  from: 'n4',  to: 'n5',  label: 'yes',           branch: 'success', from_port: 'yes' },
    { id: 'e5',  from: 'n5',  to: 'n6',  label: 'case 1',        branch: 'success', from_port: 'case_1' },
    { id: 'e6',  from: 'n6',  to: 'n7' },
    { id: 'e7',  from: 'n7',  to: 'n8',  label: 'body',          branch: 'success', from_port: 'body' },
    { id: 'e8',  from: 'n8',  to: 'n9' },
    { id: 'e9',  from: 'n9',  to: 'n10', label: 'accepted',      branch: 'success', from_port: 'accepted' },
    { id: 'e10', from: 'n10', to: 'n11', label: 'ok',            branch: 'success', from_port: 'ok' },
    { id: 'e11', from: 'n11', to: 'n12' },
    { id: 'e12', from: 'n12', to: 'n13' },
    // Error / branch routes — each declared output port has a successor so
    // the canvas doesn't read as "this case is a dead-end". All converge
    // into the fallback row's terminal so the demo stays compact.
    { id: 'e3a', from: 'n3',  to: 'n20', label: 'error',         branch: 'fallback', from_port: 'error' },
    { id: 'e3b', from: 'n3',  to: 'n20', label: 'timeout',       branch: 'timeout',  from_port: 'timeout' },
    { id: 'e4a', from: 'n4',  to: 'n5',  label: 'no',            branch: 'success',  from_port: 'no' },
    { id: 'e5a', from: 'n5',  to: 'n6',  label: 'case 2',        branch: 'success',  from_port: 'case_2' },
    { id: 'e5b', from: 'n5',  to: 'n6',  label: 'case 3',        branch: 'success',  from_port: 'case_3' },
    { id: 'e5c', from: 'n5',  to: 'n6',  label: 'case 4',        branch: 'success',  from_port: 'case_4' },
    { id: 'e5d', from: 'n5',  to: 'n20', label: 'default',       branch: 'fallback', from_port: 'default' },
    { id: 'e7a', from: 'n7',  to: 'n13', label: 'done',          branch: 'success',  from_port: 'done' },
    { id: 'e9a', from: 'n9',  to: 'n20', label: 'rejected',      branch: 'fallback', from_port: 'rejected' },
    { id: 'e10a',from: 'n10', to: 'n23', label: 'catch',         branch: 'fallback', from_port: 'catch' },
    // Fallback branch from reservation timeout
    { id: 'e20', from: 'n9',  to: 'n20', label: 'timeout',       branch: 'timeout', from_port: 'timeout' },
    { id: 'e21', from: 'n20', to: 'n21' },
    { id: 'e22', from: 'n21', to: 'n22', label: 'retry',         branch: 'success' },
    { id: 'e23', from: 'n22',  to: 'n22b' },
    { id: 'e23a', from: 'n22b', to: 'n23' },
    { id: 'e24', from: 'n23',  to: 'n24' },
  ],
};

// ----- Simulator + trace -----

export interface SimulationInput {
  channel: 'voice' | 'chat' | 'email';
  customer_tier: 'gold' | 'silver' | 'bronze';
  required_skills: string[];
  customer_id: string;
  arrived_at: string;
}

export const MOCK_SIM_INPUT: SimulationInput = {
  channel: 'voice',
  customer_tier: 'gold',
  required_skills: ['skill_voice_tier1', 'skill_billing'],
  customer_id: 'cust_77a91',
  arrived_at: '2026-05-19T09:12:43.000Z',
};

export type StepStatus = 'ok' | 'skipped' | 'fail' | 'timeout';

export interface TraceStep {
  id: string;
  node_id: string;
  node_kind: FlowNodeKind;
  label: string;
  started_at_ms: number;
  duration_ms: number;
  status: StepStatus;
  inputs: Record<string, unknown>;
  outputs: Record<string, unknown>;
  note?: string;
  // 3D-2 control-flow nesting (omitted for flat steps).
  region?: string;
  iteration?: number;
  branch?: number;
  caught?: boolean;
}

export const MOCK_TRACE_STEPS: TraceStep[] = [
  { id: 's1',  node_id: 'n1',  node_kind: 'trigger',      label: 'On Voice Inbound',     started_at_ms: 0.0,   duration_ms: 0.4, status: 'ok',
    inputs:  { channel: 'voice', source: 'pstn', from: '+84 90 123 4567' },
    outputs: { interaction_id: 'int_018f0a', channel: 'voice', from: '+84 90 123 4567' } },
  { id: 's1a', node_id: 'n1a', node_kind: 'get_dtmf',     label: 'IVR menu',             started_at_ms: 0.4,   duration_ms: 4200.0, status: 'ok',
    inputs:  { prompt: 'Press 1 for billing, 2 for tech, 3 for sales, 4 for ops', timeout_sec: 8, max_digits: 1 },
    outputs: { menu_choice: '1', captured_at_ms: 4200.4 },
    note: 'Sim paused on this step until input was submitted. Press the input field above to provide the DTMF digit.' },
  { id: 's2',  node_id: 'n2',  node_kind: 'set_var',      label: 'Extract customer_id',  started_at_ms: 4200.4, duration_ms: 0.2, status: 'ok',
    inputs:  { name: 'customer_id', value_expr: 'parse_caller(from)', from: '+84 90 123 4567' },
    outputs: { customer_id: 'cust_77a91' } },
  { id: 's3',  node_id: 'n3',  node_kind: 'http_request', label: 'Lookup CRM',           started_at_ms: 4200.6, duration_ms: 184.3, status: 'ok',
    inputs:  { method: 'GET', url: 'https://crm.example/customers/cust_77a91', mode: 'async', timeout_ms: 2000 },
    outputs: { status: 200, 'customer.tier': 'gold', 'customer.lang': 'vi', 'customer.lifetime_value': 4820 },
    note: 'Async HTTP — body parsed into nested customer.* vars' },
  { id: 's4',  node_id: 'n4',  node_kind: 'if_else',      label: 'Gold tier?',           started_at_ms: 4384.9, duration_ms: 0.1, status: 'ok',
    inputs:  { expr: 'customer.tier == "gold"', 'customer.tier': 'gold' },
    outputs: { branch: 'yes' } },
  { id: 's5',  node_id: 'n5',  node_kind: 'switch_case',  label: 'Switch on menu',       started_at_ms: 4385.0, duration_ms: 0.1, status: 'ok',
    inputs:  { switch_on: 'menu_choice', menu_choice: '1', cases: ['1', '2', '3', '4'] },
    outputs: { case_taken: '1' } },
  { id: 's6',  node_id: 'n6',  node_kind: 'match_skill',  label: 'Match skill',          started_at_ms: 4385.1, duration_ms: 1.4, status: 'ok',
    inputs:  { required_skills: ['skill_voice_tier1', 'skill_billing'], min_proficiency: 7 },
    outputs: { matched_skills: ['skill_voice_tier1', 'skill_billing'], candidates: ['agent_002', 'agent_006', 'agent_001'], candidate_count: 3 } },
  { id: 's7',  node_id: 'n7',  node_kind: 'loop_for',     label: 'For each candidate',   started_at_ms: 4386.5, duration_ms: 0.1, status: 'ok',
    inputs:  { iter: 'agent in candidates', candidates: ['agent_002', 'agent_006', 'agent_001'], max_iter: 3 },
    outputs: { iteration: 1, current_agent: 'agent_002' } },
  { id: 's8',  node_id: 'n8',  node_kind: 'route_queue',  label: 'Route → VIP',          started_at_ms: 4386.6, duration_ms: 0.8, status: 'ok',
    inputs:  { queue_code: 'queue_vip', priority: 10 },
    outputs: { queue_id: '01919f00-0002-7000-8000-200000000005', queued_at_ms: 4386.6, ahead_in_queue: 0 } },
  { id: 's9',  node_id: 'n9',  node_kind: 'reservation',  label: 'Reserve agent',        started_at_ms: 4387.4, duration_ms: 9.1, status: 'ok',
    inputs:  { offer_timeout_sec: 12, retry: 2, candidate: 'agent_002' },
    outputs: { agent_id: '01919f00-0003-7000-8000-300000000002', agent_name: 'Bao Tran', offered_at_ms: 4387.4, accepted_at_ms: 4396.5 } },
  { id: 's10', node_id: 'n10', node_kind: 'try_catch',    label: 'Try: Notify CRM',      started_at_ms: 4396.5, duration_ms: 0.1, status: 'ok',
    inputs:  { catch_kinds: ['http_error', 'timeout'] },
    outputs: { entering: 'try_block' } },
  { id: 's11', node_id: 'n11', node_kind: 'effect',       label: 'POST /crm/notify',     started_at_ms: 4396.6, duration_ms: 142.4, status: 'ok',
    inputs:  { method: 'POST', url: 'https://crm.example/hooks/route', mode: 'sync', body_keys: ['interaction_id', 'agent_id', 'queue_id'] },
    outputs: { status: 200, request_id: 'req_018f0a91' },
    note: 'Effect MOCKED — set Effects mode to Live to actually fire' },
  { id: 's12', node_id: 'n12', node_kind: 'log',          label: 'Audit log',            started_at_ms: 4539.0, duration_ms: 0.3, status: 'ok',
    inputs:  { level: 'info', message: 'routing.completed' },
    outputs: { log_id: 'log_018f0a91c7_main', written_bytes: 412 } },
  { id: 's13', node_id: 'n13', node_kind: 'end',          label: 'Done',                 started_at_ms: 4539.3, duration_ms: 0.0, status: 'ok',
    inputs:  {},
    outputs: { outcome: 'routed', total_ms: 4539.3 } },
];

export interface SimulationOutcome {
  outcome: 'routed' | 'fallback' | 'failed';
  agent_id: string | null;
  agent_name: string | null;
  queue_code: string | null;
  total_latency_ms: number;
  used_fallback: boolean;
  effects_fired: number;
  trace_id: string;
}

export const MOCK_SIM_OUTCOME: SimulationOutcome = {
  outcome: 'routed',
  agent_id: '01919f00-0003-7000-8000-300000000002',
  agent_name: 'Bao Tran',
  queue_code: 'queue_vip',
  total_latency_ms: 14.5,
  used_fallback: false,
  effects_fired: 1,
  trace_id: 'trace_018f0a91c7',
};

// ----- Init vars + variable bag (live simulator on canvas) -----

export interface InitVar {
  key: string;
  value: string;
  type: 'string' | 'number' | 'boolean' | 'json';
  source: 'trigger' | 'user';   // trigger-derived = read-only; user-added = editable
}

export const MOCK_INIT_VARS: InitVar[] = [
  { key: 'channel',        value: 'voice',         type: 'string', source: 'trigger' },
  { key: 'source',         value: 'pstn',          type: 'string', source: 'trigger' },
  { key: 'from',           value: '+84 90 123 4567', type: 'string', source: 'trigger' },
  { key: 'customer.tier',  value: 'gold',          type: 'string', source: 'user' },
  { key: 'customer.id',    value: 'cust_77a91',    type: 'string', source: 'user' },
];

export interface VarBagEntry {
  key: string;
  value: unknown;
  set_at_step: string;            // step id that wrote this key (or 'init')
  set_at_node_label: string;      // human label for display
}

// Compute the running variable bag at a given step index of a given trace.
// Trace is parameterised so the success and failure scenarios produce
// independent bags — passing MOCK_TRACE_STEPS unconditionally caused the
// var bag to disagree with the canvas in the failure scenario.
export function computeVarBag(stepIdx: number, trace: TraceStep[] = MOCK_TRACE_STEPS, initVars: InitVar[] = MOCK_INIT_VARS): VarBagEntry[] {
  const bag = new Map<string, VarBagEntry>();
  // Seed from the caller's LIVE init vars (what the run actually uses), not the
  // hardcoded mock — otherwise editing customer.tier to "gold111" never shows.
  for (const v of initVars) {
    bag.set(v.key, { key: v.key, value: v.value, set_at_step: 'init', set_at_node_label: 'Init vars' });
  }
  // Only set_var/compute actually write to the variable bag; every other node's
  // `outputs` are step I/O metadata (candidates, skill, attempts…) shown in the
  // Step I/O panel — NOT variables. set_var reports {name,value} and compute
  // {result,var}, so map those to a single `<name> = <value>` entry instead of
  // surfacing the metadata keys as bogus variables.
  for (let i = 0; i <= stepIdx && i < trace.length; i++) {
    const step = trace[i]!;
    const o = step.outputs;
    if (step.node_kind === 'set_var') {
      const name = o['name'];
      if (typeof name === 'string') {
        bag.set(name, { key: name, value: o['value'], set_at_step: step.id, set_at_node_label: step.label });
      }
    } else if (step.node_kind === 'compute') {
      const name = o['var'];
      if (typeof name === 'string') {
        bag.set(name, { key: name, value: o['result'], set_at_step: step.id, set_at_node_label: step.label });
      }
    } else if (typeof o['save_as'] === 'string' && o['save_as'] !== '') {
      // Response-capture nodes (http_request) write their (mock) response into
      // the named variable — surface it like set_var/compute, not as I/O metadata.
      const name = o['save_as'];
      bag.set(name, { key: name, value: o['response'], set_at_step: step.id, set_at_node_label: step.label });
    }
  }
  return Array.from(bag.values());
}

// Failing-scenario trace — happy path up to the try_catch (s1..s10),
// then the effect step (POST /crm/notify) times out. Demonstrates the
// inline error-fix UI in the run panel.
export const MOCK_TRACE_STEPS_FAIL: TraceStep[] = [
  ...MOCK_TRACE_STEPS.slice(0, 10),
  { id: 's11f', node_id: 'n11', node_kind: 'effect',  label: 'POST /crm/notify',  started_at_ms: 4396.6, duration_ms: 1502.0, status: 'fail',
    inputs:  { method: 'POST', url: 'https://crm.example/hooks/route', mode: 'sync', timeout_ms: 1500 },
    outputs: { status: 0, error: 'connect ETIMEDOUT crm.example:443', request_id: 'req_018f0a91' },
    note: 'Effect failed — CRM webhook timed out at 1.5s. Fix the URL or raise timeout, then resume.' },
];

// ----- Test cases (saved scenarios for regression replay) -----

// Captured combination of init vars + scenario + user-input choices so a
// developer can replay a path without manually stepping through each time.
// Real engine would diff the actual trace against the expected outcome and
// surface PASS/FAIL on each replay; we surface that as last_outcome here.
export interface TestCase {
  id: string;
  name: string;
  description: string;
  scenario: 'success' | 'fail';
  // Init-var overrides applied on replay (only keys present override; rest
  // come from MOCK_INIT_VARS).
  init_var_overrides: Record<string, string>;
  // Captured wait_input values keyed by step id (e.g. s1a → "1" for the
  // IVR menu choice). On replay these get auto-submitted when the sim
  // pauses on the matching step.
  captured_inputs: Record<string, string>;
  created_at: string;
  // Result of last replay; 'never_run' means it's a freshly-saved case
  // that hasn't been replayed since the flow was last edited.
  last_outcome: 'pass' | 'fail' | 'never_run';
}

export const MOCK_TEST_CASES: TestCase[] = [
  {
    id: 'tc_018f0a91c7_1',
    name: 'Gold customer — billing IVR press 1',
    description: 'Gold tier voice caller selects billing menu → routed to VIP queue → Bao Tran accepts',
    scenario: 'success',
    init_var_overrides: { 'customer.tier': 'gold', 'customer.id': 'cust_77a91' },
    captured_inputs: { s1a: '1' },
    created_at: '2026-05-18T09:42:00Z',
    last_outcome: 'pass',
  },
  {
    id: 'tc_018f0a91c7_2',
    name: 'Silver customer — tech IVR press 2',
    description: 'Silver tier caller, picks tech support — should drop into the no-fast-track branch',
    scenario: 'success',
    init_var_overrides: { 'customer.tier': 'silver' },
    captured_inputs: { s1a: '2' },
    created_at: '2026-05-18T14:11:00Z',
    last_outcome: 'never_run',
  },
  {
    id: 'tc_018f0a91c7_3',
    name: 'CRM webhook 500 — fallback path',
    description: 'Gold customer, all routing succeeds, but Notify CRM times out; supervisor approves overflow',
    scenario: 'fail',
    init_var_overrides: { 'customer.tier': 'gold' },
    captured_inputs: { s1a: '1', s21: 'retry' },
    created_at: '2026-05-19T08:32:00Z',
    last_outcome: 'fail',
  },
];
