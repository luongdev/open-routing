import { html } from 'lit';
import '../../primitives/or-switch.js';
import '../../primitives/or-checkbox.js';

export const switchCheckboxSlot = html`
  <div style="display:flex; flex-direction:column; gap:16px;">
    <div>
      <strong style="font-size:12px; text-transform:uppercase; letter-spacing:.05em; color:var(--or-color-text-muted,#737373)">Switch</strong>
      <div style="display:flex; flex-wrap:wrap; gap:12px; margin-top:8px; align-items:center;">
        <or-switch></or-switch>
        <or-switch checked></or-switch>
        <or-switch disabled></or-switch>
        <or-switch label="Notifications enabled"></or-switch>
        <or-switch label="Email + SMS" helper-text="Receive email + SMS"></or-switch>
      </div>
    </div>
    <div>
      <strong style="font-size:12px; text-transform:uppercase; letter-spacing:.05em; color:var(--or-color-text-muted,#737373)">Checkbox</strong>
      <div style="display:flex; flex-wrap:wrap; gap:12px; margin-top:8px; align-items:center;">
        <or-checkbox></or-checkbox>
        <or-checkbox checked></or-checkbox>
        <or-checkbox indeterminate></or-checkbox>
        <or-checkbox disabled></or-checkbox>
        <or-checkbox label="Accept terms"></or-checkbox>
      </div>
    </div>
  </div>
`;
