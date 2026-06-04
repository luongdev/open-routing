import { html, render } from 'lit';
import './or-dialog.js';
import type { OrDialog } from './or-dialog.js';

// Plain buttons (not or-button) styled inline with theme vars: the helper
// renders into the dialog's LIGHT DOM, where a custom element's slotted text is
// fragile, so a self-contained button is more robust.
const BTN_BASE =
  'padding:7px 16px;border-radius:8px;font-size:13px;font-weight:600;cursor:pointer;font-family:inherit;line-height:1.2;border:1px solid transparent;';
const BTN_CANCEL = BTN_BASE + 'background:var(--card,#fff);border-color:var(--border,#e5e5e5);color:var(--foreground,#111);';
const BTN_DANGER = BTN_BASE + 'background:var(--destructive,#dc2626);color:#fff;';
const BTN_PRIMARY = BTN_BASE + 'background:var(--primary,#2563eb);color:var(--primary-foreground,#fff);';

export interface ConfirmOptions {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Style the confirm button as destructive (delete/irreversible). */
  danger?: boolean;
}

// Promise-based styled confirmation, replacing window.confirm. Mounts a
// transient <or-dialog> on document.body, resolves true (confirm) / false
// (cancel, Escape, or backdrop click), then unmounts. Use: `if (!(await
// confirmDialog({...}))) return;`.
export function confirmDialog(opts: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    const dialog = document.createElement('or-dialog') as OrDialog;
    dialog.size = 'sm';
    dialog.centered = true;

    let settled = false;
    const finish = (v: boolean): void => {
      if (settled) return;
      settled = true;
      dialog.open = false;
      queueMicrotask(() => dialog.remove());
      resolve(v);
    };

    // Escape / backdrop click close the dialog → treat as cancel.
    dialog.addEventListener('or-dialog-close', () => finish(false));

    render(
      html`
        <span slot="header" style="font-weight:600;font-size:0.95rem">${opts.title}</span>
        <p style="margin:0;line-height:1.55;color:var(--muted-foreground, #555)">${opts.message}</p>
        <span slot="footer" style="display:flex;gap:8px;justify-content:flex-end">
          <button type="button" data-action="cancel" style=${BTN_CANCEL} @click=${() => finish(false)}>
            ${opts.cancelLabel ?? 'Cancel'}
          </button>
          <button type="button" data-action="confirm" style=${opts.danger ? BTN_DANGER : BTN_PRIMARY} @click=${() => finish(true)}>
            ${opts.confirmLabel ?? 'Confirm'}
          </button>
        </span>
      `,
      dialog,
    );

    document.body.appendChild(dialog);
    dialog.open = true;
  });
}

// confirmDelete is the common "Delete X? This cannot be undone." prompt.
export function confirmDelete(name: string, kind = 'item'): Promise<boolean> {
  return confirmDialog({
    title: `Delete ${kind}?`,
    message: `Delete “${name}”? This cannot be undone.`,
    confirmLabel: 'Delete',
    danger: true,
  });
}
