import { html } from 'lit';
import type { TemplateResult } from 'lit';
import { ref, createRef } from 'lit/directives/ref.js';
import type { OrDialog } from '../../primitives/or-dialog.js';
import '../../primitives/or-dialog.js';
import '../../primitives/or-button.js';

export function dialogSlot(): TemplateResult {
  const dialogRef = createRef<OrDialog>();

  return html`
    <div style="display:flex;flex-direction:column;gap:12px;">
      <or-button variant="destructive" @click=${() => {
        if (dialogRef.value) dialogRef.value.open = true;
      }}>
        Open Delete Dialog
      </or-button>
      <or-dialog
        ${ref(dialogRef)}
        size="sm"
        @or-dialog-close=${() => {
          if (dialogRef.value) dialogRef.value.open = false;
        }}
      >
        <span slot="header" style="font-weight:600;font-size:16px;">Delete Agent?</span>
        <p style="margin:0;color:var(--or-color-text-muted,#737373);">
          This action cannot be undone. The agent and all its configuration
          will be permanently deleted.
        </p>
        <div slot="footer" style="display:flex;gap:8px;justify-content:flex-end;width:100%;">
          <or-button variant="default" @click=${() => {
            if (dialogRef.value) dialogRef.value.open = false;
          }}>Cancel</or-button>
          <or-button variant="destructive" @click=${() => {
            if (dialogRef.value) dialogRef.value.open = false;
          }}>Delete</or-button>
        </div>
      </or-dialog>
    </div>
  `;
}
