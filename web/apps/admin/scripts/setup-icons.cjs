const { mkdirSync, symlinkSync, lstatSync } = require('fs');
const { resolve } = require('path');
const dest = 'public/shoelace/assets/icons';
const src = resolve('node_modules/@shoelace-style/shoelace/dist/assets/icons');
mkdirSync('public/shoelace/assets', { recursive: true });
let exists = false;
try { lstatSync(dest); exists = true; } catch { /* not present */ }
if (!exists) symlinkSync(src, dest, 'dir');
