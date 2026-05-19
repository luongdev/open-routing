// Phase 6 Plan 09 Task 2: <or-break-reason-form> — Single-step form for BreakReason create.
// D6-17: single-step for BreakReason (simple entity).
// D04_1-02: code field uses or-code-input (required, writable on create).
// ADMIN-04: only this.client.POST — never direct fetch().
// ajv validateCreateBreakReason called on submit.
// UI-SPEC §5.5 D6-V-16: Fields: code, name, external_id, routable (default true), display_order (required), enabled.
// W0.1-18: Ember dashboard layout — uk-* classes, adoptShadowSheets, zero sl-*.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import type { ApiClient } from '../../api/client.js';
import validateCreateBreakReason from '../../validators/CreateBreakReasonRequest.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

// Primitives
import { nameToCode } from '../primitives/code-input.js';
import '../primitives/code-input.js';

interface FormData {
  code: string;
  name: string;
  external_id: string;
  routable: boolean;
  display_order: number | '';
  enabled: boolean;
}

/**
 * <or-break-reason-form> — Ember-style single-step form for creating a new BreakReason.
 *
 * Fields: code, name, external_id, routable (default true), display_order (required), enabled.
 * On 201 → dispatches open-routing:navigate to /orgs/{orgId}/break-reasons/{newId}
 * On 409 duplicate_code → inline code error
 *
 * Properties:
 *   - orgId: (attribute 'org-id') — the current org UUID
 *   - client: ApiClient — passed from shell at boot
 */
@customElement('or-break-reason-form')
export class OrBreakReasonForm extends LitElement {
  static override styles = css`
    :host {
      display: block;
      padding: 24px;
      max-width: 680px;
      background: var(--background);
      min-height: 100%;
    }

    /* ── Page header ───────────────────────────────────── */
    .page-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 24px;
    }

    .page-title {
      font-size: 24px;
      font-weight: 700;
      margin: 0;
      color: var(--foreground);
    }

    .back-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: none;
      border: none;
      cursor: pointer;
      font-size: 14px;
      color: var(--muted-foreground);
      padding: 4px 8px;
      border-radius: 6px;
      transition: color .12s, background .12s;
    }

    .back-btn:hover {
      color: var(--foreground);
      background: var(--muted);
    }

    /* ── Form card ─────────────────────────────────────── */
    .form-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: var(--shadow-sm);
      padding: 32px;
    }

    .step-helper {
      font-size: 13px;
      color: var(--muted-foreground);
      margin: 0 0 24px;
    }

    /* ── Form rows ─────────────────────────────────────── */
    .form-row {
      margin-bottom: 16px;
    }

    .form-label {
      display: block;
      font-size: 13px;
      font-weight: 600;
      color: var(--foreground);
      margin-bottom: 6px;
    }

    .form-label-required::after {
      content: ' *';
      color: var(--destructive);
    }

    .field-error {
      font-size: 12px;
      color: var(--destructive);
      margin-top: 4px;
    }

    .field-help {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 4px;
    }

    /* ── Toggle switch ─────────────────────────────────── */
    .switch-wrap {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      cursor: pointer;
      font-size: 14px;
      color: var(--foreground);
      user-select: none;
    }

    .switch-wrap input[type="checkbox"] {
      position: absolute;
      opacity: 0;
      width: 0;
      height: 0;
    }

    .switch-track {
      position: relative;
      width: 36px;
      height: 20px;
      border-radius: 9999px;
      background: var(--border);
      transition: background .15s;
      flex-shrink: 0;
    }

    .switch-wrap:has(input[type="checkbox"]:checked) .switch-track {
      background: var(--primary);
    }

    .switch-thumb {
      position: absolute;
      top: 2px;
      left: 2px;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      background: white;
      box-shadow: 0 1px 3px rgba(0,0,0,.2);
      transition: transform .15s;
    }

    .switch-wrap:has(input[type="checkbox"]:checked) .switch-thumb {
      transform: translateX(16px);
    }

    /* ── Routable toggle block ─────────────────────────── */
    .toggle-block {
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 12px 14px;
      background: var(--muted);
    }

    .toggle-block-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .toggle-block-label {
      font-size: 14px;
      font-weight: 500;
      color: var(--foreground);
    }

    .toggle-block-sub {
      font-size: 12px;
      color: var(--muted-foreground);
      margin-top: 4px;
    }

    /* ── API error ─────────────────────────────────────── */
    .api-error {
      background: color-mix(in oklch, var(--destructive) 10%, transparent);
      border: 1px solid color-mix(in oklch, var(--destructive) 30%, transparent);
      color: var(--destructive);
      border-radius: 8px;
      padding: 12px 16px;
      font-size: 13px;
      margin-bottom: 16px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    /* ── Form actions ──────────────────────────────────── */
    .form-actions {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
      margin-top: 24px;
      padding-top: 20px;
      border-top: 1px solid var(--border);
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  // --- Properties ---
  @property({ type: String, attribute: 'org-id' }) accessor orgId = '';
  @property({ type: Object }) accessor client!: ApiClient;

  // --- Internal state ---
  @state() accessor _formData: FormData = {
    code: '',
    name: '',
    external_id: '',
    routable: true,
    display_order: '',
    enabled: true,
  };
  @state() accessor _errors: Record<string, string> = {};
  @state() private accessor _apiError: string | null = null;
  @state() private accessor _submitting = false;
  private _codeAutoFill = true;

  // --- Navigation ---

  private _navigate(path: string): void {
    this.dispatchEvent(
      new CustomEvent('open-routing:navigate', {
        detail: { path },
        bubbles: true,
        composed: true,
      })
    );
  }

  async _handleSubmit(): Promise<void> {
    if (this._submitting) return;

    const displayOrderVal = this._formData.display_order;
    if (displayOrderVal === '' || displayOrderVal === null || displayOrderVal === undefined) {
      this._errors = { display_order: 'Display order is required.' };
      return;
    }

    const body = {
      code: this._formData.code,
      name: this._formData.name,
      external_id: this._formData.external_id || null,
      routable: this._formData.routable,
      display_order: Number(this._formData.display_order),
      enabled: this._formData.enabled,
    };

    // ajv standalone validators attach .errors dynamically; cast to access it.
    const validateFn = validateCreateBreakReason as unknown as {
      (data: unknown): boolean;
      errors: Array<{ instancePath: string; message?: string }> | null;
    };
    if (!validateFn(body)) {
      const errors: Record<string, string> = {};
      for (const err of validateFn.errors ?? []) {
        const field = err.instancePath.replace(/^\//, '') || 'form';
        errors[field] = err.message ?? 'Invalid value';
      }
      this._errors = errors;
      return;
    }
    this._errors = {};
    this._apiError = null;
    this._submitting = true;

    try {
      const result = await this.client.POST('/v1/orgs/{org_id}/break-reasons' as never, {
        params: { path: { org_id: this.orgId } },
        body,
      } as never);

      const { data, error } = result as { data: { id: string; code: string } | null; error: unknown };

      if (error) {
        if (
          error &&
          typeof error === 'object' &&
          'error' in error &&
          (error as { error: string }).error === 'duplicate_code'
        ) {
          this._errors = { code: 'This code is already in use.' };
          return;
        }
        this._apiError = (error as { reason?: string })?.reason ?? 'Create failed';
        return;
      }

      if (data?.id) {
        this.dispatchEvent(
          new CustomEvent('open-routing:entity-created', {
            detail: { entityId: data.id, entityType: 'break-reason' },
            bubbles: true,
            composed: true,
          })
        );
        this._navigate(`/orgs/${this.orgId}/break-reasons/${data.id}`);
      }
    } finally {
      this._submitting = false;
    }
  }

  // --- Render ---

  override render() {
    return html`
      <div class="page-header">
        <button
          type="button"
          class="back-btn"
          @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons`)}
        >
          <uk-icon icon="chevron-left" height="16" width="16"></uk-icon>
          Break reasons
        </button>
        <h1 class="page-title">Create break reason</h1>
      </div>

      <div class="form-card">
        <p class="step-helper">Define the break reason. Code cannot be changed after create.</p>

        <div class="form-row">
          <or-code-input
            .value=${this._formData.code}
            .required=${true}
            @or-code-input=${(e: CustomEvent) => {
              this._codeAutoFill = false;
              this._formData = { ...this._formData, code: e.detail.value };
              if (this._errors['code']) {
                this._errors = { ...this._errors, code: '' };
              }
            }}
          ></or-code-input>
          ${when(
            this._errors['code'],
            () => html`<div class="field-error">${this._errors['code']}</div>`
          )}
        </div>

        <div class="form-row">
          <label class="form-label form-label-required" for="br-name">Name</label>
          <input
            id="br-name"
            class="uk-input"
            type="text"
            required
            .value=${this._formData.name}
            placeholder="e.g. Lunch Break"
            @input=${(e: Event) => {
              const name = (e.target as HTMLInputElement).value;
              this._formData = {
                ...this._formData,
                name,
                ...(this._codeAutoFill ? { code: nameToCode(name) } : {}),
              };
            }}
          />
          ${when(
            this._errors['name'],
            () => html`<div class="field-error">${this._errors['name']}</div>`
          )}
        </div>

        <div class="form-row">
          <label class="form-label" for="br-ext-id">External ID</label>
          <input
            id="br-ext-id"
            class="uk-input"
            type="text"
            .value=${this._formData.external_id}
            placeholder="Optional reference from your system"
            @input=${(e: Event) => {
              this._formData = {
                ...this._formData,
                external_id: (e.target as HTMLInputElement).value,
              };
            }}
          />
          <div class="field-help">Match an ID from your CRM or identity provider.</div>
        </div>

        <div class="form-row">
          <div class="toggle-block">
            <div class="toggle-block-header">
              <div>
                <div class="toggle-block-label">Routable</div>
                <div class="toggle-block-sub">When on, agents on this break can still receive routed interactions.</div>
              </div>
              <label class="switch-wrap" aria-label="Routable">
                <input
                  type="checkbox"
                  .checked=${this._formData.routable}
                  @change=${(e: Event) => {
                    this._formData = {
                      ...this._formData,
                      routable: (e.target as HTMLInputElement).checked,
                    };
                  }}
                />
                <span class="switch-track"><span class="switch-thumb"></span></span>
              </label>
            </div>
          </div>
        </div>

        <div class="form-row">
          <label class="form-label form-label-required" for="br-display-order">Display Order</label>
          <input
            id="br-display-order"
            class="uk-input"
            type="number"
            min="0"
            step="1"
            .value=${this._formData.display_order === '' ? '' : String(this._formData.display_order)}
            required
            placeholder="e.g. 1"
            @input=${(e: Event) => {
              const val = (e.target as HTMLInputElement).value;
              this._formData = {
                ...this._formData,
                display_order: val === '' ? '' : parseInt(val, 10),
              };
            }}
          />
          <div class="field-help">Lower values appear first in the break picker.</div>
          ${when(
            this._errors['display_order'],
            () => html`<div class="field-error">${this._errors['display_order']}</div>`
          )}
        </div>

        <div class="form-row">
          <label class="switch-wrap">
            <input
              type="checkbox"
              .checked=${this._formData.enabled}
              @change=${(e: Event) => {
                this._formData = {
                  ...this._formData,
                  enabled: (e.target as HTMLInputElement).checked,
                };
              }}
            />
            <span class="switch-track"><span class="switch-thumb"></span></span>
            <span>Enabled</span>
          </label>
          <div class="field-help">Disabled break reasons are hidden from the break picker.</div>
        </div>

        ${this._apiError
          ? html`
              <div class="api-error">
                <uk-icon icon="alert-triangle" height="16" width="16"></uk-icon>
                ${this._apiError}
              </div>
            `
          : nothing}

        <div class="form-actions">
          <button
            type="button"
            class="uk-button uk-button-default"
            @click=${() => this._navigate(`/orgs/${this.orgId}/break-reasons`)}
          >Cancel</button>
          <button
            type="button"
            class="uk-button uk-button-primary"
            ?disabled=${this._submitting}
            @click=${this._handleSubmit}
          >
            ${this._submitting ? 'Creating…' : 'Create break reason'}
          </button>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-break-reason-form': OrBreakReasonForm;
  }
}
