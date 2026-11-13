import type { Metadata } from "next";
import { bytes, loadCatalog } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Content catalog",
  description: "Every archive, map region and AI model Outpost can install, with exact sizes, dates and licences.",
};

const TIERS = ["essential", "standard", "comprehensive"];

export default function CatalogPage() {
  const c = loadCatalog();
  const items = new Map(c.items.map((i) => [i.id, i]));
  const regions = c.items.filter((i) => i.kind === "map-region");
  const groups = new Map<string, typeof regions>();
  for (const r of regions) {
    const g = r.group || "Other";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(r);
  }
  const groupOrder = ["Continents", "United States", "Canada", "Australia", "North America", "South America", "Europe", "Africa", "Asia", "Oceania"];
  const models = c.items.filter((i) => i.kind === "model");
  const firstTier = (cat: (typeof c.categories)[number], id: string) => TIERS.find((t) => cat.tiers?.[t]?.includes(id)) || "";
  return (
    <div className="wrap">
      <div className="page-head">
        <h1 style={{ fontSize: "clamp(2.6rem, 6vw, 4.4rem)" }}>Content catalog</h1>
        <p className="lede">
          Everything a node can install, with the exact size of today&apos;s files. Catalog generated {c.generated.slice(0, 10)}. Tiers add up: Standard
          includes Essential, Comprehensive includes Standard, unless a tier replaces smaller editions with a larger one.
        </p>
      </div>

      {c.categories.map((cat) => (
        <div className="cat-block" key={cat.id} id={cat.id}>
          <h2>{cat.name}</h2>
          <p className="quiet">{cat.description}</p>
          {cat.kind === "single" ? (
            <table className="cat-items">
              <thead><tr><th>Option</th><th>Archive</th><th>Licence</th><th style={{ textAlign: "right" }}>Size</th></tr></thead>
              <tbody>
                {cat.options!.filter((o) => o.items.length).map((o) => {
                  const it = items.get(o.items[0])!;
                  return (
                    <tr key={o.id}>
                      <td className="tier-tag">{o.label}</td>
                      <td>{it.title} <span className="quiet">({it.date})</span><br /><span className="quiet">{o.description}</span></td>
                      <td>{it.license}</td>
                      <td className="num">{bytes(it.size)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : (
            <>
              <p>
                {TIERS.map((t) => (
                  <span key={t} style={{ marginRight: "1.5rem" }}>
                    <span className="tier-tag">{t[0].toUpperCase() + t.slice(1)}</span>{" "}
                    <code>{bytes((cat.tiers?.[t] || []).reduce((n, id) => n + (items.get(id)?.size || 0), 0))}</code>
                  </span>
                ))}
              </p>
              <table className="cat-items">
                <thead><tr><th>From</th><th>Archive</th><th>Licence</th><th style={{ textAlign: "right" }}>Size</th></tr></thead>
                <tbody>
                  {[...new Set(TIERS.flatMap((t) => cat.tiers?.[t] || []))].map((id) => {
                    const it = items.get(id)!;
                    const t = firstTier(cat, id);
                    return (
                      <tr key={id}>
                        <td className="tier-tag">{t[0].toUpperCase() + t.slice(1)}</td>
                        <td>
                          {it.title} <span className="quiet">({it.date}{it.publisher ? `, ${it.publisher}` : ""})</span>
                          {it.description && <><br /><span className="quiet">{it.description}</span></>}
                        </td>
                        <td>{it.license}</td>
                        <td className="num">{bytes(it.size)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </>
          )}
        </div>
      ))}

      <div className="cat-block" id="models">
        <h2>AI models</h2>
        <p className="quiet">Installed through Ollama when the assistant is enabled. Memory is what the model needs to run comfortably.</p>
        <table className="cat-items">
          <thead><tr><th>Model</th><th>About</th><th>Memory</th><th style={{ textAlign: "right" }}>Size</th></tr></thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.id}><td className="tier-tag">{m.title}</td><td>{m.description} <code className="quiet">{m.model}</code></td><td>{m.ramGB} GB</td><td className="num">{bytes(m.size)}</td></tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="cat-block" id="maps">
        <h2>Map regions</h2>
        <p className="quiet">
          {regions.length} regions at street-level detail (zoom 15), cut from the Protomaps planet build of {c.mapBuild.split("/").pop()?.slice(0, 8)}. Map data ©
          OpenStreetMap contributors, ODbL. A {bytes(items.get("map-world")?.size || 0)} world overview and {bytes(items.get("map-assets")?.size || 0)} of
          fonts and icons come with the first region.
        </p>
        <div className="region-groups">
          {[...groups.keys()].sort((a, b) => ((groupOrder.indexOf(a) + 99) % 99) - ((groupOrder.indexOf(b) + 99) % 99) || a.localeCompare(b)).map((g) => (
            <div key={g}>
              <h3>{g}</h3>
              {groups.get(g)!.sort((a, b) => a.title.localeCompare(b.title)).map((r) => (
                <p key={r.id}>{r.title} <span>{bytes(r.size)}</span></p>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
