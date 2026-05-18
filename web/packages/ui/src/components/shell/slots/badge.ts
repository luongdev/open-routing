import { html } from 'lit';
import type { TemplateResult } from 'lit';
import '../../primitives/or-badge.js';

export function badgeSlot(): TemplateResult {
  return html`
    <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:center;margin-bottom:12px;">
      <or-badge variant="default">Default</or-badge>
      <or-badge variant="success">Success</or-badge>
      <or-badge variant="warning">Warning</or-badge>
      <or-badge variant="destructive">Destructive</or-badge>
      <or-badge variant="info">Info</or-badge>
      <or-badge variant="primary">Primary</or-badge>
    </div>
    <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:center;margin-bottom:12px;">
      <or-badge variant="success" pill>Success Pill</or-badge>
      <or-badge variant="warning" pill>Warning Pill</or-badge>
      <or-badge variant="primary" pill>Primary Pill</or-badge>
    </div>
    <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:center;margin-bottom:12px;">
      <or-badge variant="success" dot></or-badge>
      <or-badge variant="warning" dot></or-badge>
      <or-badge variant="destructive" dot></or-badge>
    </div>
    <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:center;">
      <or-badge variant="success" pill>Active</or-badge>
      <or-badge variant="default" pill>Inactive</or-badge>
      <or-badge variant="primary" pill>In Consultation</or-badge>
      <or-badge variant="warning" pill>Moderate</or-badge>
      <or-badge variant="success" pill>Mild</or-badge>
    </div>
  `;
}
