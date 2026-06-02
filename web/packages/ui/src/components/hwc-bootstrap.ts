// hwc-core must load first — it sets window.Lit (+ LitDecorators, UIkit).
// hwc-icon sets window.Lucide, hwc-chart sets window.ApexCharts.
// hwc-components.iife.js is the only bundle that references those globals
// as IIFE arguments: `})({}，Lit，Lucide，ApexCharts)`. Without the three
// preceding side-effect imports the runtime throws ReferenceError: Lit is
// not defined before a single custom element can register.
import 'frankenstyle/js/hwc-core.iife';
import 'frankenstyle/js/hwc-icon.iife';
import 'frankenstyle/js/hwc-chart.iife';
import 'frankenstyle/js/hwc-components.iife';
