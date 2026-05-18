// Phase 6 Plan 04: <or-form-wizard> — multi-step wizard for entity create flows.
// Multi-step: Agent (3 steps), Channel (3 steps) — renders stepper with step circles.
// Single-step: Skill, Queue, BreakReason, Adapter — stepper hidden, simple layout.
// Slot-based step content: <slot name="step-{key}"> per step.
// D6-17: same component, right-sized per entity complexity.
// Per-component Shoelace imports (D6-08).

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { when } from 'lit/directives/when.js';

// Shoelace per-component imports (D6-08)
import '@shoelace-style/shoelace/dist/components/button/button.js';
import '@shoelace-style/shoelace/dist/components/icon/icon.js';

/** A single step definition for or-form-wizard. */
export interface OrFormWizardStep {
  key: string;
  label: string;
}

/**
 * <or-form-wizard> — configurable step-based form component.
 *
 * Multi-step (Agent, Channel): renders a stepper with step circles and connectors.
 * Single-step (Skill, Queue, BreakReason, Adapter): hides the stepper, shows
 * a simple form layout with Create button directly.
 *
 * Step state:
 *   - completed (index < currentStep): filled primary background + check icon
 *   - current (index === currentStep): primary border + step number
 *   - future (index > currentStep): muted border + step number
 *
 * Events:
 *   - 'open-routing:wizard-step-changed'  CustomEvent<{ step: number; key: string }>
 *   - 'open-routing:wizard-completed'     CustomEvent<void>
 *
 * Usage:
 *   <or-form-wizard
 *     .steps=${[{ key: 'basics', label: 'Basics' }, { key: 'skills', label: 'Skills' }, { key: 'review', label: 'Review' }]}
 *     .currentStep=${0}
 *   >
 *     <div slot="step-basics">…form fields…</div>
 *     <div slot="step-skills">…skills picker…</div>
 *     <div slot="step-review">…summary…</div>
 *   </or-form-wizard>
 */
@customElement('or-form-wizard')
export class OrFormWizard extends LitElement {
  static override styles = css`
    :host {
      display: block;
    }

    /* Stepper (hidden for single-step entities) */
    .stepper {
      display: flex;
      align-items: center;
      margin-bottom: 32px;
    }

    .step-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      position: relative;
      flex: 1;
    }

    .step-item:first-child {
      flex: 0;
    }

    .step-item:last-child {
      flex: 0;
    }

    /* Connector line between step circles */
    .step-connector {
      flex: 1;
      height: 2px;
      background: var(--or-color-divider, #e5e5e5);
      margin: 0;
      align-self: flex-start;
      margin-top: 11px; /* align with center of 24px circles */
    }

    .step-connector.completed {
      background: var(--sl-color-primary-500, #2b8a93);
    }

    /* Step circle: 24px diameter */
    .step-circle {
      width: 24px;
      height: 24px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 12px;
      font-weight: 600;
      flex-shrink: 0;
      transition: background 0.2s ease, border-color 0.2s ease;
    }

    .step-circle.completed {
      background: var(--sl-color-primary-500, #2b8a93);
      border: 2px solid var(--sl-color-primary-500, #2b8a93);
      color: #ffffff;
    }

    .step-circle.current {
      background: transparent;
      border: 2px solid var(--sl-color-primary-500, #2b8a93);
      color: var(--sl-color-primary-500, #2b8a93);
    }

    .step-circle.future {
      background: transparent;
      border: 2px solid var(--or-color-divider, #e5e5e5);
      color: var(--or-color-text-muted, #737373);
    }

    .step-label {
      margin-top: 6px;
      font-size: 12px;
      text-align: center;
      white-space: nowrap;
    }

    .step-label.completed {
      color: var(--sl-color-primary-500, #2b8a93);
      font-weight: 500;
    }

    .step-label.current {
      color: var(--sl-color-primary-500, #2b8a93);
      font-weight: 600;
    }

    .step-label.future {
      color: var(--or-color-text-muted, #737373);
    }

    /* Step content area */
    .step-content {
      min-height: 200px;
    }

    /* Navigation buttons */
    .nav-buttons {
      display: flex;
      gap: 8px;
      margin-top: 24px;
      justify-content: flex-end;
    }
  `;

  /** Steps definition. Array length determines single-step vs multi-step mode. */
  @property({ type: Array }) accessor steps: OrFormWizardStep[] = [];

  /** Currently active step index (0-based). Parent may sync this via property. */
  @property({ type: Number }) accessor currentStep = 0;

  /**
   * When true, the wizard's built-in navigation buttons (Back/Next/Create) are hidden.
   * Use this when the parent component provides its own navigation (e.g. or-agent-form
   * which needs step-level validation before advancing). The stepper and slot content
   * still render; only the nav bar is suppressed.
   */
  @property({ type: Boolean, attribute: 'hide-nav' }) accessor hideNav = false;

  /** Internal dirty flag — user has entered data and may lose it on cancel. */
  @state() private accessor _dirty = false;

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
      // Final step: dispatch wizard-completed
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

      // Add step circle + label
      items.push(html`
        <div class="step-item">
          <div class="step-circle ${state}">
            ${isCompleted
              ? html`<sl-icon name="check" style="font-size:12px"></sl-icon>`
              : String(i + 1)
            }
          </div>
          <span class="step-label ${state}">${step.label}</span>
        </div>
      `);

      // Add connector line between steps (not after last)
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
        ${when(
          this.steps.length === 0,
          () => html`<slot></slot>`
        )}
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
            <sl-button variant="default" size="small" @click=${this._handleBack}>
              Back
            </sl-button>
          `
        )}
        <sl-button
          variant="primary"
          size="small"
          @click=${this._handleNext}
        >
          ${this._isLastStep || this._isSingleStep ? finalLabel : 'Next'}
        </sl-button>
      </div>
    `;
  }

  override render() {
    return html`
      ${this._renderStepper()}
      ${this._renderStepContent()}
      ${this.hideNav ? null : this._renderNavButtons()}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'or-form-wizard': OrFormWizard;
  }
}
