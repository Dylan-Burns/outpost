"use client";
import { useMemo, useState } from "react";
import type { PlannerData } from "@/lib/catalog";

const TIERS = ["", "essential", "standard", "comprehensive"];
const TIER_LABEL: Record<string, string> = { "": "None", essential: "Essential", standard: "Standard", comprehensive: "Comprehensive" };

function bytes(n: number) {
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1000 && i < u.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${u[i]}`;
}

function driveFor(n: number) {
  const sizes = [128e9, 256e9, 512e9, 1e12, 2e12, 4e12, 8e12];
  const need = n * 1.25 + 20e9; // room for the system and growth
  const d = sizes.find((s) => s >= need);
  return d ? (d >= 1e12 ? `${d / 1e12} TB` : `${d / 1e9} GB`) : "more than 8 TB";
}

export default function Planner({ data }: { data: PlannerData }) {
  const wiki = data.categories.find((c) => c.id === "wikipedia")!;
  const tiered = data.categories.filter((c) => c.kind === "tiered");
  const [wikiOpt, setWikiOpt] = useState(wiki.default || "top-maxi");
  const [tiers, setTiers] = useState<Record<string, string>>({ medicine: "essential", survival: "essential", repair: "essential" });
  const [regions, setRegions] = useState<string[]>([]);
  const [models, setModels] = useState<string[]>([]);
  const [q, setQ] = useState("");

  const regionList = useMemo(() => Object.entries(data.items).filter(([, v]) => v.k === "map-region"), [data]);
  const modelList = useMemo(() => Object.entries(data.items).filter(([, v]) => v.k === "model"), [data]);
  const matches = q.trim().length < 2 ? [] : regionList.filter(([, v]) => v.t.toLowerCase().includes(q.toLowerCase())).slice(0, 8);

  const total = useMemo(() => {
    const want = new Set<string>();
    wiki.options?.find((o) => o.id === wikiOpt)?.items.forEach((i) => want.add(i));
    for (const c of tiered) (tiers[c.id] ? c.tiers?.[tiers[c.id]] || [] : []).forEach((i) => want.add(i));
    if (regions.length) {
      regions.forEach((r) => want.add(r));
      for (const [id, v] of Object.entries(data.items)) if (v.k === "map-world" || v.k === "map-assets") want.add(id);
    }
    models.forEach((m) => want.add(m));
    let n = 0;
    want.forEach((id) => (n += data.items[id]?.s || 0));
    return { n, count: want.size };
  }, [wikiOpt, tiers, regions, models, data, tiered, wiki]);

  const sizeOf = (ids: string[] = []) => ids.reduce((n, id) => n + (data.items[id]?.s || 0), 0);

  return (
    <div className="planner">
      <div className="planner-main">
        <fieldset className="wiki">
          <legend>Wikipedia</legend>
          <div className="wiki-options">
            {wiki.options?.map((o) => (
              <label key={o.id} className={wikiOpt === o.id ? "on" : ""}>
                <input type="radio" name="wiki" checked={wikiOpt === o.id} onChange={() => setWikiOpt(o.id)} />
                <span className="name">{o.label}</span>
                <span className="size">{o.items.length ? bytes(sizeOf(o.items)) : "0"}</span>
              </label>
            ))}
          </div>
        </fieldset>

        <div className="collections">
          {tiered.map((c) => (
            <div className="collection" key={c.id}>
              <div className="collection-head">
                <h3>{c.name}</h3>
                <p>{c.description}</p>
              </div>
              <div className="tierbar" role="radiogroup" aria-label={`${c.name}: amount`}>
                {TIERS.map((t) => (
                  <button
                    type="button"
                    role="radio"
                    aria-checked={(tiers[c.id] || "") === t}
                    key={t}
                    onClick={() => setTiers({ ...tiers, [c.id]: t })}
                  >
                    {TIER_LABEL[t]}
                    <small>{t ? bytes(sizeOf(c.tiers?.[t])) : "—"}</small>
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>

        <div className="planner-extras">
          <div>
            <h3>Map regions</h3>
            <input type="search" placeholder="Search a country, state or province" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search map regions" />
            {matches.length > 0 && (
              <ul className="matches">
                {matches.map(([id, v]) => (
                  <li key={id}>
                    <button type="button" onClick={() => { if (!regions.includes(id)) setRegions([...regions, id]); setQ(""); }}>
                      {v.t} <span>{v.g}</span> <b>{bytes(v.s)}</b>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <div className="picked">
              {regions.map((id) => (
                <button type="button" key={id} onClick={() => setRegions(regions.filter((r) => r !== id))} aria-label={`Remove ${data.items[id].t}`}>
                  {data.items[id].t} ×
                </button>
              ))}
              {regions.length === 0 && <span className="quiet">None yet. {regionList.length} countries, states and provinces available.</span>}
            </div>
          </div>
          <div>
            <h3>AI models</h3>
            <div className="models">
              {modelList.map(([id, v]) => (
                <label key={id}>
                  <input type="checkbox" checked={models.includes(id)} onChange={() => setModels(models.includes(id) ? models.filter((m) => m !== id) : [...models, id])} />
                  {v.t} <span>{bytes(v.s)} · {v.r} GB RAM</span>
                </label>
              ))}
            </div>
          </div>
        </div>
      </div>
      <aside className="planner-total" aria-live="polite">
        <span className="label">Your library</span>
        <strong>{bytes(total.n)}</strong>
        <span>{total.count} downloads</span>
        <hr />
        <span className="label">Suggested drive</span>
        <strong className="drive">{driveFor(total.n)}</strong>
        <p>Sizes are the real files in today&apos;s catalog. The setup wizard on your machine shows the same choices.</p>
      </aside>
    </div>
  );
}
