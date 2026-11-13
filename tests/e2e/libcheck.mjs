import { chromium } from "playwright";
const [lib, out] = process.argv.slice(2);
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 1280, height: 900 } });
const problems = [];
p.on("console", (m) => { if (/Content Security Policy|Refused to/i.test(m.text())) problems.push(m.text()); });
p.on("pageerror", (e) => problems.push(String(e)));
for (const path of ["/", "/viewer#wikipedia_en_chemistry_nopic/A/Contact_process", "/content/wikipedia_en_chemistry_nopic/Contact_process", "/content/zimgit-water_en/"]) {
  await p.goto(lib + path, { waitUntil: "networkidle" });
  await p.waitForTimeout(1500);
  const text = await p.evaluate(() => {
    const f = document.querySelector("iframe");
    try { return (f?.contentDocument?.body?.innerText || document.body.innerText).slice(0, 80); } catch { return document.body.innerText.slice(0, 80); }
  });
  console.log(path, "->", JSON.stringify(text));
}
await p.goto(lib + "/viewer#wikipedia_en_chemistry_nopic/Contact_process", { waitUntil: "networkidle" });
await p.waitForTimeout(2000);
await p.screenshot({ path: out });
console.log("CSP/JS problems:", problems.length ? problems : "none");
await b.close();
