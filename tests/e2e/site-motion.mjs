// Does the hero globe on the site move, on this machine, in each browser?
//
//   node site-motion.mjs https://projectoutpost.org chromium,firefox,msedge
//
// A browser name is one Playwright ships (chromium, firefox, webkit) or a
// channel of an installed one (chrome, msedge). Each is tried twice: as the
// system is, and with the system asking for reduced motion, which on Windows
// is the "animation effects" switch and is often off. The globe must move in
// both. Exits non-zero if it stands still anywhere.
import { chromium, firefox, webkit } from "playwright";

const [url = "https://projectoutpost.org", list = "chromium"] = process.argv.slice(2);
const engines = { chromium, firefox, webkit };
let failed = 0;

// A fingerprint of what the canvas shows. Two taken a moment apart differ if it is moving.
const picture = (page) =>
  page.evaluate(() => {
    const c = document.querySelector(".globe canvas");
    const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
    let h = 0;
    let lit = 0;
    for (let i = 3; i < d.length; i += 4 * 53) {
      h = (h * 31 + d[i]) | 0;
      if (d[i] > 40) lit++;
    }
    return { h, lit, size: [c.width, c.height] };
  });

for (const name of list.split(",").filter(Boolean)) {
  const engine = engines[name] || chromium;
  const launch = engines[name] ? {} : { channel: name };
  let browser;
  try {
    browser = await engine.launch(launch);
  } catch (e) {
    console.log(`${name}: not available here (${String(e.message).split("\n")[0]})`);
    continue;
  }
  for (const motion of ["as the system is", "reduce"]) {
    const context = await browser.newContext({ viewport: { width: 1366, height: 768 }, ...(motion === "reduce" ? { reducedMotion: "reduce" } : {}) });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e).slice(0, 200)));
    page.on("console", (m) => m.type() === "error" && errors.push(m.text().slice(0, 200)));
    let line;
    try {
      await page.goto(url, { waitUntil: "networkidle", timeout: 60000 });
      await page.waitForSelector(".globe[data-ready]", { timeout: 30000 });
      await page.waitForTimeout(1200);
      const asksForLess = await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches);
      const a = await picture(page);
      await page.waitForTimeout(1000);
      const b = await picture(page);
      const button = (await page.locator(".globe-toggle").textContent()).trim();
      const moves = a.h !== b.h && a.lit > 100;
      if (!moves || errors.length) failed++;
      line = `${moves ? "MOVES" : "STANDS STILL"} | button says ${button} | system asks for reduced motion: ${asksForLess} | canvas ${a.size.join("x")} | errors: ${errors.length ? errors.join(" ; ") : "none"}`;
    } catch (e) {
      failed++;
      line = `FAILED to check: ${String(e.message).split("\n")[0]}`;
    }
    console.log(`${name.padEnd(9)} ${motion.padEnd(17)} ${line}`);
    await context.close();
  }
  console.log(`${name.padEnd(9)} ${await browser.version()} on ${process.platform}`);
  await browser.close();
}
process.exit(failed ? 1 : 0);
