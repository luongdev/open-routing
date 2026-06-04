import { html, render } from 'lit';
import './or-dialog.js';
import './or-button.js';
import type { OrDialog } from './or-dialog.js';

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
        <span slot="footer">
          <or-button variant="default" @click=${() => finish(false)}>${opts.cancelLabel ?? 'Cancel'}</or-button>
          <or-button variant=${opts.danger ? 'destructive' : 'primary'} @click=${() => finish(true)}>
            ${opts.confirmLabel ?? 'Confirm'}
          </or-button>
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
