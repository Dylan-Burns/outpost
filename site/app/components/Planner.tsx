"use client";
import { useMemo, useState } from "react";
import type { PlannerData } from "@/lib/catalog";

const TIERS = ["", "essential", "standard", "comprehensive"];
const TIER_LABEL: Record<string, string> = { "": "None", essential: "Essential", standard: "Standard", comprehensive: "Comprehensive" };
const SEGMENTS = 24;

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

/** The smallest common drive that holds the selection with room for the system and growth. */
function driveFor(n: number) {
  const sizes = [128e9, 256e9, 512e9, 1e12, 2e12, 4e12, 8e12];
  const need = n * 1.25 + 20e9;
  const d = sizes.find((s) => s >= need);
  if (!d) return { label: "8 TB +", used: 1 };
  return { label: d >= 1e12 ? `${d / 1e12} TB` : `${d / 1e9} GB`, used: n / d };
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
  const drive = driveFor(total.n);
  const lit = total.n > 0 ? Math.max(1, Math.round(drive.used * SEGMENTS)) : 0;
  const percent = Math.round(drive.used * 100);

  return (
    <div className="planner">
      <div className="planner-main">
        <div className="pl-row" role="group" aria-labelledby="pl-wiki">
          <div className="collection-head">
            <h3 id="pl-wiki">Wikipedia</h3>
            <p>{wiki.description}</p>
          </div>
          <div className="wiki-options">
            {wiki.options?.map((o) => (
              <label key={o.id} className={wikiOpt === o.id ? "on" : ""}>
                <input type="radio" name="wiki" checked={wikiOpt === o.id} onChange={() => setWikiOpt(o.id)} />
                <span className="name">{o.label}</span>
                <span className="size">{o.items.length ? bytes(sizeOf(o.items)) : "0 GB"}</span>
              </label>
            ))}
          </div>
        </div>

        {tiered.map((c) => (
          <div className="pl-row" key={c.id}>
            <div className="collection-head">
              <h3>{c.name}</h3>
              <p>{c.description}</p>
            </div>
            <div className="tierbar" role="radiogroup" aria-label={`${c.name}: amount`}>
              {TIERS.map((t) => (
                <button type="button" role="radio" aria-checked={(tiers[c.id] || "") === t} key={t} onClick={() => setTiers({ ...tiers, [c.id]: t })}>
                  {TIER_LABEL[t]}
                  <small>{t ? bytes(sizeOf(c.tiers?.[t])) : "0 GB"}</small>
                </button>
              ))}
            </div>
          </div>
        ))}

        <div className="pl-row">
          <div className="collection-head">
            <h3>Map regions</h3>
            <p>{regionList.length} countries, states and provinces at street level.</p>
          </div>
          <div className="pl-maps">
            <input
              type="search"
              placeholder="Search a country, state or province"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              aria-label="Search map regions"
            />
            {matches.length > 0 && (
              <ul className="matches">
                {matches.map(([id, v]) => (
                  <li key={id}>
                    <button
                      type="button"
                      onClick={() => {
                        if (!regions.includes(id)) setRegions([...regions, id]);
                        setQ("");
                      }}
                    >
                      {v.t} <span>{v.g}</span> <b>{bytes(v.s)}</b>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {regions.length > 0 && (
              <div className="picked">
                {regions.map((id) => (
                  <button type="button" key={id} onClick={() => setRegions(regions.filter((r) => r !== id))} aria-label={`Remove ${data.items[id].t}`}>
                    {data.items[id].t} <span aria-hidden="true">×</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="pl-row">
          <div className="collection-head">
            <h3>AI models</h3>
            <p>Optional. Each runs on your own computer; bigger ones need more memory.</p>
          </div>
          <div className="models">
            {modelList.map(([id, v]) => (
              <label key={id} className={models.includes(id) ? "on" : ""}>
                <input
                  type="checkbox"
                  checked={models.includes(id)}
                  onChange={() => setModels(models.includes(id) ? models.filter((m) => m !== id) : [...models, id])}
                />
                <span className="name">{v.t}</span>
                <span className="size">
                  {`${bytes(v.s)} // ${v.r} GB RAM`}
                </span>
              </label>
            ))}
          </div>
        </div>
      </div>

      <aside className="planner-total ink" aria-live="polite">
        <div className="sticky">
          <div className="tot">
            <span className="label">Your library</span>
            <strong>{bytes(total.n)}</strong>
            <span className="count">
              {total.count} {total.count === 1 ? "download" : "downloads"}
            </span>
          </div>
          <div className="tot">
            <span className="label">Suggested drive</span>
            <strong>{drive.label}</strong>
            <span className="count">{percent}% full with this selection</span>
          </div>
          <div className="meter" role="img" aria-label={`This selection fills ${percent}% of that drive`}>
            {Array.from({ length: SEGMENTS }, (_, i) => (
              <i key={i} className={i < lit ? "on" : ""} />
            ))}
          </div>
          <p>These are the real file sizes in today&apos;s catalog. Setup on your own computer offers the same choices.</p>
        </div>
      </aside>
    </div>
  );
}
