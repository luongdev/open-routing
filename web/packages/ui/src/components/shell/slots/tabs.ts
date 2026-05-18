import { html } from 'lit';
import type { TemplateResult } from 'lit';
import '../../primitives/or-tabs.js';
import type { Tab } from '../../primitives/or-tabs.js';

const HEALTH_TABS: Tab[] = [
  { id: 'summary',  label: 'Summary' },
  { id: 'history',  label: 'History' },
  { id: 'labs',     label: 'Labs' },
  { id: 'imaging',  label: 'Imaging' },
  { id: 'notes',    label: 'Notes' },
];

const SETTINGS_TABS: Tab[] = [
  { id: 'general',       label: 'General' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'security',      label: 'Security' },
];

export function tabsSlot(): TemplateResult {
  return html`
    <or-tabs
      .tabs=${HEALTH_TABS}
      active-tab="summary"
      style="display:block;margin-bottom:24px;"
    >
      <div slot="tab-summary">Summary content</div>
      <div slot="tab-history">History content</div>
      <div slot="tab-labs">Labs content</div>
      <div slot="tab-imaging">Imaging content</div>
      <div slot="tab-notes">Notes content</div>
    </or-tabs>
    <or-tabs
      .tabs=${SETTINGS_TABS}
      active-tab="general"
      style="display:block;"
    >
      <div slot="tab-general">General settings</div>
      <div slot="tab-notifications">Notification settings</div>
      <div slot="tab-security">Security settings</div>
    </or-tabs>
  `;
}
