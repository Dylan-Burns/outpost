// Drives an installed node: re-claim after reset-admin, home, library, search,
// open an article on the library origin, maps, downloads.
import { chromium } from "playwright";
const [base, code, shots] = process.argv.slice(2);
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1280, height: 860 } });
const p = await ctx.newPage();
const errors = [];
p.on("pageerror", (e) => errors.push(String(e)));
const step = async (name, fn) => { await fn(); await p.screenshot({ path: `${shots}/vm-${name}.png` }); console.log("ok", name); };
await step("claim", async () => {
  await p.goto(base);
  await p.getByRole("heading", { name: "Welcome to Outpost" }).waitFor();
  await p.getByLabel("Setup code").fill(code);
  await p.getByLabel("Choose an admin password").fill("vm browser password");
  await p.getByLabel("Repeat the password").fill("vm browser password");
  await p.getByRole("button", { name: "Claim this node" }).click();
  await p.getByRole("heading", { name: "VM node" }).waitFor();
});
await step("library", async () => {
  await p.getByRole("link", { name: "Library", exact: true }).click();
  await p.getByText("Chemistry by Wikipedia (text only)").waitFor();
});
await step("search", async () => {
  await p.getByRole("link", { name: "Search", exact: true }).click();
  await p.getByLabel("Search terms").fill("catalyst vanadium");
  await p.getByRole("button", { name: "Search" }).click();
  await p.locator(".result").first().waitFor();
});
await step("article", async () => {
  const [tab] = await Promise.all([ctx.waitForEvent("page"), p.locator(".result a.title").first().click()]);
  await tab.waitForLoadState("networkidle");
  const text = await tab.locator("body").innerText();
  if (text.length < 200) throw new Error("article empty");
  await tab.screenshot({ path: `${shots}/vm-article-tab.png` });
  await tab.close();
});
await step("maps", async () => {
  await p.getByRole("link", { name: "Maps", exact: true }).click();
  await p.locator("#map canvas").waitFor();
  await p.waitForTimeout(3500);
});
await step("downloads", async () => {
  await p.getByRole("link", { name: "Downloads" }).click();
  await p.getByRole("heading", { name: "Downloads" }).waitFor();
});
console.log(errors.length ? "ERRORS " + errors.join(" | ") : "PASS: installed node works in the browser");
await b.close();
