import { html } from 'lit';
import '../../primitives/or-input.js';

export const inputSlot = html`
  <div style="display:flex; flex-direction:column; gap:16px;">
    <or-input placeholder="Default input"></or-input>
    <or-input label="With label" placeholder="Enter value"></or-input>
    <or-input label="With helper" placeholder="Enter value" helper-text="This is helper text"></or-input>
    <or-input label="With error" placeholder="Enter value" error-text="This field is required"></or-input>
    <or-input label="Disabled" placeholder="Disabled" .disabled=${true}></or-input>
    <or-input label="Readonly" value="readonly value" .readonly=${true}></or-input>
    <or-input label="With prefix icon" placeholder="Search...">
      <span slot="prefix" aria-hidden="true">&#x1F50D;</span>
    </or-input>
    <or-input label="With suffix icon" placeholder="Enter value">
      <span slot="suffix" aria-hidden="true">&#x2713;</span>
    </or-input>
    <div style="display:flex; gap:12px; align-items:flex-start;">
      <or-input size="sm" placeholder="Small"></or-input>
      <or-input size="md" placeholder="Medium"></or-input>
      <or-input size="lg" placeholder="Large"></or-input>
    </div>
  </div>
`;
