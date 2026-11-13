import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";

/**
 * The hero globe is drawn from the same region outlines a node uses to cut
 * its maps, so the picture is the catalog. This runs at build time: it reads
 * every outline, drops detail a few hundred pixels cannot show, and packs the
 * rings as small integers.
 */

type Ring = [number, number][];

/** Units per degree in the packed output. 20 means 0.05 degree steps. */
export const GLOBE_SCALE = 20;

const TOLERANCE = 0.16; // degrees; under a pixel on a 700px globe
const MIN_SPAN = 0.55; // rings smaller than this in both directions are dust

function simplify(ring: Ring, tol: number): Ring {
  const n = ring.length;
  if (n < 5) return ring;
  const keep = new Uint8Array(n);
  keep[0] = keep[n - 1] = 1;
  const stack: [number, number][] = [[0, n - 1]];
  while (stack.length) {
    const [a, b] = stack.pop()!;
    const [ax, ay] = ring[a];
    const [bx, by] = ring[b];
    // Longitude shrinks with latitude; measure in roughly equal units.
    const k = Math.cos(((ay + by) / 2) * (Math.PI / 180));
    const dx = (bx - ax) * k;
    const dy = by - ay;
    const len = Math.hypot(dx, dy) || 1e-9;
    let worst = 0;
    let at = -1;
    for (let i = a + 1; i < b; i++) {
      const px = (ring[i][0] - ax) * k;
      const py = ring[i][1] - ay;
      const d = len < 1e-8 ? Math.hypot(px, py) : Math.abs(px * dy - py * dx) / len;
      if (d > worst) {
        worst = d;
        at = i;
      }
    }
    if (worst > tol && at > 0) {
      keep[at] = 1;
      stack.push([a, at], [at, b]);
    }
  }
  return ring.filter((_, i) => keep[i]);
}

function ringsOf(file: string): Ring[] {
  const g = JSON.parse(readFileSync(file, "utf8"));
  const out: Ring[] = [];
  for (const f of g.features || []) {
    const geo = f.geometry;
    if (!geo) continue;
    const polys = geo.type === "MultiPolygon" ? geo.coordinates : geo.type === "Polygon" ? [geo.coordinates] : [];
    for (const poly of polys) if (poly[0]) out.push(poly[0] as Ring); // outer boundary only
  }
  return out;
}

export type GlobeData = { scale: number; regions: number; rings: number[][] };

let cached: GlobeData | null = null;
export function globeData(): GlobeData {
  if (cached) return cached;
  const dir = path.join(process.cwd(), "public", "catalog", "v1", "regions");
  // Countries only. Continents repeat them, and states inside a country turn
  // it into a mesh that the headline cannot be read through.
  const files = readdirSync(dir).filter((f) => /^map-[a-z]{3}\.geojson$/.test(f)).sort();
  const rings: number[][] = [];
  for (const f of files) {
    for (const ring of ringsOf(path.join(dir, f))) {
      let minX = 180, maxX = -180, minY = 90, maxY = -90;
      for (const [x, y] of ring) {
        if (x < minX) minX = x;
        if (x > maxX) maxX = x;
        if (y < minY) minY = y;
        if (y > maxY) maxY = y;
      }
      if (maxX - minX < MIN_SPAN && maxY - minY < MIN_SPAN) continue;
      const s = simplify(ring, TOLERANCE);
      if (s.length < 4) continue;
      // First point absolute, the rest as steps from the one before.
      const packed: number[] = [];
      let px = 0, py = 0;
      for (const [x, y] of s) {
        const qx = Math.round(x * GLOBE_SCALE);
        const qy = Math.round(y * GLOBE_SCALE);
        if (packed.length && qx === px && qy === py) continue;
        packed.push(qx - px, qy - py);
        px = qx;
        py = qy;
      }
      if (packed.length >= 8) rings.push(packed);
    }
  }
  cached = { scale: GLOBE_SCALE, regions: files.length, rings };
  return cached;
}
