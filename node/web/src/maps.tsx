import { useEffect, useRef, useState } from "preact/hooks";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Protocol } from "pmtiles";
import { layers, namedFlavor } from "@protomaps/basemaps";
import { api, type Status } from "./api";
import { Link } from "./router";

type Region = { id: string; name: string; group: string; bbox: number[]; file: string };
type MapsInfo = { regions: Region[]; world: string; assets: boolean };

let protocolAdded = false;

function buildStyle(info: MapsInfo, region: Region | null, dark: boolean): maplibregl.StyleSpecification {
  const origin = location.origin;
  const flavor = namedFlavor(dark ? "dark" : "light");
  const sources: maplibregl.StyleSpecification["sources"] = {};
  const all: maplibregl.LayerSpecification[] = [];
  const add = (name: string, file: string, maxLabelZoom?: number) => {
    sources[name] = {
      type: "vector",
      url: `pmtiles://${origin}/maps/files/${file}`,
      attribution: '<a href="https://openstreetmap.org/copyright">© OpenStreetMap contributors</a>, <a href="https://protomaps.com">Protomaps</a>',
    };
    for (const l of layers(name, flavor, { lang: "en" }) as maplibregl.LayerSpecification[]) {
      if (all.length > 0 && l.type === "background") continue;
      const copy: any = { ...l, id: `${name}-${l.id}` };
      if (maxLabelZoom !== undefined && l.type === "symbol") copy.maxzoom = maxLabelZoom;
      all.push(copy);
    }
  };
  if (info.world) add("world", info.world, region ? 7 : undefined);
  if (region) add("region", region.file);
  return {
    version: 8,
    glyphs: `${origin}/maps/assets/fonts/{fontstack}/{range}.pbf`,
    sprite: `${origin}/maps/assets/sprites/v4/${dark ? "dark" : "light"}`,
    sources,
    layers: all,
  };
}

export default function Maps({ status }: { status: Status }) {
  const el = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const [info, setInfo] = useState<MapsInfo | null>(null);
  const [err, setErr] = useState("");
  const [current, setCurrent] = useState<Region | null>(null);
  const dark = matchMedia("(prefers-color-scheme: dark)").matches;

  useEffect(() => {
    api.get<MapsInfo>("/api/maps").then((d) => {
      setInfo(d);
      setCurrent(d.regions[0] || null);
    }, (e) => setErr(e.message));
  }, []);

  useEffect(() => {
    if (!info || !el.current || (!info.world && !info.regions.length) || !info.assets) return;
    if (!protocolAdded) {
      const protocol = new Protocol();
      maplibregl.addProtocol("pmtiles", protocol.tile);
      protocolAdded = true;
    }
    const style = buildStyle(info, current, dark);
    if (!map.current) {
      map.current = new maplibregl.Map({
        container: el.current,
        style,
        center: [0, 20],
        zoom: 1.5,
        maxPitch: 60,
        attributionControl: { compact: true },
      });
      map.current.addControl(new maplibregl.NavigationControl(), "top-right");
      map.current.addControl(new maplibregl.ScaleControl({}), "bottom-left");
      map.current.on("error", (e) => console.warn("map:", e.error?.message));
    } else {
      map.current.setStyle(style);
    }
    if (current?.bbox?.length === 4) {
      const [w, s, e, n] = current.bbox;
      map.current.fitBounds([[w, s], [e, n]], { padding: 30, duration: 800 });
    }
  }, [info, current]);

  useEffect(() => () => map.current?.remove(), []);

  if (err) return <p class="alert warn">{err}</p>;
  if (!info) return <p class="empty"><span class="spinner" /></p>;
  if (!info.regions.length && !info.world)
    return (
      <div class="card empty">
        <h1>Maps</h1>
        <p>No map regions on this node yet.</p>
        {status.signedIn ? <Link class="btn primary" href="/content">Choose regions</Link> : <p class="small">The administrator can add them.</p>}
      </div>
    );
  if (!info.assets) return <p class="alert">Map data is still downloading. Come back when the map fonts and icons have arrived.</p>;
  return (
    <div class="mapwrap">
      <div class="side">
        <h1 style={{ fontSize: "1.3rem" }}>Maps</h1>
        {info.regions.map((r) => (
          <button key={r.id} class={`btn small ${current?.id === r.id ? "primary" : ""}`} onClick={() => setCurrent(r)}>
            <span>{r.name}</span>
            <span class="small" style={{ opacity: 0.7 }}>{r.group}</span>
          </button>
        ))}
        {info.world && (
          <button class={`btn small ${!current ? "primary" : ""}`} onClick={() => setCurrent(null)}>
            <span>World overview</span>
          </button>
        )}
        <p class="hint">Maps work fully offline. Data © OpenStreetMap contributors (ODbL), styled by Protomaps.</p>
      </div>
      <div id="map" ref={el} role="region" aria-label="Map" />
    </div>
  );
}
