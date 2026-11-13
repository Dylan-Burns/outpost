import type { Metadata } from "next";
import { bytes, loadCatalog } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Content catalog",
  description: "Every archive, map region and AI model Outpost can install, with exact sizes, dates and licences.",
};

const TIERS = ["essential", "standard", "comprehensive"];
const cap = (s: string) => s[0].toUpperCase() + s.slice(1);
/** "2026-10, Wikipedia", leaving out whichever part the publisher did not give. */
const edition = (...parts: (string | undefined)[]) => parts.filter((p) => p && p !== "-").join(", ");

export default function CatalogPage() {
  const c = loadCatalog();
  const items = new Map(c.items.map((i) => [i.id, i]));
  const regions = c.items.filter((i) => i.kind === "map-region");
  const archives = c.items.filter((i) => i.kind === "zim");
  const groups = new Map<string, typeof regions>();
  for (const r of regions) {
    const g = r.group || "Other";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(r);
  }
  const groupOrder = ["Continents", "United States", "Canada", "Australia", "North America", "South America", "Europe", "Africa", "Asia", "Oceania"];
  const models = c.items.filter((i) => i.kind === "model");
  const firstTier = (cat: (typeof c.categories)[number], id: string) => TIERS.find((t) => cat.tiers?.[t]?.includes(id)) || "";
  const sum = (ids: string[] = []) => ids.reduce((n, id) => n + (items.get(id)?.size || 0), 0);
  return (
    <>
      <div className="page-head">
        <div className="wrap">
          <h1>Content catalog</h1>
          <p className="lede">
            Everything you can keep on an Outpost, with the exact size of today&apos;s files. Each subject comes in three sizes that build on
            each other: Standard includes Essential, and Comprehensive includes Standard, unless a larger edition replaces a smaller one.
          </p>
          <dl className="titleblock">
            <div>
              <dt>Generated</dt>
              <dd>
                {c.generated.slice(0, 10)}
                <small>Signed, sequence {c.sequence}</small>
              </dd>
            </div>
            <div>
              <dt>Titles</dt>
              <dd>
                {archives.length}
                <small>{bytes(archives.reduce((n, i) => n + i.size, 0))} in all</small>
              </dd>
            </div>
            <div>
              <dt>Map regions</dt>
              <dd>
                {regions.length}
                <small>Street level, zoom 15</small>
              </dd>
            </div>
            <div>
              <dt>AI models</dt>
              <dd>
                {models.length}
                <small>Optional, through Ollama</small>
              </dd>
            </div>
          </dl>
          <ul className="cat-jump" aria-label="Jump to a category">
            {c.categories.map((cat) => (
              <li key={cat.id}>
                <a href={`#${cat.id}`}>{cat.name}</a>
              </li>
            ))}
            <li>
              <a href="#models">AI models</a>
            </li>
            <li>
              <a href="#maps">Map regions</a>
            </li>
          </ul>
        </div>
      </div>

      {c.categories.map((cat) => (
        <section className="cat-block" key={cat.id} id={cat.id}>
          <div className="wrap">
            <div className="cat-side">
              <div>
                <h2>{cat.name}</h2>
                <p>{cat.description}</p>
                {cat.kind === "tiered" && (
                  <dl className="tier-sizes">
                    {TIERS.map((t) => (
                      <div key={t}>
                        <dt>{cap(t)}</dt>
                        <dd>{bytes(sum(cat.tiers?.[t]))}</dd>
                      </div>
                    ))}
                  </dl>
                )}
              </div>
            </div>
            {cat.kind === "single" ? (
              <table className="cat-items">
                <thead>
                  <tr>
                    <th scope="col">Option</th>
                    <th scope="col">Title</th>
                    <th scope="col">Licence</th>
                    <th scope="col" style={{ textAlign: "right" }}>Size</th>
                  </tr>
                </thead>
                <tbody>
                  {cat.options!.filter((o) => o.items.length).map((o) => {
                    const it = items.get(o.items[0])!;
                    return (
                      <tr key={o.id}>
                        <td className="tier-tag">{o.label}</td>
                        <td>
                          {it.title} {edition(it.date) && <span className="quiet">({edition(it.date)})</span>}
                          <br />
                          <span className="quiet">{o.description}</span>
                        </td>
                        <td className="quiet">{it.license}</td>
                        <td className="num">{bytes(it.size)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            ) : (
              <table className="cat-items">
                <thead>
                  <tr>
                    <th scope="col">From</th>
                    <th scope="col">Title</th>
                    <th scope="col">Licence</th>
                    <th scope="col" style={{ textAlign: "right" }}>Size</th>
                  </tr>
                </thead>
                <tbody>
                  {[...new Set(TIERS.flatMap((t) => cat.tiers?.[t] || []))].map((id) => {
                    const it = items.get(id)!;
                    return (
                      <tr key={id}>
                        <td className="tier-tag">{cap(firstTier(cat, id))}</td>
                        <td>
                          {it.title} {edition(it.date, it.publisher) && <span className="quiet">({edition(it.date, it.publisher)})</span>}
                          {it.description && (
                            <>
                              <br />
                              <span className="quiet">{it.description}</span>
                            </>
                          )}
                        </td>
                        <td className="quiet">{it.license}</td>
                        <td className="num">{bytes(it.size)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </section>
      ))}

      <section className="cat-block" id="models">
        <div className="wrap">
          <div className="cat-side">
            <div>
              <h2>AI models</h2>
              <p>Installed through Ollama when the assistant is enabled. Memory is what the model needs to run comfortably.</p>
            </div>
          </div>
          <table className="cat-items">
            <thead>
              <tr>
                <th scope="col">Model</th>
                <th scope="col">About</th>
                <th scope="col">Memory</th>
                <th scope="col" style={{ textAlign: "right" }}>Size</th>
              </tr>
            </thead>
            <tbody>
              {models.map((m) => (
                <tr key={m.id}>
                  <td className="tier-tag">{m.title}</td>
                  <td>
                    {m.description} <code className="quiet">{m.model}</code>
                  </td>
                  <td className="quiet">{m.ramGB} GB</td>
                  <td className="num">{bytes(m.size)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="cat-block" id="maps">
        <div className="wrap">
          <div className="cat-side">
            <div>
              <h2>Map regions</h2>
              <p>
                {regions.length} regions at street-level detail (zoom 15), cut from the Protomaps planet build of{" "}
                {c.mapBuild.split("/").pop()?.slice(0, 8)}. Map data © OpenStreetMap contributors, ODbL. A{" "}
                {bytes(items.get("map-world")?.size || 0)} world overview and {bytes(items.get("map-assets")?.size || 0)} of fonts and icons come
                with the first region.
              </p>
            </div>
          </div>
          <div className="region-groups">
            {[...groups.keys()]
              .sort((a, b) => ((groupOrder.indexOf(a) + 99) % 99) - ((groupOrder.indexOf(b) + 99) % 99) || a.localeCompare(b))
              .map((g) => (
                <div key={g}>
                  <h3>{g}</h3>
                  {groups
                    .get(g)!
                    .sort((a, b) => a.title.localeCompare(b.title))
                    .map((r) => (
                      <p key={r.id}>
                        {r.title} <span>{bytes(r.size)}</span>
                      </p>
                    ))}
                </div>
              ))}
          </div>
        </div>
      </section>
    </>
  );
}
