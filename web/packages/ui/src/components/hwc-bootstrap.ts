// Must be imported once at the app entry point — later imports of the same
// IIFE are no-ops, but missing this import leaves uk-* custom elements
// undefined (silent rendering failure, not an error).
import 'frankenstyle/js/hwc-components.iife';
import 'frankenstyle/js/hwc-icon.iife';
