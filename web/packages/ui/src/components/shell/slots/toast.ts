import { html } from 'lit';
import type { TemplateResult } from 'lit';
import { notify, notifyError } from '../../primitives/notify.js';

export function toastSlot(): TemplateResult {
  return html`
    <div style="display:flex;flex-direction:column;gap:8px;">
      <button
        class="uk-button uk-button-default uk-button-small"
        @click=${() => notify({ message: 'Background sync complete', variant: 'info' })}
      >Info</button>
      <button
        class="uk-button uk-button-primary uk-button-small"
        @click=${() => notify({ message: 'Saved successfully', variant: 'success' })}
      >Success</button>
      <button
        class="uk-button uk-button-secondary uk-button-small"
        @click=${() => notify({ message: 'Unsaved changes', variant: 'warning' })}
      >Warning</button>
      <button
        class="uk-button uk-button-danger uk-button-small"
        @click=${() => notify({ message: 'Delete failed', variant: 'destructive' })}
      >Destructive</button>
      <button
        class="uk-button uk-button-danger uk-button-small"
        @click=${() => notifyError(new Error('Sample error message'))}
      >Error helper</button>
    </div>
  `;
}
