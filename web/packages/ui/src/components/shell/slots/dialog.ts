import { html } from 'lit';
import type { TemplateResult } from 'lit';
import '../../primitives/or-dialog.js';
import '../../primitives/or-button.js';

export function dialogSlot(): TemplateResult {
  let dialogOpen = false;

  const toggle = (open: boolean) => {
    dialogOpen = open;
    const dlg = document.querySelector('or-dialog[data-demo="confirm-delete"]') as any;
    if (dlg) dlg.open = open;
  };

  return html`
    <div style="display:flex;flex-direction:column;gap:12px;">
      <or-button variant="destructive" @click=${() => toggle(true)}>
        Open Delete Dialog
      </or-button>
      <or-dialog
        data-demo="confirm-delete"
        size="sm"
        @or-dialog-close=${() => { dialogOpen = false; }}
      >
        <span slot="header" style="font-weight:600;font-size:16px;">Delete Agent?</span>
        <p style="margin:0;color:var(--or-color-text-muted,#737373);">
          This action cannot be undone. The agent and all its configuration
          will be permanently deleted.
        </p>
        <div slot="footer" style="display:flex;gap:8px;justify-content:flex-end;width:100%;">
          <or-button variant="default" @click=${() => toggle(false)}>Cancel</or-button>
          <or-button variant="destructive" @click=${() => toggle(false)}>Delete</or-button>
        </div>
      </or-dialog>
    </div>
  `;
}
