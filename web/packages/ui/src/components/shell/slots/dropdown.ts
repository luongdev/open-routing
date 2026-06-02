import { html } from 'lit';
import type { TemplateResult } from 'lit';
import '../../primitives/or-dropdown.js';
import '../../primitives/or-icon.js';

const userMenuItems = [
  { id: 'profile', label: 'Profile', icon: 'user-cog' },
  { id: 'settings', label: 'Settings', icon: 'pencil' },
  { divider: true as const },
  { id: 'signout', label: 'Sign Out', destructive: true },
];

const rowActionItems = [
  { id: 'edit', label: 'Edit', icon: 'pencil' },
  { id: 'duplicate', label: 'Duplicate', icon: 'file-text' },
  { divider: true as const },
  { id: 'delete', label: 'Delete', icon: 'trash-2', destructive: true },
];

export function dropdownSlot(): TemplateResult {
  return html`
    <div style="display:flex;gap:32px;align-items:flex-start;flex-wrap:wrap;">
      <div>
        <p style="margin:0 0 8px;font-size:12px;color:var(--or-color-text-muted,#737373);text-transform:uppercase;letter-spacing:.05em;">User Menu</p>
        <or-dropdown .items=${userMenuItems} align="left" style="cursor:pointer;">
          <button class="uk-button uk-button-default uk-button-small" type="button" style="pointer-events:none;">
            <or-icon name="user-cog" size="14" style="margin-inline-end:4px;vertical-align:middle"></or-icon>
            Profile
          </button>
        </or-dropdown>
      </div>
      <div>
        <p style="margin:0 0 8px;font-size:12px;color:var(--or-color-text-muted,#737373);text-transform:uppercase;letter-spacing:.05em;">Row Actions</p>
        <or-dropdown .items=${rowActionItems} align="left" style="cursor:pointer;">
          <button class="uk-button uk-button-default uk-button-small" type="button" style="pointer-events:none;" aria-label="Row actions">
            <or-icon name="more-vertical" size="16"></or-icon>
          </button>
        </or-dropdown>
      </div>
    </div>
  `;
}
