import { describe, it, expect, vi, afterEach } from 'vitest';

import './agents/agent-form.js';
import './skills/skill-form.js';
import './queues/queue-form.js';
import './channels/channel-form.js';
import './adapters/adapter-form.js';
import './break-reasons/break-reason-form.js';

type CatalogCreateCase = {
  tag: string;
  nameSelector: string;
};

const CASES: CatalogCreateCase[] = [
  { tag: 'or-agent-form', nameSelector: '#agent-name' },
  { tag: 'or-skill-form', nameSelector: '#skill-name' },
  { tag: 'or-queue-form', nameSelector: '#queue-name' },
  { tag: 'or-channel-form', nameSelector: '#ch-name' },
  { tag: 'or-adapter-form', nameSelector: '#adapter-name' },
  { tag: 'or-break-reason-form', nameSelector: '#br-name' },
];

async function settle(el: HTMLElement): Promise<void> {
  await (el as any).updateComplete;
  await new Promise((resolve) => setTimeout(resolve, 0));
  await (el as any).updateComplete;
}

function codeField(el: HTMLElement): any {
  return el.shadowRoot!.querySelector('or-code-input') as any;
}

function inputName(el: HTMLElement, selector: string, value: string): void {
  const input = el.shadowRoot!.querySelector(selector) as HTMLInputElement | null;
  expect(input).toBeTruthy();
  input!.value = value;
  input!.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
}

describe('catalog create code fields', () => {
  afterEach(() => {
    document.body.replaceChildren();
    vi.restoreAllMocks();
  });

  for (const item of CASES) {
    it(`${item.tag} saves or cancels custom code edits`, async () => {
      const el = document.createElement(item.tag);
      const mockPost = vi.fn();
      (el as any).orgId = 'test-org';
      (el as any).client = {
        GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
        POST: mockPost,
      };
      document.body.appendChild(el);
      await settle(el);

      expect(codeField(el).readonly).toBe(true);
      expect(codeField(el).editButton).toBe(true);

      inputName(el, item.nameSelector, 'Billing Support');
      await settle(el);

      expect((el as any)._formData.code).toBe('billing_support');
      expect(codeField(el).readonly).toBe(true);

      codeField(el).dispatchEvent(new CustomEvent('or-code-edit', { bubbles: true, composed: true }));
      await settle(el);

      expect(codeField(el).readonly).toBe(false);
      expect(codeField(el).editButton).toBe(false);
      expect(codeField(el).saveButton).toBe(true);
      expect(codeField(el).cancelButton).toBe(true);

      codeField(el).dispatchEvent(new CustomEvent('or-code-input', {
        detail: { value: 'draft_code' },
        bubbles: true,
        composed: true,
      }));
      await settle(el);

      expect((el as any)._formData.code).toBe('billing_support');

      if (typeof (el as any)._handleNext === 'function') {
        await (el as any)._handleNext();
      } else {
        await (el as any)._handleSubmit();
      }
      await settle(el);

      expect((el as any)._errors.code).toBe('Save or cancel code before continuing.');
      expect(mockPost).not.toHaveBeenCalled();

      codeField(el).dispatchEvent(new CustomEvent('or-code-cancel', { bubbles: true, composed: true }));
      await settle(el);

      expect(codeField(el).readonly).toBe(true);
      expect((el as any)._formData.code).toBe('billing_support');

      inputName(el, item.nameSelector, 'Priority Support');
      await settle(el);

      expect((el as any)._formData.code).toBe('priority_support');

      codeField(el).dispatchEvent(new CustomEvent('or-code-edit', { bubbles: true, composed: true }));
      await settle(el);

      codeField(el).dispatchEvent(new CustomEvent('or-code-input', {
        detail: { value: 'manual_code' },
        bubbles: true,
        composed: true,
      }));
      await settle(el);

      codeField(el).dispatchEvent(new CustomEvent('or-code-save', { bubbles: true, composed: true }));
      await settle(el);

      expect(codeField(el).readonly).toBe(true);
      expect(codeField(el).editButton).toBe(true);
      expect((el as any)._formData.code).toBe('manual_code');

      inputName(el, item.nameSelector, 'Escalation Desk');
      await settle(el);

      expect((el as any)._formData.code).toBe('manual_code');
    });
  }
});
