import { html } from 'lit';
import '../../primitives/or-icon.js';
import { ICON_NAMES } from '../../primitives/icon-names.js';

export const iconSlot = html`
  <div style="display:grid; grid-template-columns: repeat(4, 1fr); gap: 16px;">
    ${ICON_NAMES.map(n => html`
      <div style="display:flex; flex-direction:column; align-items:center; gap:4px;">
        <or-icon name=${n} size="20" label=${n}></or-icon>
        <code style="font-size:11px; color:var(--or-color-text-muted, #737373)">${n}</code>
      </div>
    `)}
  </div>
`;
