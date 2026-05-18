import { html } from 'lit';
import '../../primitives/or-button.js';

export const buttonSlot = html`
  <div style="display:flex; flex-direction:column; gap:12px;">
    ${(['default', 'primary', 'secondary', 'ghost', 'destructive'] as const).map(v => html`
      <div style="display:flex; gap:8px; align-items:center;">
        ${(['sm', 'md', 'lg'] as const).map(s => html`
          <or-button variant=${v} size=${s}>${v}/${s}</or-button>
        `)}
      </div>
    `)}
    <div style="display:flex; gap:8px;">
      <or-button variant="primary" .disabled=${true}>Disabled</or-button>
      <or-button variant="primary" .loading=${true}>Loading</or-button>
    </div>
  </div>
`;
