import { html } from 'lit';
import type { TemplateResult } from 'lit';
import '../../primitives/or-table.js';
import '../../primitives/or-badge.js';
import '../../primitives/or-icon.js';

const USERS = [
  { initials: 'AS', name: 'Aigars Silkalns',  email: 'aigars@company.com',   role: 'Admin',   dept: 'Engineering', status: 'Active',   lastActive: '2 hours ago' },
  { initials: 'ML', name: 'Mara Liepa',        email: 'mara@company.com',     role: 'Agent',   dept: 'Support',     status: 'Active',   lastActive: '5 min ago' },
  { initials: 'JK', name: 'Janis Kalnins',     email: 'janis@company.com',    role: 'Agent',   dept: 'Support',     status: 'Inactive', lastActive: '3 days ago' },
  { initials: 'LB', name: 'Laura Berzina',     email: 'laura@company.com',    role: 'Admin',   dept: 'Product',     status: 'Active',   lastActive: '1 hour ago' },
  { initials: 'RP', name: 'Roberts Petersons', email: 'roberts@company.com',  role: 'Agent',   dept: 'Sales',       status: 'Active',   lastActive: '20 min ago' },
  { initials: 'AO', name: 'Anna Ozola',        email: 'anna@company.com',     role: 'Agent',   dept: 'Billing',     status: 'Inactive', lastActive: '1 week ago' },
  { initials: 'EZ', name: 'Edgars Zitars',     email: 'edgars@company.com',   role: 'Agent',   dept: 'Engineering', status: 'Active',   lastActive: '30 min ago' },
  { initials: 'IK', name: 'Ilze Kalva',        email: 'ilze@company.com',     role: 'Agent',   dept: 'Support',     status: 'Active',   lastActive: '45 min ago' },
  { initials: 'MV', name: 'Martins Vitols',    email: 'martins@company.com',  role: 'Admin',   dept: 'Operations',  status: 'Active',   lastActive: '10 min ago' },
  { initials: 'DL', name: 'Dace Liepa',        email: 'dace@company.com',     role: 'Agent',   dept: 'Support',     status: 'Inactive', lastActive: '2 weeks ago' },
];

const AVATAR_STYLE = [
  'display:inline-flex;align-items:center;justify-content:center;',
  'width:32px;height:32px;border-radius:50%;',
  'background:var(--uk-primary,#0d8b96);color:#fff;',
  'font-size:12px;font-weight:600;flex-shrink:0;',
].join('');

export function tableSlot(): TemplateResult {
  return html`
    <or-table hover>
      <tr slot="head">
        <th>Name</th>
        <th>Role</th>
        <th>Department</th>
        <th>Status</th>
        <th>Last Active</th>
        <th></th>
      </tr>
      ${USERS.map(u => html`
        <tr>
          <td>
            <div style="display:flex;align-items:center;gap:10px;">
              <span style=${AVATAR_STYLE}>${u.initials}</span>
              <div>
                <div style="font-weight:500;">${u.name}</div>
                <div style="font-size:12px;color:var(--or-color-text-muted,#737373);">${u.email}</div>
              </div>
            </div>
          </td>
          <td>
            <or-badge variant=${u.role === 'Admin' ? 'destructive' : 'default'} pill>
              ${u.role}
            </or-badge>
          </td>
          <td>${u.dept}</td>
          <td>
            <or-badge variant=${u.status === 'Active' ? 'success' : 'default'} pill>
              ${u.status}
            </or-badge>
          </td>
          <td style="color:var(--or-color-text-muted,#737373);font-size:13px;">${u.lastActive}</td>
          <td>
            <div style="display:flex;gap:8px;">
              <or-icon name="pencil" size="14"></or-icon>
              <or-icon name="trash-2" size="14"></or-icon>
            </div>
          </td>
        </tr>
      `)}
    </or-table>
  `;
}
