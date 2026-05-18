import { html, TemplateResult } from 'lit';
import '../../primitives/or-card.js';

export const cardSlot: TemplateResult = html`
  <div style="display:flex;flex-direction:column;gap:16px;">
    <or-card>
      <p style="margin:0;">Simple card with body content.</p>
    </or-card>

    <or-card>
      <span slot="header">Card Title</span>
      <p style="margin:0;">Body with header and footer.</p>
      <span slot="footer">Footer action</span>
    </or-card>

    <or-card hoverable>
      <p style="margin:0;">Hoverable card — elevation on hover.</p>
    </or-card>

    <div style="display:grid;grid-template-columns:repeat(2,1fr);gap:12px;">
      <or-card padding="sm">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <div>
            <div style="font-size:28px;font-weight:700;line-height:1;">24</div>
            <div style="font-size:12px;color:var(--or-color-text-muted,#737373);margin-top:4px;">Today's Total</div>
          </div>
          <span style="font-size:24px;">📅</span>
        </div>
      </or-card>
      <or-card padding="sm">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <div>
            <div style="font-size:28px;font-weight:700;line-height:1;">12</div>
            <div style="font-size:12px;color:var(--or-color-text-muted,#737373);margin-top:4px;">In-Person</div>
          </div>
          <span style="font-size:24px;">🏥</span>
        </div>
      </or-card>
      <or-card padding="sm">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <div>
            <div style="font-size:28px;font-weight:700;line-height:1;">8</div>
            <div style="font-size:12px;color:var(--or-color-text-muted,#737373);margin-top:4px;">Telehealth</div>
          </div>
          <span style="font-size:24px;">💻</span>
        </div>
      </or-card>
      <or-card padding="sm">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <div>
            <div style="font-size:28px;font-weight:700;line-height:1;">4</div>
            <div style="font-size:12px;color:var(--or-color-text-muted,#737373);margin-top:4px;">Cancelled</div>
          </div>
          <span style="font-size:24px;">❌</span>
        </div>
      </or-card>
    </div>
  </div>
`;
