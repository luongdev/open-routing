// Phase 6 Plan 04: <or-form-wizard> — multi-step wizard for entity create flows.
// Wave 0.1 polish: pure Lit + Ember tokens, no shoelace. Coral active/completed
// step indicators, lucide icons, smooth connector transitions.

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';
import { adoptShadowSheets } from '../../styles/shadow-sheets.js';

export interface OrFormWizardStep {
  key: string;
  label: string;
}

@customElement('or-form-wizard')
export class OrFormWizard extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    /* ── Stepper ─────────────────────────────────────────────────── */
    .stepper {
      display: flex;
      align-items: flex-start;
      margin-bottom: 24px;
    }

    .step-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      position: relative;
      flex: 0 0 auto;
    }

    .step-connector {
      flex: 1;
      height: 2px;
      background: var(--border);
      margin: 0 4px;
      align-self: flex-start;
      margin-top: 13px; /* center on 28px circles */
      border-radius: 1px;
      transition: background .25s ease;
    }

    .step-connector.completed {
      background: var(--primary);
    }

    /* Step circle: 28px diameter, more breathing room */
    .step-circle {
      width: 28px;
      height: 28px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 13px;
      font-weight: 600;
      flex-shrink: 0;
      transition: background .2s, border-color .2s, color .2s, box-shadow .2s;
    }

    .step-circle.completed {
      background: var(--primary);
      border: 2px solid var(--primary);
      color: var(--primary-foreground);
      box-shadow: 0 2px 4px -1px oklch(0.62 0.22 28 / 0.30);
    }

    .step-circle.current {
      background: var(--card);
      border: 2px solid var(--primary);
      color: var(--primary);
      box-shadow: 0 0 0 4px color-mix(in oklch, var(--primary) 18%, transparent);
    }

    .step-circle.future {
      background: var(--card);
      border: 2px solid var(--border);
      color: var(--muted-foreground);
    }

    .step-circle uk-icon {
      color: inherit;
    }

    .step-label {
      margin-top: 8px;
      font-size: 12px;
      text-align: center;
      white-space: nowrap;
      max-width: 120px;
    }

    .step-label.completed {
      color: var(--foreground);
      font-weight: 500;
    }

    .step-label.current {
      color: var(--primary);
      font-weight: 600;
    }

    .step-label.future {
      color: var(--muted-foreground);
    }

    /* Step content area */
    .step-content {
      min-height: 200px;
    }

    /* Navigation buttons */
    .nav-buttons {
      display: flex;
      gap: 8px;
      margin-top: 20px;
      justify-content: flex-end;
    }
  `;

  override createRenderRoot() {
    const root = super.createRenderRoot() as ShadowRoot;
    adoptShadowSheets(root);
    return root;
  }

  @property({ type: Array }) steps: OrFormWizardStep[] = [];
  @property({ type: Number }) currentStep = 0;
  @property({ type: Boolean, attribute: 'hide-nav' }) hideNav = false;

  @state() private _dirty = false;

  private get _isSingleStep(): boolean {
    return this.steps.length <= 1;
  }

  private get _isFirstStep(): boolean {
    return this.currentStep === 0;
  }

  private get _isLastStep(): boolean {
    return this.currentStep >= this.steps.length - 1;
  }

  private _getStepState(index: number): 'completed' | 'current' | 'future' {
    if (index < this.currentStep) return 'completed';
    if (index === this.currentStep) return 'current';
    return 'future';
  }

  private _handleBack(): void {
    if (this._isFirstStep) return;
    const newStep = this.currentStep - 1;
    this.currentStep = newStep;
    this.dispatchEvent(
      new CustomEvent('open-routing:wizard-step-changed', {
        detail: { step: newStep, key: this.steps[newStep]?.key ?? '' },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleNext(): void {
    if (this._isLastStep || this._isSingleStep) {
      this.dispatchEvent(
        new CustomEvent('open-routing:wizard-completed', {
          detail: {},
          bubbles: true,
          composed: true,
        })
      );
      return;
    }
    const newStep = this.currentStep + 1;
    this.currentStep = newStep;
    this.dispatchEvent(
      new CustomEvent('open-routing:wizard-step-changed', {
        detail: { step: newStep, key: this.steps[newStep]?.key ?? '' },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _renderStepper() {
    if (this._isSingleStep) return null;

    const items: ReturnType<typeof html>[] = [];

    this.steps.forEach((step, i) => {
      const state = this._getStepState(i);
      const isCompleted = state === 'completed';

      items.push(html`
        <div class="step-item">
          <div class="step-circle ${state}">
            ${isCompleted
              ? html`<uk-icon icon="check" height="14" width="14"></uk-icon>`
              : String(i + 1)}
          </div>
          <span class="step-label ${state}">${step.label}</span>
        </div>
      `);

      if (i < this.steps.length - 1) {
        items.push(html`
          <div class="step-connector ${isCompleted ? 'completed' : ''}"></div>
        `);
      }
    });

    return html`<div class="stepper">${items}</div>`;
  }

  private _renderStepContent() {
    return html`
      <div class="step-content">
        ${this.steps.map((step, i) => html`
          <div style="display: ${i === this.currentStep ? 'block' : 'none'}">
            <slot name="step-${step.key}"></slot>
          </div>
        `)}
        ${when(this.steps.length === 0, () => html`<slot></slot>`)}
      </div>
    `;
  }

  private _renderNavButtons() {
    const finalLabel = 'Create';
    return html`
      <div class="nav-buttons">
        ${when(
          !this._isFirstStep && !this._isSingleStep,
          () => html`
            <button class="uk-button uk-button-default uk-button-small" @click=${this._handleBack}>
              Back
            </button>
          `
        )}
        <button class="uk-button uk-button-primary uk-button-small" @click=${this._handleNext}>
          ${this._isLastStep || this._isSingleStep ? finalLabel : 'Next'}
        </button>
      </div>
    `;
  }

  override render() {
    return html`
      ${this._renderStepper()}
      ${this._renderStepContent()}
      ${this.hideNav ? nothing : this._renderNavButtons()}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-form-wizard': OrFormWizard;
  }
}
