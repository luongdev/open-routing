import 'urlpattern-polyfill';

if (typeof Element !== 'undefined') {
  if (!Element.prototype.getAnimations) {
    Element.prototype.getAnimations = function () { return []; };
  }
  if (!Element.prototype.animate) {
    Element.prototype.animate = function () { return {/* stub Animation */} as unknown as Animation; };
  }
}
