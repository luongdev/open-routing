import { html, TemplateResult } from 'lit';
import '../../primitives/or-select.js';

export const selectSlot: TemplateResult = html`
  <div style="display:flex;flex-direction:column;gap:16px;">
    <or-select
      label="Default"
      .options=${[{ value: 'a', label: 'Option A' }, { value: 'b', label: 'Option B' }]}
    ></or-select>

    <or-select
      label="With helper text"
      helper-text="Choose the best option"
      .options=${[{ value: 'x', label: 'Choice X' }, { value: 'y', label: 'Choice Y' }]}
    ></or-select>

    <or-select
      label="With error"
      error-text="Selection required"
      .options=${[{ value: '1', label: 'Item 1' }]}
    ></or-select>

    <or-select
      label="Disabled"
      disabled
      .options=${[{ value: 'n', label: 'None' }]}
    ></or-select>

    <or-select
      label="Small"
      size="sm"
      .options=${[{ value: 's', label: 'Small item' }]}
    ></or-select>

    <or-select
      label="Large"
      size="lg"
      placeholder="Select an option..."
      .options=${[{ value: 'l', label: 'Large item' }]}
    ></or-select>
  </div>
`;
