import * as esbuild from "esbuild";
import { mkdir, rm, writeFile, readFile } from "node:fs/promises";
import { createHash } from "node:crypto";

const out = new URL("../internal/server/ui/dist/", import.meta.url).pathname;
await rm(out, { recursive: true, force: true });
await mkdir(out + "assets", { recursive: true });

const result = await esbuild.build({
  entryPoints: { app: "src/main.tsx" },
  bundle: true,
  splitting: true,
  format: "esm",
  outdir: out + "assets",
  entryNames: "[name]-[hash]",
  chunkNames: "chunk-[hash]",
  assetNames: "[name]-[hash]",
  minify: true,
  sourcemap: false,
  target: ["es2020", "safari15"],
  jsx: "automatic",
  jsxImportSource: "preact",
  metafile: true,
  legalComments: "linked",
  // The two typefaces travel inside the program: an Outpost has no internet to fetch them from.
  loader: { ".woff2": "file" },
  define: { "process.env.NODE_ENV": '"production"' },
  logLevel: "warning",
});

const outputs = Object.keys(result.metafile.outputs).map((p) => p.split("/assets/")[1]);
const js = outputs.find((p) => p.startsWith("app-") && p.endsWith(".js"));
const css = outputs.filter((p) => p.endsWith(".css") && p.startsWith("app-"));
const html = (await readFile("src/index.html", "utf8"))
  .replace("<!--CSS-->", css.map((c) => `<link rel="stylesheet" href="/assets/${c}">`).join(""))
  .replace("<!--JS-->", `<script type="module" src="/assets/${js}"></script>`);
await writeFile(out + "index.html", html);
await writeFile(out + "favicon.svg", await readFile("src/favicon.svg"));
const total = Object.values(result.metafile.outputs).reduce((n, o) => n + o.bytes, 0);
console.log(`dashboard built: ${outputs.length} files, ${(total / 1024).toFixed(0)} KiB`);
