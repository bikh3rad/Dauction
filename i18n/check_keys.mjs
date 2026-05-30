#!/usr/bin/env node
// CI key-parity check (wired into `make check`): asserts all four locale
// catalogs have an IDENTICAL set of leaf keys — no missing translations,
// no stray keys. Nested objects (e.g. "err") are compared by flattened path.
// Exit 0 = parity holds; exit 1 = mismatch (prints the offending keys).
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const LANGS = ['en', 'fa', 'ar', 'tr'];

function flatten(obj, prefix = '') {
  const out = [];
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) out.push(...flatten(v, path));
    else out.push(path);
  }
  return out;
}

const keysets = {};
for (const lang of LANGS) {
  const raw = readFileSync(join(here, `${lang}.json`), 'utf8');
  keysets[lang] = new Set(flatten(JSON.parse(raw)));
}

const reference = keysets.en;
let failed = false;
for (const lang of LANGS) {
  if (lang === 'en') continue;
  const missing = [...reference].filter((k) => !keysets[lang].has(k));
  const extra = [...keysets[lang]].filter((k) => !reference.has(k));
  if (missing.length || extra.length) {
    failed = true;
    console.error(`✗ ${lang}.json differs from en.json:`);
    if (missing.length) console.error(`  missing (${missing.length}): ${missing.join(', ')}`);
    if (extra.length) console.error(`  extra   (${extra.length}): ${extra.join(', ')}`);
  }
}

if (failed) {
  console.error('\ni18n key parity FAILED.');
  process.exit(1);
}
console.log(`✓ i18n key parity OK — ${reference.size} keys identical across ${LANGS.join(', ')}.`);
