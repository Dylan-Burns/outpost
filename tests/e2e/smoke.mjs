// Smoke test for an installed node, over its HTTP API only (no browser), so
// it runs the same on Linux and macOS, on a laptop or in CI.
//
//   node smoke.mjs <dashboard-url> <library-url> <setup-code>
//
// It claims the node, installs one small archive and one small map region,
// then checks that the library serves a page, search finds it and the map
// file can be read in ranges. Exits 0 and prints PASS, or 1 with the reason.
const [base, lib, code] = process.argv.slice(2);
if (!base || !lib || !code) {
  console.error("usage: node smoke.mjs <dashboard-url> <library-url> <setup-code>");
  process.exit(2);
}
const ARCHIVE = process.env.SMOKE_ARCHIVE || "devdocs_en_python";
const REGION = process.env.SMOKE_REGION || "map-pcn";
const PASSWORD = "smoke test passphrase";

let cookie = "";
let csrf = "";
const fail = (msg) => {
  console.error("FAIL:", msg);
  process.exit(1);
};
async function api(method, path, body) {
  const headers = { Origin: base };
  if (cookie) headers.Cookie = cookie;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET" && csrf) headers["X-CSRF-Token"] = csrf;
  const res = await fetch(base + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const set = res.headers.getSetCookie?.() || [];
  if (set.length) cookie = set.map((c) => c.split(";")[0]).join("; ");
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    json = { raw: text.slice(0, 200) };
  }
  return { status: res.status, json };
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const step = (name) => console.log("ok ", name);

let r = await api("GET", "/api/status");
if (r.status !== 200) fail(`status returned ${r.status}`);
if (r.json.claimed) fail("the node is already claimed; this test needs a fresh one");
step(`node ${r.json.version} is up and unclaimed`);

r = await api("POST", "/api/setup/claim", { code: "AAAA-BBBB-CCCC", password: PASSWORD });
if (r.status !== 403) fail(`a wrong setup code was not refused (${r.status})`);
r = await api("POST", "/api/setup/claim", { code, password: PASSWORD });
if (r.status !== 200 || !r.json.csrf) fail(`claim failed: ${r.status} ${JSON.stringify(r.json)}`);
csrf = r.json.csrf;
step("claimed with the setup code");

r = await api("POST", "/api/setup/complete", {
  nodeName: "Smoke",
  apps: { library: true, maps: true, assistant: false },
  selection: { wikipedia: "none", tiers: {}, regions: [REGION], models: [], extra: [ARCHIVE] },
});
if (r.status !== 200) fail(`setup did not complete: ${r.status} ${JSON.stringify(r.json)}`);
if (!r.json.items.includes(ARCHIVE) || !r.json.items.includes(REGION)) fail(`selection came back without the test items: ${r.json.items}`);
step(`setup complete, ${r.json.items.length} downloads queued`);

const deadline = Date.now() + 8 * 60 * 1000;
for (;;) {
  r = await api("GET", "/api/jobs");
  const jobs = r.json.jobs || [];
  const bad = jobs.filter((j) => j.state === "failed");
  if (bad.length) fail(`download failed: ${bad.map((j) => `${j.itemId}: ${j.error}`).join("; ")}`);
  if (jobs.length && jobs.every((j) => j.state === "done")) break;
  if (Date.now() > deadline) fail(`downloads did not finish: ${jobs.map((j) => `${j.itemId}=${j.state}`).join(" ")}`);
  await sleep(2000);
}
step("every download finished and verified");

// kiwix-serve restarts when the library changes; give it a moment.
let books = [];
for (let i = 0; i < 30; i++) {
  r = await api("GET", "/api/library");
  books = r.json.books || [];
  if (r.json.running && books.some((b) => b.id === ARCHIVE)) break;
  await sleep(1000);
}
const book = books.find((b) => b.id === ARCHIVE);
if (!book || !r.json.running) fail(`the library is not serving ${ARCHIVE}: ${JSON.stringify(r.json).slice(0, 200)}`);
const page = await fetch(lib + book.path + "/", { redirect: "follow" });
const html = await page.text();
if (page.status !== 200 || html.length < 200) fail(`the library page returned ${page.status} with ${html.length} bytes`);
if (!/frame-ancestors|default-src/.test(page.headers.get("content-security-policy") || "")) fail("the library origin sent no content security policy");
step(`library serves ${book.title} (${html.length} bytes)`);

let found = 0;
for (let i = 0; i < 20 && !found; i++) {
  r = await api("GET", "/api/search?q=" + encodeURIComponent("dictionary"));
  if (r.status === 200) found = r.json.total;
  if (!found) await sleep(1000);
}
if (!found) fail(`search returned nothing: ${r.status} ${JSON.stringify(r.json).slice(0, 200)}`);
step(`search finds ${found} results`);

r = await api("GET", "/api/maps");
const region = (r.json.regions || []).find((x) => x.id === REGION);
if (!region) fail(`the map region is missing: ${JSON.stringify(r.json).slice(0, 300)}`);
const tiles = await fetch(`${base}/maps/files/${region.file}`, { headers: { Range: "bytes=0-6", Cookie: cookie } });
const head = Buffer.from(await tiles.arrayBuffer()).toString("latin1");
if (tiles.status !== 206 || head !== "PMTiles") fail(`the map file did not answer a range request (${tiles.status}, ${JSON.stringify(head)})`);
step(`map region ${region.name} is installed and readable`);

r = await api("GET", "/api/system");
if (r.status !== 200 || !(r.json.disk.total > 0) || !(r.json.system.memBytes > 0)) fail(`system report is incomplete: ${JSON.stringify(r.json).slice(0, 300)}`);
step(`system: ${r.json.system.os}/${r.json.system.arch}, ${Math.round(r.json.system.memBytes / 2 ** 30)} GB memory, ${Math.round(r.json.disk.free / 1e9)} GB free`);

// A write without the session token must be refused.
const saved = csrf;
csrf = "wrong";
r = await api("POST", "/api/jobs/clear", {});
if (r.status !== 403) fail(`a write with a bad token was accepted (${r.status})`);
csrf = saved;
step("writes without the session token are refused");

console.log("PASS");
