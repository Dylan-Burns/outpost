"use client";
/* The content index: one table of drop-downs, subject › collection › sections
 * › item. Subjects and collections come with the page; a collection's section
 * tree is fetched when it is opened, and a shelf's rows (at most 200) when that
 * shelf is opened, each by one byte-range read of a packed file. */
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import type { Collection, IndexRoot, Subject } from "@/lib/contentIndex";

type TreeNode = { l: string; n: number; s: number; ss?: number[]; u?: number; cr?: [number, number, number]; d?: string; c?: TreeNode[]; r?: [number, number, number] };
type Row = {
  t: string;
  k: string;
  s?: number;
  pg?: number;
  min?: number;
  w?: number;
  d?: string;
  by?: string;
  x?: string;
  ans?: number;
  score?: number;
  asked?: string;
  tags?: string;
  steps?: number;
  time?: string;
  diff?: string;
  fmt?: string;
  files?: number;
  gid?: number;
  ram?: number;
  ollama?: string;
  zoom?: number;
  bbox?: number[];
  rank?: number;
  qs?: number;
  ss?: (number | null)[];
  /** Path inside the archive, when it is not simply the title with underscores. */
  u?: string;
};
type Ref = [number, string, number];
type Hit = [string, Ref[]];

// --- formatting ----------------------------------------------------------------

function bytes(n: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}
const num = (n: number) => n.toLocaleString("en-US");
const plural = (n: number, one: string, many = one + "s") => `${num(n)} ${n === 1 ? one : many}`;

function minutes(m: number) {
  if (m < 1) return `${Math.max(1, Math.round(m * 60))} s`;
  if (m < 60) return `${Math.round(m)} min`;
  return `${Math.floor(m / 60)} h ${Math.round(m % 60)} min`;
}

function length(r: Row): string {
  if (r.pg) return plural(r.pg, "page");
  if (r.min) return minutes(r.min);
  if (r.steps) return plural(r.steps, "step");
  if (r.qs) return plural(r.qs, "question");
  if (r.k === "question" && r.ans !== undefined) return plural(r.ans, "answer");
  if (r.w) return plural(r.w, "word");
  if (r.ram) return `${r.ram} GB memory`;
  if (r.zoom) return `zoom ${r.zoom}`;
  return "";
}

function sizeText(r: Row, ed: number) {
  const s = r.ss ? r.ss[ed] : r.s;
  if (r.ss && s === null) return "not in this edition";
  return s === undefined || s === null || s < 0 ? "not yet measured" : bytes(s);
}
const nodeSize = (n: TreeNode, ed: number) => (n.ss ? n.ss[ed] : n.s);

// --- links to the items themselves --------------------------------------------

const KIWIX = "https://browse.library.kiwix.org/content";
const OUTPOST_KEY = "outpost-address";

/** An Outpost serves its library with kiwix-serve, addressing each archive by
 *  its file name without the _YYYY-MM date (see node/internal/server/api.go). */
function undated(zim: string) {
  return zim.replace(/_\d{4}-\d{2}$/, "");
}

function encPath(p: string) {
  return p.split("/").map(encodeURIComponent).join("/");
}

/** The library address of an Outpost, from the address its dashboard is opened
 *  at: same machine, the library port (8081 unless the owner changed it). */
export function libraryOf(addr: string): string | null {
  let a = addr.trim();
  if (!a) return null;
  if (!/^https?:\/\//i.test(a)) a = "http://" + a;
  try {
    const u = new URL(a);
    const port = u.port === "" || u.port === "8080" ? "8081" : u.port;
    return `${u.protocol}//${u.hostname}:${port}`;
  } catch {
    return null;
  }
}

const OutpostCtx = createContext<string | null>(null);

function itemPath(r: Row) {
  return r.u ?? r.t.replace(/ /g, "_");
}

function OpenLinks({ zim, path }: { zim?: string; path: string }) {
  const lib = useContext(OutpostCtx);
  if (!zim) return null;
  return (
    <span className="ix-open">
      <a href={`${KIWIX}/${zim}/${encPath(path)}`} target="_blank" rel="noopener noreferrer">
        Preview online
      </a>
      {lib ? (
        <a href={`${lib}/content/${undated(zim)}/${encPath(path)}`} target="_blank" rel="noopener">
          Open on my Outpost
        </a>
      ) : (
        <a href="#outpost-address" className="quiet">
          Open on my Outpost: set its address above
        </a>
      )}
    </span>
  );
}

function OutpostAddress({ value, set }: { value: string; set: (v: string) => void }) {
  const [draft, setDraft] = useState(value);
  const lib = libraryOf(draft);
  return (
    <form
      className="ix-outpost"
      onSubmit={(e) => {
        e.preventDefault();
        set(draft.trim());
      }}
    >
      <label htmlFor="outpost-address" className="label">
        Have an Outpost? Its address, as you open its dashboard
      </label>
      <div>
        <input id="outpost-address" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="e.g. 192.168.1.20:8080 or mymac.local:8080" autoComplete="off" />
        <button type="submit">Save</button>
      </div>
      <small className="quiet">
        {value && lib
          ? `Items open from ${libraryOf(value)} on your own network. Kept in this browser only.`
          : "Then each item can open from your own Outpost, with no internet. Kept in this browser only."}{" "}
        Without one, “Preview online” opens the same file in Kiwix&apos;s online library.
      </small>
    </form>
  );
}

// --- data access ---------------------------------------------------------------

function norm(s: string) {
  return s.normalize("NFKD").replace(/\p{M}/gu, "").toLowerCase().replace(/ß/g, "ss").replace(/\s+/g, " ").trim();
}

const cache = new Map<string, Promise<unknown>>();

async function gunzipJson(buf: ArrayBuffer) {
  const stream = new Blob([buf]).stream().pipeThrough(new DecompressionStream("gzip"));
  return JSON.parse(await new Response(stream).text());
}

function fetchJson<T>(url: string): Promise<T> {
  if (!cache.has(url)) {
    const p = fetch(url).then((r) => {
      if (!r.ok) throw new Error(`${r.status} ${url}`);
      return r.json();
    });
    p.catch(() => cache.delete(url));
    cache.set(url, p);
  }
  return cache.get(url) as Promise<T>;
}

function fetchPacked<T>(url: string, off: number, len: number): Promise<T> {
  const key = `${url}#${off}`;
  if (!cache.has(key)) {
    const p = fetch(url, { headers: { Range: `bytes=${off}-${off + len - 1}` } })
      .then(async (r) => {
        if (!r.ok) throw new Error(`${r.status} ${url}`);
        let buf = await r.arrayBuffer();
        if (r.status === 200) buf = buf.slice(off, off + len); // the server ignored the range
        return gunzipJson(buf);
      });
    p.catch(() => cache.delete(key));
    cache.set(key, p);
  }
  return cache.get(key) as Promise<T>;
}

// --- the table -----------------------------------------------------------------

export default function ContentIndex({ root, base }: { root: IndexRoot; base: string }) {
  const [target, setTarget] = useState<{ col: string; path: number[]; row: number } | null>(null);
  const [outpost, setOutpost] = useState("");
  useEffect(() => {
    let v = "";
    try {
      v = localStorage.getItem(OUTPOST_KEY) || "";
    } catch {
      v = "";
    }
    // Read once after mount: the page is rendered on the server without it.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (v) setOutpost(v);
  }, []);
  const saveOutpost = useCallback((v: string) => {
    setOutpost(v);
    try {
      if (v) localStorage.setItem(OUTPOST_KEY, v);
      else localStorage.removeItem(OUTPOST_KEY);
    } catch {
      /* private mode: kept for this visit only */
    }
  }, []);
  const colIndex = useMemo(() => new Map(root.collections.map((c, i) => [c, i])), [root]);
  const subjectsOf = useMemo(() => {
    const m = new Map<string, Subject[]>();
    for (const s of root.subjects) for (const c of s.collections) m.set(c.id, [...(m.get(c.id) || []), s]);
    return m;
  }, [root]);

  // Deep links: #/collection/0.3.12/57
  useEffect(() => {
    const read = () => {
      const m = location.hash.match(/^#\/([^/]+)(?:\/([\d.]*))?(?:\/(\d+))?$/);
      if (!m || !colIndex.has(decodeURIComponent(m[1]))) return;
      setTarget({ col: decodeURIComponent(m[1]), path: m[2] ? m[2].split(".").filter(Boolean).map(Number) : [], row: m[3] ? Number(m[3]) : -1 });
    };
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, [colIndex]);

  const go = useCallback((col: string, path: number[], row = -1) => {
    const hash = `#/${encodeURIComponent(col)}/${path.join(".")}${row >= 0 ? `/${row}` : ""}`;
    history.replaceState(null, "", hash);
    setTarget({ col, path, row });
  }, []);

  return (
    <OutpostCtx.Provider value={libraryOf(outpost)}>
    <div className="ix">
      <Search root={root} base={base} go={go} />
      <OutpostAddress key={outpost} value={outpost} set={saveOutpost} />
      <div className="ix-head" aria-hidden="true">
        <span>Name</span>
        <span>Kind</span>
        <span>Length or contents</span>
        <span>Size</span>
        <span>Updated</span>
      </div>
      {root.subjects.map((s) => (
        <SubjectRow key={s.id} s={s} base={base} target={target} go={go} subjectsOf={subjectsOf} />
      ))}
    </div>
    </OutpostCtx.Provider>
  );
}

function SubjectRow({ s, base, target, go, subjectsOf }: {
  s: Subject; base: string; target: Tgt; go: Go; subjectsOf: Map<string, Subject[]>;
}) {
  const unique = s.collections.filter((c) => !(c.alsoIn || []).length || subjectsOf.get(c.id)?.[0]?.id === s.id);
  const items = unique.reduce((n, c) => n + c.items, 0);
  const size = unique.reduce((n, c) => n + c.size, 0);
  const wanted = !!target && s.collections.some((c) => c.id === target.col) && subjectsOf.get(target.col)?.[0]?.id === s.id;
  return (
    <details className="ix-subject" id={`subject-${s.id}`} open={wanted || undefined}>
      <summary className="ix-row lvl-0">
        <span className="ix-name">
          <strong>{s.name}</strong>
          <small>{s.purpose}</small>
        </span>
        <span className="ix-kind">{plural(s.collections.length, "collection")}</span>
        <span className="ix-len">{plural(items, "item")}</span>
        <span className="ix-size">{bytes(size)}</span>
        <span className="ix-date" />
      </summary>
      {s.sizes && Object.keys(s.sizes).length > 0 && (
        <p className="ix-note lvl-1">
          {s.kind === "tiered" ? "Sizes to install: " : "Choices: "}
          {Object.entries(s.sizes)
            .map(([k, v]) => `${k[0].toUpperCase()}${k.slice(1)} ${bytes(v)}`)
            .join(" · ")}
          {s.kind === "tiered" && ". Each size includes the one before it."}
        </p>
      )}
      {s.collections.map((c) => (
        <CollectionRow key={c.id} c={c} base={base} target={target} go={go} also={c.alsoIn || []} home={subjectsOf.get(c.id)?.[0]?.id === s.id} />
      ))}
    </details>
  );
}

type Tgt = { col: string; path: number[]; row: number } | null;
type Go = (col: string, path: number[], row?: number) => void;

function CollectionRow({ c, base, target, go, also, home }: { c: Collection; base: string; target: Tgt; go: Go; also: string[]; home: boolean }) {
  const [open, setOpen] = useState(false);
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [err, setErr] = useState("");
  const ref = useRef<HTMLDetailsElement>(null);
  const treeId = c.tree || c.id;
  const ed = c.ed || 0;
  const wanted = home && target?.col === treeId && (!c.tree || c.ed === 0);
  useEffect(() => {
    if (wanted && ref.current && !ref.current.open) ref.current.open = true;
  }, [wanted]);
  useEffect(() => {
    if (!open || tree) return;
    fetchJson<TreeNode>(`${base}/c/${c.tree || c.id}.json`).then(setTree, (e) => setErr(String(e)));
  }, [open, tree, base, c.id, c.tree]);
  const kinds = Object.entries(c.kinds || {});
  const other = Object.entries(c.other || {}).filter(([, v]) => v > 0);
  return (
    <details className="ix-collection" ref={ref} onToggle={(e) => setOpen((e.target as HTMLDetailsElement).open)}>
      <summary className="ix-row lvl-1">
        <span className="ix-name">
          <strong>{c.title}</strong>
          {c.tier && <em className="ix-tag">{c.tier}</em>}
          <small>{c.purpose}</small>
        </span>
        <span className="ix-kind">{kinds.length === 1 ? kinds[0][0] : kinds.length ? "mixed" : ""}</span>
        <span className="ix-len">{plural(c.items, "item")}</span>
        <span className="ix-size">{bytes(c.size)}</span>
        <span className="ix-date">{c.edition}</span>
      </summary>
      <div className="ix-about lvl-2">
        <dl>
          {c.description && c.description !== c.purpose && (
            <div>
              <dt>Publisher&apos;s description</dt>
              <dd>{c.longDescription || c.description}</dd>
            </div>
          )}
          <div>
            <dt>What is in it</dt>
            <dd>
              {kinds.map(([k, n]) => plural(n, k)).join(", ") || "Nothing listed"}
              {c.itemBytes > 0 && c.kind === "zim" && <>; {bytes(c.itemBytes)} of the file is these items, the rest is pictures, styles and the search index</>}
              {c.unmeasured > 0 && <>; {plural(c.unmeasured, "item")} not yet measured</>}
            </dd>
          </div>
          {other.length > 0 && (
            <div>
              <dt>Also in the file</dt>
              <dd>{other.map(([k, n]) => `${k} (${num(n)})`).join(", ")}</dd>
            </div>
          )}
          {c.publisher && (
            <div>
              <dt>Publisher</dt>
              <dd>{c.publisher}</dd>
            </div>
          )}
          {c.license && (
            <div>
              <dt>Licence</dt>
              <dd>{c.license}</dd>
            </div>
          )}
          {c.notice === "medical" && (
            <div>
              <dt>Notice</dt>
              <dd>Medical reference, not a substitute for a clinician.</dd>
            </div>
          )}
          {also.length > 0 && (
            <div>
              <dt>Also listed under</dt>
              <dd>{also.map((s) => <a key={s} href={`#subject-${s}`}>{s}</a>)}</dd>
            </div>
          )}
          {c.zim && (
            <div>
              <dt>Open</dt>
              <dd>
                <OpenLinks zim={c.zim} path="" />
              </dd>
            </div>
          )}
          {c.file && (
            <div>
              <dt>File</dt>
              <dd>
                <code>{c.file}</code>
                <br />
                <code className="ix-hash">SHA-256 {c.sha256}</code>
              </dd>
            </div>
          )}
        </dl>
      </div>
      {err && <p className="ix-note lvl-2">Could not load this collection&apos;s contents ({err}).</p>}
      {open && !tree && !err && <p className="ix-note lvl-2">Loading…</p>}
      {tree && <Children node={tree} col={treeId} ed={ed} zim={c.zim} path={[]} depth={2} base={base} target={wanted ? target : null} go={go} />}
    </details>
  );
}

type Where = { node: TreeNode; col: string; ed: number; zim?: string; path: number[]; depth: number; base: string; target: Tgt; go: Go };

function Children({ node, col, ed, zim, path, depth, base, target, go }: Where) {
  return (
    <>
      {(node.c || []).map((ch, i) => (
        <Section key={i} node={ch} col={col} ed={ed} zim={zim} path={[...path, i]} depth={depth} base={base} target={target} go={go} />
      ))}
      {node.r && <Leaf node={node} col={col} ed={ed} zim={zim} path={path} depth={depth} base={base} target={target} go={go} />}
    </>
  );
}

function onPath(target: Tgt, path: number[]) {
  return !!target && path.every((p, i) => target.path[i] === p) && target.path.length >= path.length;
}

function Section({ node, col, ed, zim, path, depth, base, target, go }: Where) {
  const [open, setOpen] = useState(false);
  const [kids, setKids] = useState<TreeNode[] | null>(node.c || null);
  const [err, setErr] = useState("");
  useEffect(() => {
    if (!open || kids || !node.cr) return;
    const [pack, off, len] = node.cr;
    fetchPacked<TreeNode[]>(`${base}/p/${encodeURIComponent(col)}.${pack}.bin`, off, len).then(setKids, (e) => setErr(String(e)));
  }, [open, kids, node.cr, base, col]);
  const ref = useRef<HTMLDetailsElement>(null);
  const wanted = onPath(target, path);
  useEffect(() => {
    if (wanted && ref.current && !ref.current.open) ref.current.open = true;
  }, [wanted]);
  return (
    <details className="ix-section" ref={ref} onToggle={(e) => setOpen((e.target as HTMLDetailsElement).open)}>
      <summary className={`ix-row lvl-${Math.min(depth, 7)}`}>
        <span className="ix-name">
          {node.l || "Contents"}
          {node.d && <small>{node.d}</small>}
        </span>
        <span className="ix-kind">{node.c || node.cr ? "section" : "shelf"}</span>
        <span className="ix-len">{plural(node.n, "item")}</span>
        <span className="ix-size">{node.u ? `${bytes(nodeSize(node, ed))}+` : bytes(nodeSize(node, ed))}</span>
        <span className="ix-date" />
      </summary>
      {err && <p className={`ix-note lvl-${Math.min(depth + 1, 7)}`}>Could not load this section ({err}).</p>}
      {open && node.cr && !kids && !err && <p className={`ix-note lvl-${Math.min(depth + 1, 7)}`}>Loading…</p>}
      {open && (kids || !node.cr) && (
        <Children node={{ ...node, c: kids || node.c }} col={col} ed={ed} zim={zim} path={path} depth={depth + 1} base={base} target={wanted ? target : null} go={go} />
      )}
    </details>
  );
}

function Leaf({ node, col, ed, zim, path, depth, base, target, go }: Where) {
  const [rows, setRows] = useState<Row[] | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => {
    const [pack, off, len] = node.r!;
    fetchPacked<Row[]>(`${base}/p/${encodeURIComponent(col)}.${pack}.bin`, off, len).then(setRows, (e) => setErr(String(e)));
  }, [node, col, base]);
  const hit = target && target.path.length === path.length && onPath(target, path) ? target.row : -1;
  const where = path.join(".");
  useEffect(() => {
    if (rows && hit >= 0) document.getElementById(`row-${col}-${where}-${hit}`)?.scrollIntoView({ block: "center" });
  }, [rows, hit, col, where]);
  if (err) return <p className={`ix-note lvl-${Math.min(depth, 7)}`}>Could not load these items ({err}).</p>;
  if (!rows) return <p className={`ix-note lvl-${Math.min(depth, 7)}`}>Loading {plural(node.n, "item")}…</p>;
  return (
    <>
      {rows.map((r, i) => (
        <Item key={i} r={r} ed={ed} zim={zim} id={`row-${col}-${where}-${i}`} depth={depth} hit={i === hit} link={() => go(col, path, i)} />
      ))}
    </>
  );
}

function Item({ r, ed, zim, id, depth, hit, link }: { r: Row; ed: number; zim?: string; id: string; depth: number; hit: boolean; link: () => void }) {
  const facts: [string, string][] = [];
  if (r.by) facts.push([r.k === "talk" ? "Speaker" : r.k === "book" ? "Author" : "By", r.by]);
  if (r.x) facts.push(["About", r.x]);
  if (r.tags) facts.push(["Tags", r.tags]);
  if (r.asked) facts.push(["Asked", r.asked]);
  if (r.score !== undefined) facts.push(["Score", String(r.score)]);
  if (r.ans !== undefined && r.k === "question") facts.push(["Answers", String(r.ans)]);
  if (r.w && (r.pg || r.min || r.steps || r.k === "question")) facts.push(["Words", num(r.w)]);
  if (r.diff) facts.push(["Difficulty", r.diff]);
  if (r.time) facts.push(["Time needed", r.time]);
  if (r.fmt) facts.push(["Formats", r.fmt]);
  if (r.files) facts.push(["Files", String(r.files)]);
  if (r.gid) facts.push(["Gutenberg number", `#${r.gid}`]);
  if (r.ollama) facts.push(["Ollama name", r.ollama]);
  if (r.rank) facts.push(["Rank in Outpost's tests", `${r.rank}`]);
  if (r.bbox) facts.push(["Area", r.bbox.map((v) => v.toFixed(2)).join(", ") + " (W, S, E, N)"]);
  return (
    <details className={`ix-item${hit ? " hit" : ""}`} id={id} open={hit || undefined}>
      <summary className={`ix-row lvl-${Math.min(depth, 7)}`}>
        <span className="ix-name">{r.t}</span>
        <span className="ix-kind">{r.k}</span>
        <span className="ix-len">{length(r)}</span>
        <span className="ix-size">{sizeText(r, ed)}</span>
        <span className="ix-date">{r.d || ""}</span>
      </summary>
      <dl className={`ix-facts lvl-${Math.min(depth + 1, 8)}`}>
        {facts.map(([k, v]) => (
          <div key={k}>
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
        {zim && !(r.ss && r.ss[ed] === null) && (
          <div>
            <dt>Open</dt>
            <dd>
              <OpenLinks zim={zim} path={itemPath(r)} />
            </dd>
          </div>
        )}
        <div>
          <dt>Link</dt>
          <dd>
            <button type="button" className="ix-link" onClick={link}>
              Link to this item
            </button>
          </dd>
        </div>
      </dl>
    </details>
  );
}

// --- search --------------------------------------------------------------------

function Search({ root, base, go }: { root: IndexRoot; base: string; go: Go }) {
  const [q, setQ] = useState("");
  const [res, setRes] = useState<{ k: string; hits: Hit[] } | null>(null);
  const key = norm(q);
  const hits = res && res.k === key ? res.hits : null;
  const busy = key.length >= 2 && !hits;
  const colTitle = useMemo(() => {
    const m = new Map<string, string>();
    for (const s of root.subjects) for (const c of s.collections) m.set(c.id, c.title);
    return m;
  }, [root]);
  const collectionHits = useMemo(() => {
    const k = norm(q);
    if (k.length < 2) return [];
    const out: Collection[] = [];
    const seen = new Set<string>();
    for (const s of root.subjects)
      for (const c of s.collections)
        if (!seen.has(c.id) && (norm(c.title).includes(k) || norm(c.purpose).includes(k))) {
          seen.add(c.id);
          out.push(c);
        }
    return out.slice(0, 8);
  }, [q, root]);

  useEffect(() => {
    const k = key;
    if (k.length < 2) return;
    let stale = false;
    const t = setTimeout(async () => {
      let found: Hit[] = [];
      try {
        const idx = await fetchJson<{ blocks: [string, number, number, number][] }>(`${base}/s/index.json`);
        const b = idx.blocks;
        let lo = 0;
        let hi = b.length - 1;
        while (lo < hi) {
          const mid = (lo + hi + 1) >> 1;
          if (b[mid][0] <= k) lo = mid;
          else hi = mid - 1;
        }
        for (let i = lo; i < b.length && found.length < 60; i++) {
          if (i > lo && !b[i][0].startsWith(k) && b[i][0] > k) break;
          const block = await fetchPacked<Hit[]>(`${base}/s/s.${b[i][1]}.bin`, b[i][2], b[i][3]);
          for (const h of block) if (norm(h[0]).startsWith(k)) found.push(h);
        }
      } catch {
        found = [];
      }
      if (!stale) setRes({ k, hits: found.slice(0, 60) });
    }, 220);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [key, base]);

  return (
    <div className="ix-search" role="search">
      <label htmlFor="ix-q" className="label">
        Find an item by title
      </label>
      <input id="ix-q" type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Type the start of a title, e.g. water purification" autoComplete="off" />
      {key.length >= 2 && (
        <div className="ix-results" aria-live="polite">
          {collectionHits.map((c) => (
            <p key={c.id}>
              <button type="button" onClick={() => go(c.id, [])}>
                {c.title}
              </button>{" "}
              <span>collection · {plural(c.items, "item")} · {bytes(c.size)}</span>
            </p>
          ))}
          {busy && !hits && <p className="quiet">Searching…</p>}
          {hits && hits.length === 0 && !collectionHits.length && <p className="quiet">No titles start with that.</p>}
          {hits?.map(([title, refs], i) => (
            <p key={i}>
              <button type="button" onClick={() => go(root.collections[refs[0][0]], refs[0][1] ? refs[0][1].split(".").map(Number) : [], refs[0][2])}>
                {title}
              </button>{" "}
              <span>
                in{" "}
                {refs.slice(0, 4).map((r, j) => (
                  <a key={j} href={`#/${encodeURIComponent(root.collections[r[0]])}/${r[1]}/${r[2]}`}>
                    {colTitle.get(root.collections[r[0]])}
                  </a>
                ))}
                {refs.length > 4 && ` and ${refs.length - 4} more`}
              </span>
            </p>
          ))}
          {hits && hits.length >= 60 && <p className="quiet">Showing the first 60. Type more of the title to narrow it down.</p>}
        </div>
      )}
    </div>
  );
}
