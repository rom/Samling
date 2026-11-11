#!/usr/bin/env node
// Verifies the translations: sv.js and en.js must have identical keys, and
// every key used in the client source must exist. Run: node scripts/check-i18n.mjs
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const js = join(dirname(fileURLToPath(import.meta.url)), '..', 'web', 'static', 'js');
const en = (await import(pathToFileURL(join(js, 'i18n', 'en.js')))).default;
const sv = (await import(pathToFileURL(join(js, 'i18n', 'sv.js')))).default;
let bad = 0;
const fail = (msg) => { console.error(msg); bad++; };

for (const k of Object.keys(en)) if (!(k in sv)) fail(`missing in sv.js: ${k}`);
for (const k of Object.keys(sv)) if (!(k in en)) fail(`missing in en.js: ${k}`);
for (const k of Object.keys(en)) {
  const vars = (s) => (s.match(/\{\w+\}/g) || []).sort().join();
  if (k in sv && vars(en[k]) !== vars(sv[k])) fail(`placeholders differ: ${k}`);
}
const files = [js, join(js, 'views')].flatMap((d) => readdirSync(d).filter((f) => f.endsWith('.js')).map((f) => join(d, f)));
const prefixes = new Set();
for (const f of files) {
  const src = readFileSync(f, 'utf8');
  for (const m of src.matchAll(/\bt\('([a-z0-9_.]+)'\s*([,)+])/g)) {
    if (m[2] === '+') prefixes.add(m[1]);
    else if (!(m[1] in en)) fail(`${f}: unknown key ${m[1]}`);
  }
}
for (const p of prefixes) if (!Object.keys(en).some((k) => k.startsWith(p))) fail(`no keys with prefix ${p}`);
if (bad) { console.error(`${bad} problem(s)`); process.exit(1); }
console.log(`i18n OK: ${Object.keys(en).length} keys in 2 languages`);
