#!/usr/bin/env node
// Render one share page: shares/template.html + shares/data/<code>.json
//   -> frontend/public/p/<code>/index.html  (+ photos, + qr svg)
//
// frontend/public/ is copied verbatim into dist/ by Vite, and the existing
// SiteDeployment ships dist/ to grow-site -- so publishing a share page is
// just the normal site deploy. No bucket, no API, no table.
//
//   node shares/build.mjs A7K2M9
//   node shares/build.mjs A7K2M9 --no-qr

import { readFile, writeFile, mkdir, copyFile, readdir, stat } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join, extname } from 'node:path';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = join(HERE, '..');
const HOST = 'grow.chrisdargis.com';

// Phase colours are fixed here, not in the data, so a JSON can never
// restyle the page. Anything unrecognised falls back to sage.
const PHASE_COLORS = {
  germination: 'rgba(139,154,124,.45)',
  seedling:    '#8B9A7C',
  vegetative:  '#6E7C5E',
  flowering:   '#A6802B',
};

const fail = (msg) => { console.error(`\n  error: ${msg}\n`); process.exit(1); };

// ── tiny mustache-ish renderer ─────────────────────────────────────────
// {{x}} escaped · {{{x}}} raw · {{#list}}..{{/list}} · {{^flag}}..{{/flag}}
const esc = (s) => String(s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;');

const get = (ctx, path) =>
  path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), ctx);

function render(tpl, ctx) {
  // sections: repeat for each array item, with the item merged into scope
  tpl = tpl.replace(/\{\{#([\w.]+)\}\}([\s\S]*?)\{\{\/\1\}\}/g, (_, key, inner) => {
    const val = get(ctx, key);
    if (!Array.isArray(val)) return val ? render(inner, ctx) : '';
    return val.map((item, i) => render(inner, {
      ...ctx,
      ...(item && typeof item === 'object' ? item : { '.': item }),
      first: i === 0,
      last: i === val.length - 1,
    })).join('');
  });
  // inverted sections
  tpl = tpl.replace(/\{\{\^([\w.]+)\}\}([\s\S]*?)\{\{\/\1\}\}/g, (_, key, inner) =>
    get(ctx, key) ? '' : render(inner, ctx));
  // raw, then escaped
  tpl = tpl.replace(/\{\{\{([\w.]+)\}\}\}/g, (_, k) => String(get(ctx, k) ?? ''));
  tpl = tpl.replace(/\{\{([\w.]+)\}\}/g, (_, k) => {
    const v = get(ctx, k);
    return v == null ? '' : esc(v);
  });
  return tpl;
}

// ── validation ─────────────────────────────────────────────────────────
// A wrong number here ends up on a printed label, so fail loudly rather
// than rendering a page with a hole in it.
function validate(d) {
  const missing = ['code', 'strain', 'batch', 'harvested', 'totalDays', 'phases',
                   'photos', 'record', 'npk', 'npkSource', 'ingredients']
    .filter((k) => d[k] == null);
  if (missing.length) fail(`data is missing: ${missing.join(', ')}`);

  if (d.photos.length !== 3) fail(`expected 3 photos, got ${d.photos.length}`);
  for (const p of d.photos) {
    if (!p.file || !p.stage || p.day == null) {
      fail(`each photo needs file, stage and day — got ${JSON.stringify(p)}`);
    }
  }
  for (const k of ['n', 'p', 'k']) {
    if (typeof d.npk[k] !== 'number') fail(`npk.${k} must be a number`);
    if (!Number.isInteger(d.npk[k])) {
      fail(`npk.${k} is ${d.npk[k]} — publish whole grams; pot size is the ` +
           `container, not the media volume, so decimals claim accuracy that isn't there`);
    }
  }
  const sum = d.phases.reduce((t, p) => t + p.days, 0);
  if (sum !== d.totalDays) {
    fail(`phases sum to ${sum} days but totalDays is ${d.totalDays}`);
  }
  const days = d.photos.map((p) => p.day);
  if (days.some((v, i) => i && v <= days[i - 1])) {
    fail(`photo day numbers must increase: ${days.join(', ')}`);
  }
  if (days.at(-1) > d.totalDays) {
    fail(`photo day ${days.at(-1)} is past harvest at day ${d.totalDays}`);
  }
}

// ── main ───────────────────────────────────────────────────────────────
const code = process.argv[2];
const wantQr = !process.argv.includes('--no-qr');
if (!code) fail('usage: node shares/build.mjs <code> [--no-qr]');
if (!/^[0-9A-HJ-NP-TV-Z]{4,10}$/.test(code)) {
  fail(`"${code}" is not a Crockford base32 code (uppercase, no I L O U)`);
}

const dataPath = join(HERE, 'data', `${code}.json`);
if (!existsSync(dataPath)) fail(`no data file at shares/data/${code}.json`);
const data = JSON.parse(await readFile(dataPath, 'utf8'));
if (data.code !== code) fail(`data.code is "${data.code}" but you asked for "${code}"`);
validate(data);

const photoDir = join(HERE, 'photos', code);
if (!existsSync(photoDir)) {
  fail(`no photos at shares/photos/${code}/ — drop the three images in first`);
}
for (const p of data.photos) {
  if (!existsSync(join(photoDir, p.file))) {
    const have = (await readdir(photoDir)).join(', ') || '(empty)';
    fail(`shares/photos/${code}/${p.file} not found. present: ${have}`);
  }
}

const outDir = join(REPO, 'frontend', 'public', 'p', code);
await mkdir(outDir, { recursive: true });

// derived, never authored: gaps between photos, long date, arc aria-label
const harvestedLong = new Date(`${data.harvested}T12:00:00Z`).toLocaleDateString('en-GB', {
  day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC',
});
const photos = data.photos.map((p, i) => ({
  ...p, gapDays: i ? p.day - data.photos[i - 1].day : 0,
}));
const phases = data.phases.map((p) => ({
  ...p, color: PHASE_COLORS[p.label.toLowerCase()] ?? PHASE_COLORS.seedling,
}));

const html = render(await readFile(join(HERE, 'template.html'), 'utf8'), {
  ...data,
  host: HOST,
  harvestedLong,
  photos,
  phases,
  record: Object.entries(data.record).map(([label, value]) => ({ label, value })),
  lede: data.lede ?? `${data.totalDays} days from seed, in living soil.`,
  arcLabel: `${data.totalDays} days from seed: ` +
    phases.map((p) => `${p.days} days ${p.label.toLowerCase()}`).join(', ') + '.',
});
await writeFile(join(outDir, 'index.html'), html);

// Photos are downscaled and stripped of metadata on the way out -- the page
// is opened from a phone, often on cell data, and originals off a camera run
// ~700KB each. Originals in shares/photos/ are never modified. Falls back to
// a plain copy if ffmpeg isn't installed.
const MAX_W = 1100;
let photoBytes = 0;
for (const p of data.photos) {
  const src = join(photoDir, p.file);
  const dst = join(outDir, p.file);
  try {
    execFileSync('ffmpeg', [
      '-y', '-loglevel', 'error', '-i', src,
      '-vf', `scale='min(${MAX_W},iw)':-2`,
      '-q:v', '4', '-map_metadata', '-1', dst,
    ], { stdio: 'pipe' });
  } catch {
    await copyFile(src, dst);
  }
  photoBytes += (await stat(dst)).size;
}

// fonts live once at /p/_fonts/ and are cached across every share page
const fontDir = join(REPO, 'frontend', 'public', 'p', '_fonts');
await mkdir(fontDir, { recursive: true });
for (const f of await readdir(join(HERE, 'fonts'))) {
  if (extname(f) === '.woff2') {
    await copyFile(join(HERE, 'fonts', f), join(fontDir, f));
  }
}

const url = `https://${HOST}/p/${code}`;
let qrNote = 'skipped (--no-qr)';
if (wantQr) {
  try {
    const QR = (await import('qrcode')).default;
    // Byte mode, 37 chars -> Version 3 (29x29) at EC M with headroom.
    // Print at >= 1.0" on MATTE stock; gloss over mylar defeats scanners.
    const svg = await QR.toString(url, {
      type: 'svg', errorCorrectionLevel: 'M', margin: 4, color: { dark: '#1E3126', light: '#E9E0CA' },
    });
    await writeFile(join(HERE, `qr-${code}.svg`), svg);
    qrNote = `shares/qr-${code}.svg`;
  } catch (err) {
    qrNote = err.code === 'ERR_MODULE_NOT_FOUND'
      ? 'skipped — run `npm install` in shares/ to enable QR output'
      : `failed: ${err.message}`;
  }
}

console.log(`
  built  ${data.strain} · batch ${data.batch}
  page   frontend/public/p/${code}/index.html
  url    ${url}
  qr     ${qrNote}
  weight ${(photoBytes / 1024).toFixed(0)} KB of photos + ${(html.length / 1024).toFixed(0)} KB html

  deploy: aws codepipeline start-pipeline-execution --name GrowPipeline
`);
