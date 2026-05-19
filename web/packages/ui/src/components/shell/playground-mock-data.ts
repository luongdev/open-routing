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
