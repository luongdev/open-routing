import { html, type TemplateResult } from 'lit';
import type { ApiClient } from '../../api/client.js';
import { MOCK_ORG_ID, MOCK_AGENTS, MOCK_SKILLS, MOCK_QUEUES, MOCK_CHANNELS, MOCK_ADAPTERS, MOCK_BREAK_REASONS, MOCK_IMPORT_JOB, MOCK_FLOWS } from './playground-mock-data.js';
import '../flows/flow-list.js';
import '../flows/flow-builder.js';
import '../flows/trace-viewer.js';

export interface ScreenEntry {
  id: string;
  label: string;
  group: string;
  render: (client: ApiClient) => TemplateResult;
}

const AGENT_ID      = MOCK_AGENTS[0]!.id;
const SKILL_ID      = MOCK_SKILLS[0]!.id;
const QUEUE_ID      = MOCK_QUEUES[0]!.id;
const CHANNEL_ID    = MOCK_CHANNELS[0]!.id;
const ADAPTER_ID    = MOCK_ADAPTERS[0]!.id;
const BREAK_ID      = MOCK_BREAK_REASONS[0]!.id;
const IMPORT_JOB_ID = MOCK_IMPORT_JOB.id;

export const SCREENS: ReadonlyArray<ScreenEntry> = [
  // Agents
  {
    id: 'agents-list',
    label: 'Agent List',
    group: 'Agents',
    render: (c) => html`<or-agent-list .client=${c} .orgId=${MOCK_ORG_ID}></or-agent-list>`,
  },
  {
    id: 'agents-create',
    label: 'Create Agent',
    group: 'Agents',
    render: (c) => html`<or-agent-form .client=${c} .orgId=${MOCK_ORG_ID}></or-agent-form>`,
  },
  {
    id: 'agents-detail',
    label: 'Agent Detail',
    group: 'Agents',
    render: (c) => html`<or-agent-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${AGENT_ID}></or-agent-detail>`,
  },

  // Skills
  {
    id: 'skills-list',
    label: 'Skill List',
    group: 'Skills',
    render: (c) => html`<or-skill-list .client=${c} .orgId=${MOCK_ORG_ID}></or-skill-list>`,
  },
  {
    id: 'skills-create',
    label: 'Create Skill',
    group: 'Skills',
    render: (c) => html`<or-skill-form .client=${c} .orgId=${MOCK_ORG_ID}></or-skill-form>`,
  },
  {
    id: 'skills-detail',
    label: 'Skill Detail',
    group: 'Skills',
    render: (c) => html`<or-skill-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${SKILL_ID}></or-skill-detail>`,
  },

  // Queues
  {
    id: 'queues-list',
    label: 'Queue List',
    group: 'Queues',
    render: (c) => html`<or-queue-list .client=${c} .orgId=${MOCK_ORG_ID}></or-queue-list>`,
  },
  {
    id: 'queues-create',
    label: 'Create Queue',
    group: 'Queues',
    render: (c) => html`<or-queue-form .client=${c} .orgId=${MOCK_ORG_ID}></or-queue-form>`,
  },
  {
    id: 'queues-detail',
    label: 'Queue Detail',
    group: 'Queues',
    render: (c) => html`<or-queue-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${QUEUE_ID}></or-queue-detail>`,
  },

  // Channels
  {
    id: 'channels-list',
    label: 'Channel List',
    group: 'Channels',
    render: (c) => html`<or-channel-list .client=${c} .orgId=${MOCK_ORG_ID}></or-channel-list>`,
  },
  {
    id: 'channels-create',
    label: 'Create Channel',
    group: 'Channels',
    render: (c) => html`<or-channel-form .client=${c} .orgId=${MOCK_ORG_ID}></or-channel-form>`,
  },
  {
    id: 'channels-detail',
    label: 'Channel Detail',
    group: 'Channels',
    render: (c) => html`<or-channel-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${CHANNEL_ID}></or-channel-detail>`,
  },

  // Adapters
  {
    id: 'adapters-list',
    label: 'Adapter List',
    group: 'Adapters',
    render: (c) => html`<or-adapter-list .client=${c} .orgId=${MOCK_ORG_ID}></or-adapter-list>`,
  },
  {
    id: 'adapters-create',
    label: 'Create Adapter',
    group: 'Adapters',
    render: (c) => html`<or-adapter-form .client=${c} .orgId=${MOCK_ORG_ID}></or-adapter-form>`,
  },
  {
    id: 'adapters-detail',
    label: 'Adapter Detail',
    group: 'Adapters',
    render: (c) => html`<or-adapter-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${ADAPTER_ID}></or-adapter-detail>`,
  },

  // Break Reasons
  {
    id: 'break-reasons-list',
    label: 'Break Reason List',
    group: 'Break Reasons',
    render: (c) => html`<or-break-reason-list .client=${c} .orgId=${MOCK_ORG_ID}></or-break-reason-list>`,
  },
  {
    id: 'break-reasons-create',
    label: 'Create Break Reason',
    group: 'Break Reasons',
    render: (c) => html`<or-break-reason-form .client=${c} .orgId=${MOCK_ORG_ID}></or-break-reason-form>`,
  },
  {
    id: 'break-reasons-detail',
    label: 'Break Reason Detail',
    group: 'Break Reasons',
    render: (c) => html`<or-break-reason-detail .client=${c} .orgId=${MOCK_ORG_ID} .entityId=${BREAK_ID}></or-break-reason-detail>`,
  },

  // Operations
  {
    id: 'agent-status',
    label: 'Agent Status',
    group: 'Operations',
    render: (c) => html`<or-agent-status-list .client=${c} .orgId=${MOCK_ORG_ID}></or-agent-status-list>`,
  },
  {
    id: 'import-page',
    label: 'Bulk Import',
    group: 'Operations',
    render: (c) => html`<or-import-page .client=${c} .orgId=${MOCK_ORG_ID} .baseURL=${''}></or-import-page>`,
  },
  {
    id: 'import-result',
    label: 'Import Result',
    group: 'Operations',
    render: (c) => html`<or-import-result .client=${c} .orgId=${MOCK_ORG_ID} .importId=${IMPORT_JOB_ID}></or-import-result>`,
  },

  // vNext Preview — v0.2 workflow screens. flow-list + flow-builder are REAL
  // API-backed components driven by the playground mock client; only
  // trace-viewer is still a static mock (GET /traces is a 501 stub until Layer 3).
  {
    id: 'vnext-flow-list',
    label: 'Flow List',
    group: 'vNext Preview',
    render: (c) => html`<or-flow-list .client=${c} .orgId=${MOCK_ORG_ID}></or-flow-list>`,
  },
  {
    id: 'vnext-flow-builder',
    label: 'Flow Builder',
    group: 'vNext Preview',
    render: (c) => html`<or-flow-builder .client=${c} .orgId=${MOCK_ORG_ID} .flowId=${MOCK_FLOWS[0]!.id}></or-flow-builder>`,
  },
  {
    id: 'vnext-flow-create',
    label: 'Create Flow',
    group: 'vNext Preview',
    render: (c) => html`<or-flow-builder .client=${c} .orgId=${MOCK_ORG_ID} .flowId=${''}></or-flow-builder>`,
  },
  // Simulator merged into Flow Builder (sim mode toggle in toolbar). The
  // standalone 3-pane runner was the wrong shape — authoring + running
  // need to share canvas state.
  {
    id: 'vnext-trace-viewer',
    label: 'Trace Viewer',
    group: 'vNext Preview',
    render: (c) => html`<or-trace-viewer .client=${c} .orgId=${MOCK_ORG_ID}></or-trace-viewer>`,
  },
];
