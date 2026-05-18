const { mkdirSync, symlinkSync, existsSync } = require('fs');
const { resolve } = require('path');
const dest = 'public/shoelace/assets/icons';
const src = resolve('node_modules/@shoelace-style/shoelace/dist/assets/icons');
mkdirSync('public/shoelace/assets', { recursive: true });
if (!existsSync(dest)) symlinkSync(src, dest, 'dir');
