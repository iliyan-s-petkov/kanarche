import { chromium } from '../../web/node_modules/playwright/index.mjs';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const outDir = path.join(root, 'internal/web/static/og');
const kinds = { oblasti: 'oblast', cities: 'city', 'sofia-districts': 'district' };
const langs = ['bg', 'en'];

const areas = [];
for (const [file, kind] of Object.entries(kinds)) {
  const coll = JSON.parse(fs.readFileSync(path.join(root, `data/boundaries/${file}.geojson`), 'utf8'));
  for (const f of coll.features) areas.push({ kind, props: f.properties, geometry: f.geometry });
}
const cat = Object.fromEntries(langs.map(l => [l, JSON.parse(fs.readFileSync(path.join(root, `internal/i18n/${l}.json`), 'utf8'))]));
const mascot = fs.readFileSync(path.join(root, 'internal/web/static/kanarche-mascot.svg'), 'utf8').replace(/<style>[\s\S]*?<\/style>/, '');

// Mirrors rowFrom (area.name.<slug> over name_en/name_bg) and oblastForm (seo.oblast.label.<slug> over the label with {name}).
function cardName(a, lang) {
  const c = cat[lang];
  const { slug, name_bg: bg, name_en: en } = a.props;
  let name = lang === 'bg' ? bg : (en || bg);
  if (c['area.name.' + slug]) name = c['area.name.' + slug];
  if (a.kind !== 'oblast') return name;
  const key = 'seo.oblast.label.' + slug;
  return c[key] ?? c['seo.oblast.label'].replaceAll('{name}', name);
}

const esc = s => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');

// Equirectangular fit with a cos(lat) correction.
function outline(geometry, w, h) {
  const polys = geometry.type === 'Polygon' ? [geometry.coordinates] : geometry.coordinates;
  const pts = polys.flat(2);
  const k = Math.cos((pts.reduce((s, p) => s + p[1], 0) / pts.length) * Math.PI / 180);
  const xs = pts.map(p => p[0] * k), ys = pts.map(p => -p[1]);
  const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)];
  const s = Math.min(w / (x1 - x0), h / (y1 - y0));
  const ox = (w - (x1 - x0) * s) / 2, oy = (h - (y1 - y0) * s) / 2;
  const d = polys.map(poly => poly.map(r => r.map((p, i) => (i ? 'L' : 'M') + ((p[0] * k - x0) * s + ox).toFixed(1) + ' ' + ((-p[1] - y0) * s + oy).toFixed(1)).join('') + 'Z').join('')).join('');
  return `<svg viewBox="0 0 ${w} ${h}" width="${w}" height="${h}"><path d="${d}" fill="#edf5ff" stroke="#0f62fe" stroke-width="4" stroke-linejoin="round" fill-rule="evenodd"/></svg>`;
}

const html = (a, lang) => `<html><body style="margin:0"><style>
body{width:1200px;height:630px;background:#f7f8fa;font-family:"IBM Plex Sans","Helvetica Neue",Arial,sans-serif;color:#161616;position:relative;overflow:hidden}
.card{position:absolute;inset:40px;background:#fff;border:2px solid #c6c6c6;border-radius:8px}
.l{position:absolute;left:80px;top:80px;width:540px;height:400px;display:flex;align-items:center}
.n{width:540px;font-size:96px;font-weight:700;line-height:1.1;letter-spacing:-1px}
.o{position:absolute;right:70px;top:70px;width:500px;height:490px}
.b{position:absolute;left:80px;bottom:70px;display:flex;align-items:center;gap:14px;font-size:38px;font-weight:700;color:#0f62fe}
.b svg{width:64px;height:64px}
</style><div class="card"></div>
<div class="l"><div class="n" id="n">${esc(cardName(a, lang))}</div></div><div class="o">${outline(a.geometry, 500, 490)}</div>
<div class="b">${mascot}<span>${esc(cat[lang]['brand.wordmark'])}</span></div></body></html>`;

// Shrinks the name until it fits two lines inside the box without breaking a word.
const fit = () => {
  const n = document.getElementById('n');
  for (let px = 96; px >= 40; px -= 2) {
    n.style.fontSize = px + 'px';
    if (n.scrollWidth <= n.clientWidth && n.offsetHeight <= px * 1.1 * 2 + 1) return px;
  }
  return 40;
};

const b = await chromium.launch();
const pg = await b.newPage({ viewport: { width: 1200, height: 630 } });
const cdp = await pg.context().newCDPSession(pg);
await cdp.send('DOM.enable');
await cdp.send('CSS.enable');
let total = 0, count = 0;
const fonts = new Set();
for (const a of areas) {
  for (const lang of langs) {
    await pg.setContent(html(a, lang));
    const px = await pg.evaluate(fit);
    const doc = await cdp.send('DOM.getDocument');
    const { nodeId } = await cdp.send('DOM.querySelector', { nodeId: doc.root.nodeId, selector: '#n' });
    for (const f of (await cdp.send('CSS.getPlatformFontsForNode', { nodeId })).fonts) fonts.add(f.familyName);
    const raw = await pg.screenshot();
    const png = execFileSync('pngquant', ['--quality', '60-90', '--colors', '32', '-'], { input: raw, maxBuffer: 1 << 24 });
    const dir = path.join(outDir, lang);
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, a.props.slug + '.png'), png);
    total += png.length;
    count++;
    if (px < 96) console.log(`${lang}/${a.props.slug}: ${px}px`);
  }
}
await b.close();
console.log(`${count} cards, ${total} bytes, fonts: ${[...fonts].join(', ')}`);
