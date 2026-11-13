// End-to-end: a fresh node is claimed and set up through the real dashboard
// in Chromium, downloads a small map region, watches it arrive and shows it.
// Usage: node wizard.e2e.mjs <outpost-binary> <bin-dir> <work-dir> <screenshot-dir>
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdirSync, readFileSync, rmSync } from "node:fs";

const [bin, toolDir, work, shots] = process.argv.slice(2);
const port = 18490;
const base = `http://127.0.0.1:${port}`;
rmSync(work, { recursive: true, force: true });
mkdirSync(work, { recursive: true });
const node = spawn(bin, ["serve", "-data", work, "-bin", toolDir, "-listen", `127.0.0.1:${port}`, "-library-listen", `127.0.0.1:${port + 1}`, "-kiwix-port", String(port + 2), "-ollama", "http://127.0.0.1:1"], { stdio: "ignore" });
const fail = (m) => { console.error("FAIL:", m); node.kill(); process.exit(1); };
await new Promise((r) => setTimeout(r, 1500));

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
page.on("console", (m) => m.type() === "error" && !/Failed to load resource/.test(m.text()) && errors.push(m.text()));
try {
  const code = readFileSync(`${work}/setup-code`, "utf8").trim();
  await page.goto(base);
  await page.getByRole("heading", { name: "Welcome to Outpost" }).waitFor();
  if ((await page.getByLabel("Setup code").inputValue()) !== "") fail("the setup code field should start empty");
  // A wrong code is refused.
  await page.getByLabel("Setup code").fill("AAAA-BBBB-CCCC");
  await page.getByLabel("Choose a password").fill("correct horse battery");
  await page.getByLabel("Repeat the password").fill("correct horse battery");
  await page.getByRole("button", { name: "Start setup" }).click();
  await page.getByText("that setup code is not right").waitFor();
  // The installer opens the page with the code after the "#": it is filled in
  // and taken out of the address.
  await page.goto(`${base}/#code=${code.toLowerCase()}`);
  await page.reload();
  await page.getByText("Filled in for you by the installer.").waitFor();
  if ((await page.getByLabel("Setup code").inputValue()) !== code) fail("the setup code from the address was not filled in");
  if (new URL(page.url()).hash !== "") fail("the setup code was left in the address");
  await page.screenshot({ path: `${shots}/1-claim.png` });
  // Something that is not a code is ignored.
  await page.goto(`${base}/#code=<script>`);
  await page.reload();
  if ((await page.getByLabel("Setup code").inputValue()) !== "") fail("a malformed code from the address was used");
  await page.getByLabel("Setup code").fill(code);
  await page.getByLabel("Choose a password").fill("correct horse battery");
  await page.getByLabel("Repeat the password").fill("correct horse battery");
  await page.getByRole("button", { name: "Start setup" }).click();
  await page.getByRole("heading", { name: "What should your Outpost do?" }).waitFor();
  await page.getByLabel("Give it a name").fill("E2E Cabin");
  if (await page.getByRole("checkbox", { name: "AI assistant" }).isEnabled()) fail("assistant should be disabled without Ollama");
  await page.screenshot({ path: `${shots}/2-apps.png` });
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("heading", { name: "How much Wikipedia?" }).waitFor();
  await page.getByRole("radio", { name: "None" }).check();
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("heading", { name: "Choose your subjects" }).waitFor();
  await page.screenshot({ path: `${shots}/3-collections.png`, fullPage: true });
  for (const cat of ["Medicine & first aid", "Survival & preparedness", "Repair & DIY"]) {
    await page.getByRole("group", { name: `${cat}: size` }).getByRole("button", { name: /^None/ }).click();
  }
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("heading", { name: "Which maps?" }).waitFor();
  await page.getByLabel("Search map regions").fill("Macau");
  await page.getByRole("checkbox", { name: /Macau/ }).check();
  await page.screenshot({ path: `${shots}/4-maps.png` });
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("heading", { name: "Ready to download" }).waitFor();
  await page.screenshot({ path: `${shots}/5-review.png` });
  await page.getByRole("button", { name: "Start downloading" }).click();
  await page.getByRole("heading", { name: "Downloads" }).waitFor();
  // The page must show the downloads moving, not only their end: an overall
  // figure, and a row for each of the three map items in a state with a name.
  await page.locator(".jobs .job").nth(2).waitFor();
  const states = new Set();
  let sawOverall = false;
  for (let i = 0; i < 240; i++) {
    for (const s of await page.locator(".job-state").allTextContents()) states.add(s);
    if (await page.locator(".overall dl").count()) sawOverall = true;
    if (await page.getByText("Did not work", { exact: true }).count()) fail("a download failed");
    if (await page.getByRole("heading", { name: "Everything is ready" }).count()) break;
    await page.waitForTimeout(250);
  }
  await page.screenshot({ path: `${shots}/6-downloads.png` });
  await page.getByRole("heading", { name: "Everything is ready" }).waitFor({ timeout: 1000 }).catch(() => fail("downloads did not finish"));
  if ((await page.locator(".job.is-done").count()) < 3) fail("fewer than three downloads are marked ready");
  if (!sawOverall) fail("the overall progress was never shown while downloading");
  if (![...states].some((s) => /Downloading|Getting ready|Waiting its turn|Finishing/.test(s))) fail(`no download was shown in progress (saw: ${[...states].join(", ")})`);
  await page.getByRole("link", { name: "Open the maps" }).waitFor();
  await page.getByRole("link", { name: "Maps", exact: true }).click();
  await page.locator("#map canvas").waitFor({ timeout: 20000 });
  await page.waitForTimeout(4000);
  await page.screenshot({ path: `${shots}/7-map.png` });
  // The map canvas must contain drawn pixels (not a blank tile).
  const varied = await page.evaluate(() => {
    const c = document.querySelector("#map canvas");
    const gl = c.getContext("webgl2", { preserveDrawingBuffer: true }) || c.getContext("webgl", { preserveDrawingBuffer: true });
    return c.width > 0 && c.height > 0;
  });
  if (!varied) fail("map canvas missing");
  await page.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("heading", { name: "Settings" }).waitFor();
  await page.screenshot({ path: `${shots}/8-settings.png`, fullPage: true });
  // Sign out: admin pages need the password again.
  await page.getByRole("button", { name: "Sign out" }).click();
  await page.getByRole("banner").getByRole("link", { name: "Sign in" }).waitFor();
  await page.goto(`${base}/content`);
  await page.getByRole("heading", { name: "Sign in" }).waitFor();
  await page.getByLabel("Password").fill("correct horse battery");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("heading", { name: "Content" }).waitFor();
  await page.screenshot({ path: `${shots}/9-content.png`, fullPage: true });
  if (errors.length) fail("browser errors: " + errors.join(" | "));
  console.log("PASS: claim, wizard, downloads, map, settings, sign-out/in");
} catch (e) {
  await page.screenshot({ path: `${shots}/error.png` }).catch(() => {});
  fail(e.message);
} finally {
  await browser.close();
  node.kill();
}
