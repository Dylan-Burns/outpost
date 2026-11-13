// Shared pieces of the setup wizard and the content manager: computing what a
// selection needs, and the editors for each part of it.
import { useMemo, useState } from "preact/hooks";
import type { Apps, Catalog, CatalogResponse, Category, Item, Selection } from "./api";
import { bytes } from "./format";
import { Icon } from "./icons";

export const TIERS = ["essential", "standard", "comprehensive"] as const;
const TIER_LABEL: Record<string, string> = { none: "None", essential: "Essential", standard: "Standard", comprehensive: "Comprehensive" };

export function itemMap(c: Catalog): Map<string, Item> {
  return new Map(c.items.map((i) => [i.id, i]));
}

/** Mirrors the server's Resolve: the item ids a selection needs. */
export function resolve(c: Catalog, sel: Selection, apps: Apps): Set<string> {
  const want = new Set<string>();
  if (apps.library) {
    const wiki = c.categories.find((x) => x.id === "wikipedia");
    wiki?.options?.find((o) => o.id === sel.wikipedia)?.items.forEach((i) => want.add(i));
    for (const [cat, tier] of Object.entries(sel.tiers || {})) {
      const t = c.categories.find((x) => x.id === cat)?.tiers?.[tier];
      t?.forEach((i) => want.add(i));
    }
    sel.extra?.forEach((i) => want.add(i));
  }
  if (apps.maps && sel.regions.length) {
    sel.regions.forEach((r) => want.add(r));
    c.items.filter((i) => i.kind === "map-world" || i.kind === "map-assets").forEach((i) => want.add(i.id));
  }
  if (apps.assistant) sel.models.forEach((m) => want.add(m));
  return want;
}

export function sizeOf(items: Map<string, Item>, ids: Iterable<string>): number {
  let n = 0;
  for (const id of ids) n += items.get(id)?.size || 0;
  return n;
}

export function defaultSelection(c: Catalog): Selection {
  const wiki = c.categories.find((x) => x.id === "wikipedia");
  const tiers: Record<string, string> = {};
  for (const cat of c.categories) if (cat.kind === "tiered") tiers[cat.id] = ["medicine", "survival", "repair"].includes(cat.id) ? "essential" : "";
  return { wikipedia: wiki?.default || "none", tiers, regions: [], models: [] };
}

export type Plan = { want: Set<string>; add: string[]; remove: string[]; addBytes: number; removeBytes: number; totalBytes: number };

export function plan(data: CatalogResponse, sel: Selection, apps: Apps): Plan {
  const items = itemMap(data.catalog);
  const want = resolve(data.catalog, sel, apps);
  const add = [...want].filter((id) => !data.installed[id]);
  const remove = Object.keys(data.installed).filter((id) => !want.has(id));
  const removeBytes = remove.reduce((n, id) => n + (data.installed[id]?.size || 0), 0);
  return { want, add, remove, addBytes: sizeOf(items, add), removeBytes, totalBytes: sizeOf(items, want) };
}

export function StorageBar({ data, p }: { data: CatalogResponse; p: Plan }) {
  // Models live in Ollama's own folder, which may be another disk; count them anyway.
  const usedNow = data.disk.total - data.disk.free;
  const capacity = data.disk.total || 1;
  const after = usedNow + p.addBytes - p.removeBytes - data.disk.partial;
  const over = after > capacity * 0.98;
  const pct = (n: number) => `${Math.max(0, Math.min(100, (n / capacity) * 100))}%`;
  return (
    <div class="storage" aria-live="polite">
      <div class="spread small">
        <strong>
          {bytes(p.totalBytes)} chosen, {bytes(Math.max(0, p.addBytes))} still to download
        </strong>
        <span class={over ? "badge warn" : "muted"}>
          {over ? "Does not fit on the drive" : `${bytes(Math.max(0, capacity - after))} left free afterwards`}
        </span>
      </div>
      <div class="bar" role="img" aria-label={`Disk: ${bytes(usedNow)} used, ${bytes(p.addBytes)} to add, ${bytes(capacity)} total`}>
        <div class="used" style={{ width: pct(Math.min(usedNow, capacity)) }} />
        <div class={over ? "over" : "new"} style={{ width: pct(Math.min(p.addBytes, capacity - usedNow)) }} />
      </div>
      <div class="legend">
        <span><i style={{ background: "#6e6e6e" }} />In use {bytes(usedNow)}</span>
        <span><i style={{ background: "var(--fg)" }} />To add {bytes(p.addBytes)}</span>
        <span>Drive {bytes(capacity)}</span>
      </div>
    </div>
  );
}

export function AppsEditor({ apps, setApps, aiAvailable }: { apps: Apps; setApps: (a: Apps) => void; aiAvailable: boolean }) {
  const rows: [keyof Apps, string, string, boolean][] = [
    ["library", "Library", "Encyclopedias, manuals, books and lessons that anyone on your Wi-Fi can read and search in a browser.", true],
    ["maps", "Maps", "Street maps of the places you choose. They work with no internet.", true],
    [
      "assistant",
      "AI assistant",
      aiAvailable
        ? "Answers questions in plain words from your library, and shows where each answer came from. Runs on this computer only."
        : "Needs a free program called Ollama, which is not on this computer yet. On a Mac, install the Ollama app and open it once. On Linux, run the installer again with --with-ai.",
      aiAvailable,
    ],
  ];
  return (
    <div class="grid">
      {rows.map(([key, title, desc, enabled]) => (
        <label class={`choice ${apps[key] ? "on" : ""} ${enabled ? "" : "disabled"}`} key={key}>
          <div class="row">
            <input type="checkbox" checked={apps[key] && enabled} disabled={!enabled} onChange={(e) => setApps({ ...apps, [key]: (e.target as HTMLInputElement).checked })} />
            <strong>{title}</strong>
          </div>
          <p class="muted small" style={{ margin: "0.3rem 0 0 1.6rem" }}>{desc}</p>
        </label>
      ))}
    </div>
  );
}

export function WikipediaEditor({ c, sel, setSel, installed }: { c: Catalog; sel: Selection; setSel: (s: Selection) => void; installed: Record<string, unknown> }) {
  const wiki = c.categories.find((x) => x.id === "wikipedia");
  const items = itemMap(c);
  if (!wiki) return null;
  return (
    <div class="grid cols-2">
      {wiki.options!.map((o) => {
        const size = sizeOf(items, o.items);
        const have = o.items.length > 0 && o.items.every((i) => installed[i]);
        return (
          <label class={`choice ${sel.wikipedia === o.id ? "on" : ""}`} key={o.id}>
            <div class="spread">
              <span>
                <input type="radio" name="wikipedia" checked={sel.wikipedia === o.id} onChange={() => setSel({ ...sel, wikipedia: o.id })} />
                <strong>{o.label}</strong>
              </span>
              <span class="mono small">{o.items.length ? bytes(size) : "—"}</span>
            </div>
            <p class="muted small" style={{ margin: "0.3rem 0 0 1.6rem" }}>
              {o.description} {have && <span class="badge">Installed</span>}
            </p>
          </label>
        );
      })}
    </div>
  );
}

function CategoryCard({ cat, items, tier, setTier, installed }: { cat: Category; items: Map<string, Item>; tier: string; setTier: (t: string) => void; installed: Record<string, unknown> }) {
  const current = tier ? cat.tiers?.[tier] || [] : [];
  return (
    <div class="card">
      <div class="tile-head">
        <span class="icon"><Icon name={cat.icon} /></span>
        <div style={{ flex: 1 }}>
          <h3 style={{ margin: 0 }}>{cat.name}</h3>
          <p class="muted small" style={{ margin: "0.2rem 0 0" }}>{cat.description}</p>
        </div>
      </div>
      <div class="tiers" role="group" aria-label={`${cat.name}: size`}>
        {["", ...TIERS].map((t) => (
          <button type="button" key={t} aria-pressed={tier === t} onClick={() => setTier(t)}>
            {TIER_LABEL[t || "none"]}
            <span>{t ? bytes(sizeOf(items, cat.tiers?.[t] || [])) : "0 B"}</span>
          </button>
        ))}
      </div>
      {current.length > 0 && (
        <details>
          <summary>See the {current.length} title{current.length === 1 ? "" : "s"} in this size</summary>
          <ul class="itemlist">
            {current.map((id) => {
              const it = items.get(id);
              return (
                <li key={id}>
                  <span>
                    {it?.title || id} {installed[id] ? <span class="badge">Installed</span> : null}
                    {it?.notice === "medical" ? <span class="badge plain" title="Medical reference: not a substitute for a clinician">Medical</span> : null}
                  </span>
                  <span class="mono">{bytes(it?.size)}</span>
                </li>
              );
            })}
          </ul>
        </details>
      )}
    </div>
  );
}

export function CollectionsEditor({ c, sel, setSel, installed }: { c: Catalog; sel: Selection; setSel: (s: Selection) => void; installed: Record<string, unknown> }) {
  const items = useMemo(() => itemMap(c), [c]);
  return (
    <div class="grid cols-2">
      {c.categories
        .filter((x) => x.kind === "tiered")
        .map((cat) => (
          <CategoryCard key={cat.id} cat={cat} items={items} installed={installed} tier={sel.tiers[cat.id] || ""} setTier={(t) => setSel({ ...sel, tiers: { ...sel.tiers, [cat.id]: t } })} />
        ))}
    </div>
  );
}

export function RegionsEditor({ c, sel, setSel, installed }: { c: Catalog; sel: Selection; setSel: (s: Selection) => void; installed: Record<string, unknown> }) {
  const [q, setQ] = useState("");
  const regions = useMemo(() => c.items.filter((i) => i.kind === "map-region"), [c]);
  const byId = useMemo(() => itemMap(c), [c]);
  const filtered = regions.filter((r) => !q || r.title.toLowerCase().includes(q.toLowerCase()) || (r.group || "").toLowerCase().includes(q.toLowerCase()));
  const groups = new Map<string, Item[]>();
  for (const r of filtered) {
    const g = r.group || "Other";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(r);
  }
  const order = ["Continents", "United States", "Canada", "Australia", "North America", "Europe", "Asia", "Africa", "South America", "Oceania"];
  const names = [...groups.keys()].sort((a, b) => (order.indexOf(a) + 100) % 100 - (order.indexOf(b) + 100) % 100 || a.localeCompare(b));
  const toggle = (id: string) => setSel({ ...sel, regions: sel.regions.includes(id) ? sel.regions.filter((r) => r !== id) : [...sel.regions, id] });
  const world = c.items.find((i) => i.kind === "map-world");
  return (
    <div class="stack">
      <input type="search" placeholder="Search countries, states, provinces…" value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} aria-label="Search map regions" />
      {sel.regions.length > 0 && (
        <div class="chips">
          {sel.regions.map((id) => (
            <span class="chip" key={id}>
              {byId.get(id)?.title || id}
              <button type="button" aria-label={`Remove ${byId.get(id)?.title}`} onClick={() => toggle(id)}>×</button>
            </span>
          ))}
        </div>
      )}
      <div class="regions">
        {names.map((g) => (
          <div key={g}>
            <h4>{g}</h4>
            {groups.get(g)!.map((r) => (
              <label key={r.id}>
                <span>
                  <input type="checkbox" checked={sel.regions.includes(r.id)} onChange={() => toggle(r.id)} />
                  {r.title} {installed[r.id] ? <span class="badge">Installed</span> : null}
                </span>
                <span class="mono small muted">{bytes(r.size)}</span>
              </label>
            ))}
          </div>
        ))}
        {names.length === 0 && <p class="empty">No region matches “{q}”.</p>}
      </div>
      <p class="hint">
        Each region is cut from the latest OpenStreetMap-based map of the world, down to individual streets. A {bytes(world?.size)} map of the
        whole world always comes along, so you can find your way between regions. Map data © OpenStreetMap contributors.
      </p>
    </div>
  );
}

export function ModelsEditor({ c, sel, setSel, memBytes, installed }: { c: Catalog; sel: Selection; setSel: (s: Selection) => void; memBytes: number; installed: Record<string, unknown> }) {
  const models = c.items.filter((i) => i.kind === "model");
  const memGB = memBytes / 1e9;
  const toggle = (id: string) => setSel({ ...sel, models: sel.models.includes(id) ? sel.models.filter((m) => m !== id) : [...sel.models, id] });
  return (
    <div class="grid cols-2">
      {models.map((m) => {
        const fit = !memGB ? "" : memGB >= (m.ramGB || 0) * 1.25 ? "Runs well here" : memGB >= (m.ramGB || 0) ? "Tight fit" : "Needs more memory";
        const cls = fit === "Runs well here" ? "badge" : fit ? "badge warn" : "badge plain";
        return (
          <label class={`choice ${sel.models.includes(m.id) ? "on" : ""}`} key={m.id}>
            <div class="spread">
              <span>
                <input type="checkbox" checked={sel.models.includes(m.id)} onChange={() => toggle(m.id)} />
                <strong>{m.title}</strong>
              </span>
              <span class="mono small">{bytes(m.size)}</span>
            </div>
            <p class="muted small" style={{ margin: "0.3rem 0 0.3rem 1.6rem" }}>{m.description}</p>
            <div class="row" style={{ marginLeft: "1.6rem" }}>
              {fit && <span class={cls}>{fit}</span>}
              <span class="small muted">needs about {m.ramGB} GB of memory</span>
              {installed[m.id] ? <span class="badge">Installed</span> : null}
            </div>
          </label>
        );
      })}
    </div>
  );
}
