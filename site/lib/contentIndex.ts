import { readFileSync } from "node:fs";
import path from "node:path";

/** Where the index's trees, leaves and search blocks are served from. The
 *  small root.json ships with the site; the rest (hundreds of MB) can live
 *  elsewhere, so a site deploy never has to carry it. */
export const INDEX_BASE = process.env.NEXT_PUBLIC_INDEX_BASE || "/catalog/v1/index";

export type Collection = {
  id: string;
  title: string;
  purpose: string;
  description?: string;
  longDescription?: string;
  edition?: string;
  built?: string;
  size: number;
  items: number;
  itemBytes: number;
  unmeasured: number;
  kinds: Record<string, number>;
  other: Record<string, number>;
  license?: string;
  publisher?: string;
  creator?: string;
  file?: string;
  sha256?: string;
  entries?: number;
  measured: boolean;
  notice?: string;
  kind: "zim" | "maps" | "models";
  tier?: string;
  alsoIn?: string[];
  /** Editions of one book share one tree: its id, and this edition's column in it. */
  tree?: string;
  ed?: number;
  editions?: string[];
};

export type Subject = {
  id: string;
  name: string;
  description: string;
  purpose: string;
  kind: string;
  sizes?: Record<string, number>;
  collections: Collection[];
};

export type Totals = {
  downloads: number;
  archives: number;
  mapRegions: number;
  models: number;
  items: number;
  unmeasured: number;
  everyFile: number;
  archiveBytes: number;
  mapBytes: number;
  modelBytes: number;
  largestLibrary: number;
  largestLibraryRule: string;
  largestWithMaps: number;
  largestWithEverything: number;
};

export type IndexRoot = {
  schema: number;
  generated: string;
  /** Where this build's trees, leaves and search blocks are served. */
  base?: string;
  catalogSequence: number;
  limit: number;
  collections: string[];
  totals: Totals;
  subjects: Subject[];
};

export function loadIndexRoot(): IndexRoot | null {
  try {
    const p = path.join(process.cwd(), "public", "catalog", "v1", "index", "root.json");
    return JSON.parse(readFileSync(p, "utf8")) as IndexRoot;
  } catch {
    return null;
  }
}
