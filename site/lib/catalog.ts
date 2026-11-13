import { readFileSync } from "node:fs";
import path from "node:path";

export type Item = {
  id: string;
  kind: string;
  title: string;
  description?: string;
  publisher?: string;
  license?: string;
  size: number;
  date?: string;
  notice?: string;
  group?: string;
  model?: string;
  ramGB?: number;
};
export type Category = {
  id: string;
  name: string;
  description: string;
  kind: "tiered" | "single";
  tiers?: Record<string, string[]>;
  options?: { id: string; label: string; description?: string; items: string[] }[];
  default?: string;
};
export type Catalog = { sequence: number; generated: string; mapBuild: string; categories: Category[]; items: Item[] };

let cached: Catalog | null = null;
export function loadCatalog(): Catalog {
  if (!cached) {
    const p = path.join(process.cwd(), "public", "catalog", "v1", "catalog.json");
    cached = JSON.parse(readFileSync(p, "utf8")) as Catalog;
  }
  return cached;
}

export type Release = { version: string; date: string };
export function loadRelease(): Release {
  try {
    return JSON.parse(readFileSync(path.join(process.cwd(), "public", "releases", "latest.json"), "utf8"));
  } catch {
    return { version: "unreleased", date: "" };
  }
}

export function bytes(n: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}

/** The data the planner needs, kept small. */
export function plannerData() {
  const c = loadCatalog();
  const items: Record<string, { t: string; s: number; k: string; g?: string; r?: number }> = {};
  for (const i of c.items) items[i.id] = { t: i.title, s: i.size, k: i.kind, ...(i.group ? { g: i.group } : {}), ...(i.ramGB ? { r: i.ramGB } : {}) };
  return {
    items,
    categories: c.categories.map((x) => ({ id: x.id, name: x.name, description: x.description, kind: x.kind, tiers: x.tiers, options: x.options, default: x.default })),
  };
}
export type PlannerData = ReturnType<typeof plannerData>;
