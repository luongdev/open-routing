// urlpattern-polyfill is needed in Node/happy-dom test environments for @lit-labs/router
// route parsing. All major browsers support URLPattern natively as of 2025 (Baseline 2025).
// Do NOT import this in any component source file — only in test setup.
import 'urlpattern-polyfill';

// Shoelace uses the Web Animations API (Element.getAnimations, Element.animate)
// for dialog/overlay transitions. happy-dom does not implement these.
// Polyfill with no-ops to prevent unhandled rejections in component tests that
// render sl-dialog, sl-alert (animated), and similar Shoelace components.
if (typeof Element !== 'undefined') {
  if (!Element.prototype.getAnimations) {
    Element.prototype.getAnimations = function () { return []; };
  }
  if (!Element.prototype.animate) {
    // Return a minimal Animation-like object with a finished promise
    Element.prototype.animate = function () {
      return {
        finished: Promise.resolve(),
        cancel: () => {},
        finish: () => {},
        play: () => {},
        pause: () => {},
        reverse: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
      } as unknown as Animation;
    };
  }
}
