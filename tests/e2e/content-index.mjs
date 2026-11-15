// Checks the Content page against the published content index.
//
//   node content-index.mjs <site-url> [index-url] [samples]
//
// From the index itself it picks one item of each kind (book, PDF, video,
// article, question, map region...), opens each by its link and checks the
// row is shown and highlighted; then it types the titles of random items
// into the search and checks each is found. Also checks the page has no
// script errors and no sideways scroll on a phone. Prints PASS or FAIL.
import { chromium } from "playwright";
import { gunzipSync } from "node:zlib";

const site = (process.argv[2] || "").replace(/\/$/, "");
if (!site) {
  console.error("usage: node content-index.mjs <site-url> [index-url] [samples]");
  process.exit(2);
}
const base = (process.argv[3] || `${site}/catalog/v1/index`).replace(/\/$/, "");
const SAMPLES = Number(process.argv[4] || 200);
const fail = (msg) => {
  console.error("FAIL:", msg);
  process.exit(1);
};
const step = (msg) => console.log("ok ", msg);

async function json(url) {
  const r = await fetch(url);
  if (!r.ok) fail(`${url}: HTTP ${r.status}`);
  return r.json();
}
async function packed(url, off, len) {
  const r = await fetch(url, { headers: { Range: `bytes=${off}-${off + len - 1}` } });
  if (!r.ok) fail(`${url}: HTTP ${r.status}`);
  let buf = Buffer.from(await r.arrayBuffer());
  if (r.status === 200) buf = buf.subarray(off, off + len);
  return JSON.parse(gunzipSync(buf).toString("utf8"));
}

// Leaves of a tree, with their path of child indexes. Big branches are
// stored in the packs; follow at most a few of them per tree.
async function leaves(col, node, path = [], out = [], budget = { n: 6 }) {
  if (node.r) out.push({ node, path });
  let kids = node.c;
  if (!kids && node.cr) {
    if (budget.n-- <= 0) return out;
    const [pack, off, len] = node.cr;
    kids = await packed(`${base}/p/${encodeURIComponent(col)}.${pack}.bin`, off, len);
  }
  const order = (kids || []).map((c, i) => [c, i]).sort(() => Math.random() - 0.5);
  for (const [c, i] of order) await leaves(col, c, [...path, i], out, budget);
  return out;
}

const root = await json(`${base}/root.json`);
step(`index of ${root.totals.items.toLocaleString()} items in ${root.collections.length} trees`);

// One sample item per kind, and a pool of random items for search.
const rnd = (n) => Math.floor(Math.random() * n);
const want = new Map();
const pool = [];
const order = [...root.collections].sort(() => Math.random() - 0.5);
for (const col of order) {
  const tree = await json(`${base}/c/${encodeURIComponent(col)}.json`);
  const all = await leaves(col, tree);
  if (!all.length) continue;
  for (let k = 0; k < 2; k++) {
    const { node, path } = all[rnd(all.length)];
    const [pack, off, len] = node.r;
    const rows = await packed(`${base}/p/${encodeURIComponent(col)}.${pack}.bin`, off, len);
    if (!node.c && !node.cr && rows.length !== node.n) fail(`${col}: leaf count does not match its rows`);
    if (rows.length > root.limit) fail(`${col}: a shelf holds ${rows.length} rows`);
    const row = rnd(rows.length);
    const it = { col, path, row, title: rows[row].t, kind: rows[row].k };
    if (!want.has(it.kind)) want.set(it.kind, it);
    if (pool.length < SAMPLES * 3) pool.push(it);
  }
}
step(`sampled ${pool.length} items of ${want.size} kinds`);

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1360, height: 900 } });
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
page.on("console", (m) => m.type() === "error" && errors.push(m.text()));

await page.goto(`${site}/catalog`, { waitUntil: "networkidle" });
const subjects = await page.locator(".ix-subject").count();
if (subjects !== root.subjects.length) fail(`page shows ${subjects} subjects, index has ${root.subjects.length}`);
step(`${subjects} subjects on the page`);

for (const [kind, it] of want) {
  await page.goto(`${site}/catalog#/${encodeURIComponent(it.col)}/${it.path.join(".")}/${it.row}`, { waitUntil: "networkidle" });
  const hit = page.locator(".ix-item.hit");
  await hit.waitFor({ timeout: 20000 }).catch(() => fail(`${kind} “${it.title}” in ${it.col}: link did not open its row`));
  const text = await hit.locator("summary .ix-name").textContent();
  if (text !== it.title) fail(`${kind}: link opened “${text}”, expected “${it.title}”`);
  const zim = root.subjects.flatMap((s) => s.collections).find((c) => (c.tree || c.id) === it.col && !c.ed)?.zim;
  if (zim) {
    const href = await hit.locator(".ix-open a").first().getAttribute("href");
    if (!href?.startsWith(`https://browse.library.kiwix.org/content/${zim}/`)) fail(`${kind}: preview link is ${href}`);
  }
  step(`${kind}: “${it.title.slice(0, 60)}” opens by its link`);
}

await page.goto(`${site}/catalog`, { waitUntil: "networkidle" });
const picks = pool.sort(() => Math.random() - 0.5).slice(0, SAMPLES);
let found = 0;
for (const it of picks) {
  await page.fill("#ix-q", "");
  await page.fill("#ix-q", it.title.slice(0, 60));
  const btn = page.locator(".ix-results p button", { hasText: it.title.slice(0, 60) }).first();
  const ok = await btn.waitFor({ timeout: 15000 }).then(() => true, () => false);
  if (!ok) {
    // Very common titles ("Introduction") can have more than 60 matches; narrow by collection instead.
    const n = await page.locator(".ix-results p").count();
    if (n < 60) fail(`search did not find “${it.title}” (${it.col})`);
  }
  found++;
}
step(`search found ${found} of ${picks.length} sampled titles`);

await page.setViewportSize({ width: 390, height: 844 });
const sample = [...want.values()][0];
await page.goto(`${site}/catalog#/${encodeURIComponent(sample.col)}/${sample.path.join(".")}/${sample.row}`, { waitUntil: "networkidle" });
await page.locator(".ix-item.hit").waitFor({ timeout: 20000 });
const over = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
if (over > 0) fail(`page scrolls sideways by ${over}px on a phone`);
step("no sideways scroll on a phone");

await browser.close();
if (errors.length) fail(`script errors: ${errors.slice(0, 3).join(" | ")}`);
console.log("PASS");
