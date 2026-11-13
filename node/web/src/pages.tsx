import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { api, streamChat, type Apps, type CatalogResponse, type Job, type Selection, type Status } from "./api";
import { bytes, duration, libraryBase } from "./format";
import { Icon } from "./icons";
import { CollectionsEditor, ModelsEditor, RegionsEditor, StorageBar, WikipediaEditor, plan, AppsEditor } from "./selection";
import { navigate, Link } from "./router";

function usePoll<T>(path: string | null, ms: number): [T | null, string, () => void] {
  const [data, setData] = useState<T | null>(null);
  const [err, setErr] = useState("");
  const [tick, setTick] = useState(0);
  useEffect(() => {
    if (!path) return;
    let alive = true;
    const load = () =>
      api.get<T>(path).then(
        (d) => alive && (setData(d), setErr("")),
        (e) => alive && setErr(e.message),
      );
    load();
    const t = ms ? setInterval(load, ms) : 0;
    return () => {
      alive = false;
      if (t) clearInterval(t);
    };
  }, [path, ms, tick]);
  return [data, err, () => setTick((t) => t + 1)];
}

type Book = { id: string; title: string; description: string; path: string; size: number; date: string; category: string; notice?: string };

function JobSummary({ status }: { status: Status }) {
  const [data] = usePoll<{ jobs: Job[] }>(status.signedIn ? "/api/jobs" : null, 3000);
  if (!data) return null;
  const active = data.jobs.filter((j) => j.state === "running" || j.state === "queued" || j.state === "waiting");
  const failed = data.jobs.filter((j) => j.state === "failed");
  if (!active.length && !failed.length) return null;
  const remaining = active.reduce((n, j) => n + Math.max(0, (j.live?.total || j.size) - (j.live?.done || 0)), 0);
  const speed = active.reduce((n, j) => n + (j.live?.speed || 0), 0);
  return (
    <Link href="/downloads" class={`alert ${failed.length ? "warn" : "ok"}`} style={{ display: "block", textDecoration: "none", color: "inherit", marginBottom: "1.25rem" }}>
      <div class="spread">
        <strong>
          {active.length ? `Downloading ${active.length} item${active.length > 1 ? "s" : ""} · ${bytes(remaining)} left` : ""}
          {failed.length ? ` ${failed.length} failed` : ""}
        </strong>
        <span class="small muted">{speed > 0 ? `${bytes(speed)}/s · about ${duration(remaining / speed)}` : "View downloads →"}</span>
      </div>
    </Link>
  );
}

export function Home({ status }: { status: Status }) {
  const [q, setQ] = useState("");
  const [lib] = usePoll<{ books: Book[] }>("/api/library", 0);
  const [maps] = usePoll<{ regions: unknown[] }>("/api/maps", 0);
  const update = status.latestVersion && status.version !== "dev" && status.latestVersion !== status.version;
  const tiles = [
    { href: "/library", icon: "book", title: "Library", text: `${lib?.books.length ?? "…"} archives: encyclopedias, manuals and courses.`, show: status.apps.library },
    { href: "/search", icon: "search", title: "Search", text: "Full-text search across everything at once.", show: status.apps.library },
    { href: "/maps", icon: "map", title: "Maps", text: `${maps?.regions.length ?? "…"} regions available offline.`, show: status.apps.maps },
    { href: "/assistant", icon: "chat", title: "Assistant", text: "Ask questions; answers cite your library.", show: status.apps.assistant },
    { href: "/content", icon: "sliders", title: "Content", text: "Add or remove collections, maps and models.", show: status.signedIn },
    { href: "/settings", icon: "gear", title: "Settings", text: "Access, password, system and updates.", show: status.signedIn },
  ];
  return (
    <div>
      {status.signedIn && update && (
        <div class="alert ok" style={{ marginBottom: "1rem" }}>
          Outpost {status.latestVersion} is available (you have {status.version}). Update from <Link href="/settings">Settings</Link>.
        </div>
      )}
      {status.signedIn && <JobSummary status={status} />}
      <div class="hero">
        <div>
          <h1>{status.nodeName}</h1>
          <p class="muted" style={{ margin: 0 }}>Offline knowledge, maps and tools, served from this machine.</p>
        </div>
      </div>
      {status.apps.library && (
        <form class="searchbar" style={{ marginBottom: "1.5rem" }} onSubmit={(e) => (e.preventDefault(), q.trim() && navigate(`/search?q=${encodeURIComponent(q.trim())}`))}>
          <input type="search" placeholder="Search the library…" value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} aria-label="Search the library" />
          <button class="btn primary">Search</button>
        </form>
      )}
      <div class="grid cols-3">
        {tiles.filter((t) => t.show).map((t) => (
          <Link key={t.href} href={t.href} class="card tile">
            <span class="icon"><Icon name={t.icon} /></span>
            <h2 style={{ margin: 0 }}>{t.title}</h2>
            <p class="muted small" style={{ margin: 0 }}>{t.text}</p>
          </Link>
        ))}
      </div>
      {!status.signedIn && (
        <p class="small muted" style={{ marginTop: "2rem" }}>
          Administrator? <Link href="/login">Sign in</Link> to manage content.
        </p>
      )}
    </div>
  );
}

export function Library({ status }: { status: Status }) {
  const [data, err] = usePoll<{ books: Book[]; running: boolean }>("/api/library", 15000);
  const [filter, setFilter] = useState("");
  const base = libraryBase(status.libraryPort);
  if (err) return <p class="alert warn">{err}</p>;
  if (!data) return <p class="empty"><span class="spinner" /></p>;
  const books = data.books.filter((b) => !filter || (b.title + b.description + b.category).toLowerCase().includes(filter.toLowerCase()));
  const groups = new Map<string, Book[]>();
  for (const b of books) {
    const g = b.category || "Other";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(b);
  }
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Library</h1>
          <p class="muted" style={{ margin: 0 }}>{data.books.length} archives on this node.</p>
        </div>
        <a class="btn" href={base + "/"} target="_blank" rel="noopener">Open the library browser ↗</a>
      </div>
      {!data.running && data.books.length > 0 && <p class="alert warn">The library server is starting or stopped. Archives open again within a few seconds.</p>}
      {data.books.length === 0 ? (
        <div class="card empty">
          <p>No archives yet.</p>
          {status.signedIn ? <Link class="btn primary" href="/content">Choose content</Link> : <p class="small">The administrator can add content.</p>}
        </div>
      ) : (
        <>
          <input type="search" placeholder="Filter archives…" value={filter} onInput={(e) => setFilter((e.target as HTMLInputElement).value)} aria-label="Filter archives" style={{ maxWidth: 420 }} />
          {[...groups.entries()].sort().map(([g, list]) => (
            <section key={g}>
              <h2>{g}</h2>
              <div class="grid cols-3">
                {list.map((b) => (
                  <a key={b.id} class="card book" href={base + b.path + "/"} target="_blank" rel="noopener" style={{ textDecoration: "none", color: "inherit" }}>
                    <strong style={{ color: "var(--accent)" }}>{b.title}</strong>
                    <span class="small muted">{b.description}</span>
                    <span class="meta">
                      <span class="mono">{bytes(b.size)}</span>
                      {b.date && <span>{b.date}</span>}
                      {b.notice === "medical" && <span class="badge plain">Medical reference</span>}
                    </span>
                  </a>
                ))}
              </div>
            </section>
          ))}
          <p class="small muted">Medical archives are references, not a substitute for a clinician. Archives keep their publishers' licences.</p>
        </>
      )}
    </div>
  );
}

type Hit = { title: string; path: string; snippet: string; book: string };

export function Search({ status }: { status: Status }) {
  const params = new URLSearchParams(location.search);
  const [q, setQ] = useState(params.get("q") || "");
  const [query, setQuery] = useState(params.get("q") || "");
  const [start, setStart] = useState(0);
  const [res, setRes] = useState<{ results: Hit[]; total: number } | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const base = libraryBase(status.libraryPort);
  useEffect(() => {
    if (!query) return;
    setBusy(true);
    setErr("");
    api.get<{ results: Hit[]; total: number }>(`/api/search?q=${encodeURIComponent(query)}&start=${start}`).then(
      (r) => (setRes(r), setBusy(false)),
      (e) => (setErr(e.message), setBusy(false)),
    );
  }, [query, start]);
  return (
    <div class="stack">
      <h1>Search</h1>
      <form
        class="searchbar"
        onSubmit={(e) => {
          e.preventDefault();
          setStart(0);
          setQuery(q.trim());
          history.replaceState(null, "", `/search?q=${encodeURIComponent(q.trim())}`);
        }}
      >
        <input type="search" value={q} placeholder="e.g. purify water, set a broken bone, rebuild a carburetor" onInput={(e) => setQ((e.target as HTMLInputElement).value)} aria-label="Search terms" autofocus />
        <button class="btn primary">Search</button>
      </form>
      {busy && <p class="muted"><span class="spinner" /> Searching…</p>}
      {err && <p class="alert warn">{err}</p>}
      {res && !busy && (
        <div>
          <p class="muted small">{res.total.toLocaleString()} results{res.total > 0 && ` · showing ${start + 1}–${start + res.results.length}`}</p>
          {res.results.length === 0 && <p class="empty">Nothing found. Try fewer or simpler words. Archives without a full-text index (some PDF collections) are browsed from the Library.</p>}
          {res.results.map((r) => (
            <div class="result" key={r.path}>
              <a class="title" href={base + r.path} target="_blank" rel="noopener">{r.title}</a>
              <div class="small muted">{r.book}</div>
              <p>{r.snippet}</p>
            </div>
          ))}
          <div class="row" style={{ marginTop: "1rem" }}>
            <button class="btn" disabled={start === 0} onClick={() => setStart(Math.max(0, start - 20))}>Previous</button>
            <button class="btn" disabled={start + 20 >= res.total} onClick={() => setStart(start + 20)}>Next</button>
          </div>
        </div>
      )}
    </div>
  );
}

type Source = { label: string; title: string; book: string; path: string };
type Turn = { q: string; a: string; sources: Source[]; ungrounded: boolean; error: string; status: string; done: boolean; model?: string };

export function Assistant({ status }: { status: Status }) {
  const [info, err] = usePoll<{ available: boolean; models: { id: string; title: string }[] }>("/api/assistant", 0);
  const [model, setModel] = useState("");
  const [useLib, setUseLib] = useState(true);
  const [q, setQ] = useState("");
  const [turns, setTurns] = useState<Turn[]>([]);
  const ctrl = useRef<AbortController | null>(null);
  const base = libraryBase(status.libraryPort);
  const busy = turns.length > 0 && !turns[turns.length - 1].done;
  const ask = async (e: Event) => {
    e.preventDefault();
    const question = q.trim();
    if (!question || busy) return;
    setQ("");
    const turn: Turn = { q: question, a: "", sources: [], ungrounded: false, error: "", status: "Starting…", done: false };
    setTurns((t) => [...t, turn]);
    const upd = (f: (t: Turn) => void) => setTurns((ts) => { const c = [...ts]; const last = { ...c[c.length - 1] }; f(last); c[c.length - 1] = last; return c; });
    ctrl.current = new AbortController();
    try {
      await streamChat({ question, model: model || undefined, library: useLib }, (ev, data) => {
        if (ev === "token") upd((t) => (t.a += data, t.status = ""));
        else if (ev === "sources") upd((t) => (t.sources = data || []));
        else if (ev === "ungrounded") upd((t) => (t.ungrounded = true));
        else if (ev === "status") upd((t) => (t.status = data));
        else if (ev === "error") upd((t) => (t.error = data));
        else if (ev === "done") upd((t) => (t.model = data?.model));
      }, ctrl.current.signal);
    } catch (e: any) {
      if (e.name !== "AbortError") upd((t) => (t.error = e.message));
    }
    upd((t) => (t.done = true, t.status = ""));
  };
  if (err) return <p class="alert warn">{err}</p>;
  if (!info) return <p class="empty"><span class="spinner" /></p>;
  if (!info.available || info.models.length === 0)
    return (
      <div class="card empty">
        <h1>Assistant</h1>
        <p>{!info.available ? "The AI service is not running on this node." : "No AI model is installed yet."}</p>
        {status.signedIn ? <Link class="btn primary" href="/content">Add a model</Link> : <p class="small">The administrator can add one.</p>}
      </div>
    );
  const linkify = (text: string, sources: Source[]) =>
    text.split(/(\[S\d+\])/g).map((part, i) => {
      const m = part.match(/^\[S(\d+)\]$/);
      const s = m && sources[Number(m[1]) - 1];
      return s ? <a key={i} href={base + s.path} target="_blank" rel="noopener" title={s.title}>{part}</a> : part;
    });
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Assistant</h1>
          <p class="muted small" style={{ margin: 0 }}>Runs on this machine. It can be wrong: check the cited sources, especially for medical or safety questions.</p>
        </div>
        <div class="row">
          <label class="switch small" style={{ padding: 0 }}>
            <input type="checkbox" checked={useLib} onChange={(e) => setUseLib((e.target as HTMLInputElement).checked)} /> Answer from my library
          </label>
          {info.models.length > 1 && (
            <select value={model} onChange={(e) => setModel((e.target as HTMLSelectElement).value)} aria-label="Model" style={{ width: "auto" }}>
              {info.models.map((m) => <option value={m.id} key={m.id}>{m.title}</option>)}
            </select>
          )}
        </div>
      </div>
      <div class="chat" aria-live="polite">
        {turns.length === 0 && <p class="empty">Ask anything. Try “How do I make water safe to drink?”</p>}
        {turns.map((t, i) => (
          <div key={i} class="stack">
            <div class="msg q">{t.q}</div>
            <div class="msg a">
              {t.ungrounded && <p class="alert warn small" style={{ whiteSpace: "normal" }}>Nothing in your library matched, so this answer comes from the model alone.</p>}
              {t.a ? linkify(t.a, t.sources) : !t.error && <span class="muted"><span class="spinner" /> {t.status}</span>}
              {t.error && <p class="alert warn" style={{ whiteSpace: "normal" }}>{t.error}</p>}
              {t.sources.length > 0 && (
                <div class="sources">
                  {t.sources.map((s) => (
                    <a key={s.label} href={base + s.path} target="_blank" rel="noopener">[{s.label}] {s.title} · {s.book}</a>
                  ))}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
      <form class="composer" onSubmit={ask}>
        <textarea value={q} placeholder="Ask a question…" aria-label="Question" maxLength={1000} onInput={(e) => setQ((e.target as HTMLTextAreaElement).value)}
          onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); ask(e); } }} />
        {busy ? <button type="button" class="btn" onClick={() => ctrl.current?.abort()}>Stop</button> : <button class="btn primary">Ask</button>}
      </form>
    </div>
  );
}

export function Content({ status, refresh }: { status: Status; refresh: () => void }) {
  const [data, err, reload] = usePoll<CatalogResponse>("/api/catalog", 0);
  const [sel, setSel] = useState<Selection | null>(null);
  const [apps, setApps] = useState<Apps | null>(null);
  const [tab, setTab] = useState("collections");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (data && !sel) {
      setSel({ ...data.selection, tiers: { ...(data.selection.tiers || {}) }, regions: data.selection.regions || [], models: data.selection.models || [] });
      setApps(data.apps);
    }
  }, [data]);
  const p = useMemo(() => (data && sel && apps ? plan(data, sel, apps) : null), [data, sel, apps]);
  if (err) return <p class="alert warn">{err}</p>;
  if (!data || !sel || !apps || !p) return <p class="empty"><span class="spinner" /></p>;
  const items = new Map(data.catalog.items.map((i) => [i.id, i]));
  const over = data.disk.total - data.disk.free + p.addBytes - data.disk.partial > data.disk.total * 0.98;
  const apply = async () => {
    setBusy(true);
    setMsg("");
    try {
      await api.post("/api/selection", { selection: sel, apps });
      setMsg("Saved. Downloads and removals are running in the background.");
      reload();
      refresh();
    } catch (e: any) {
      setMsg(e.message);
    }
    setBusy(false);
  };
  const updateAll = async () => {
    try {
      const r = await api.post<{ queued: number }>("/api/updates/apply");
      setMsg(`${r.queued} update${r.queued === 1 ? "" : "s"} queued.`);
      reload();
    } catch (e: any) {
      setMsg(e.message);
    }
  };
  const tabs: [string, string][] = [["apps", "Apps"], ["collections", "Collections"], ["wikipedia", "Wikipedia"], ["maps", "Maps"], ["models", "AI models"]];
  const outdated = data.outdated || [];
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Content</h1>
          <p class="muted" style={{ margin: 0 }}>Change what this node keeps. Nothing changes until you apply.</p>
        </div>
      </div>
      {outdated.length > 0 && (
        <div class="alert ok spread">
          <span>Newer editions are available for {outdated.length} archive{outdated.length > 1 ? "s" : ""}: {outdated.slice(0, 4).map((id) => items.get(id)?.title || id).join(", ")}{outdated.length > 4 ? "…" : ""}</span>
          <button class="btn small" onClick={updateAll}>Update all</button>
        </div>
      )}
      <StorageBar data={data} p={p} />
      <div class="nav" role="tablist" style={{ flex: "none" }}>
        {tabs.map(([id, label]) => (
          <a href="#" role="tab" key={id} aria-current={tab === id ? "page" : undefined} aria-selected={tab === id} onClick={(e) => (e.preventDefault(), setTab(id))}>{label}</a>
        ))}
      </div>
      {tab === "apps" && <AppsEditor apps={apps} setApps={setApps} aiAvailable={status.aiAvailable} />}
      {tab === "collections" && (apps.library ? <CollectionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">The library app is off. Turn it on under Apps.</p>)}
      {tab === "wikipedia" && (apps.library ? <WikipediaEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">The library app is off. Turn it on under Apps.</p>)}
      {tab === "maps" && (apps.maps ? <RegionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">The maps app is off. Turn it on under Apps.</p>)}
      {tab === "models" && (apps.assistant ? <ModelsEditor c={data.catalog} sel={sel} setSel={setSel} memBytes={data.system.memBytes} installed={data.installed} /> : <p class="alert">The assistant is off{status.aiAvailable ? ". Turn it on under Apps." : " and not installed. Re-run the installer with --with-ai."}</p>)}
      <div class="wizard-nav spread">
        <span class="small">
          {p.add.length > 0 && <>Add <strong>{p.add.length}</strong> ({bytes(p.addBytes)}) </>}
          {p.remove.length > 0 && <>Remove <strong>{p.remove.length}</strong> ({bytes(p.removeBytes)})</>}
          {p.add.length === 0 && p.remove.length === 0 && <span class="muted">No changes.</span>}
          {msg && <span class="muted"> · {msg}</span>}
        </span>
        <button class="btn primary" disabled={busy || over || (p.add.length === 0 && p.remove.length === 0)} onClick={apply}>
          {busy ? <span class="spinner" /> : "Apply changes"}
        </button>
      </div>
    </div>
  );
}

export function Downloads() {
  const [data, err, reload] = usePoll<{ jobs: Job[] }>("/api/jobs", 1500);
  const act = async (path: string) => {
    try {
      await api.post(path);
      reload();
    } catch {
      reload();
    }
  };
  if (err) return <p class="alert warn">{err}</p>;
  if (!data) return <p class="empty"><span class="spinner" /></p>;
  const label: Record<string, string> = { queued: "Queued", running: "Working", waiting: "Retrying soon", done: "Done", failed: "Failed", cancelled: "Cancelled" };
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Downloads</h1>
          <p class="muted" style={{ margin: 0 }}>Every file is checked against its published checksum before it is used.</p>
        </div>
        <button class="btn small" onClick={() => act("/api/jobs/clear")}>Clear finished</button>
      </div>
      <div class="card">
        {data.jobs.length === 0 && <p class="empty">No downloads.</p>}
        {data.jobs.map((j) => {
          const total = j.live?.total || j.size;
          const done = j.live?.done || 0;
          const pct = j.state === "done" ? 100 : total ? Math.min(100, (done / total) * 100) : 0;
          return (
            <div class="job" key={j.id}>
              <div>
                <strong>{j.action === "remove" ? "Remove " : ""}{j.title}</strong>
                <div class="small muted">
                  <span class={j.state === "failed" ? "badge warn" : j.state === "done" ? "badge" : "badge plain"}>{label[j.state]}</span>{" "}
                  {j.state === "running" && j.live && (
                    <>
                      {j.live.phase} · <span class="mono">{bytes(done)} / {bytes(total)}</span>
                      {j.live.speed > 0 && <> · {bytes(j.live.speed)}/s · {duration((total - done) / j.live.speed)} left</>}
                    </>
                  )}
                  {j.state !== "running" && j.action === "install" && <span class="mono">{bytes(j.size)}</span>}
                  {j.error && <div class="small" style={{ color: "var(--warn)" }}>{j.error}{j.state === "waiting" ? ` (attempt ${j.attempts})` : ""}</div>}
                </div>
              </div>
              <div class="row">
                {(j.state === "queued" || j.state === "running" || j.state === "waiting") && <button class="btn small" onClick={() => act(`/api/jobs/${j.id}/cancel`)}>Cancel</button>}
                {(j.state === "failed" || j.state === "cancelled") && <button class="btn small" onClick={() => act(`/api/jobs/${j.id}/retry`)}>Retry</button>}
              </div>
              {j.action === "install" && j.state !== "cancelled" && j.state !== "failed" && (
                <div class="progress" role="progressbar" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100} aria-label={j.title}>
                  <div style={{ width: `${pct}%` }} />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function Settings({ status, refresh }: { status: Status; refresh: () => void }) {
  const [sys, err, reload] = usePoll<any>("/api/system", 0);
  const [s, setS] = useState({ nodeName: status.nodeName, publicAccess: status.publicAccess, assistantPublic: status.assistantPublic, checkForUpdates: true });
  const [msg, setMsg] = useState("");
  const [pw, setPw] = useState({ current: "", next: "" });
  const [pwMsg, setPwMsg] = useState("");
  const [catMsg, setCatMsg] = useState("");
  const save = async () => {
    try {
      await api.put("/api/settings", s);
      setMsg("Saved.");
      refresh();
    } catch (e: any) {
      setMsg(e.message);
    }
  };
  const changePw = async (e: Event) => {
    e.preventDefault();
    try {
      await api.post("/api/password", { current: pw.current, new: pw.next });
      setPwMsg("Password changed. Other browsers were signed out.");
      setPw({ current: "", next: "" });
    } catch (e: any) {
      setPwMsg(e.message);
    }
  };
  const refreshCatalog = async () => {
    setCatMsg("Checking…");
    try {
      const r = await api.post<{ changed: boolean; sequence: number }>("/api/catalog/refresh");
      setCatMsg(r.changed ? "A newer catalog was installed." : "The catalog is up to date.");
      reload();
    } catch (e: any) {
      setCatMsg(e.message);
    }
  };
  const update = status.latestVersion && status.version !== "dev" && status.latestVersion !== status.version;
  return (
    <div class="stack">
      <h1>Settings</h1>
      <div class="grid cols-2">
        <section class="card">
          <h2>Access</h2>
          <label class="field">
            <span>Node name</span>
            <input type="text" value={s.nodeName} maxLength={60} onInput={(e) => setS({ ...s, nodeName: (e.target as HTMLInputElement).value })} />
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.publicAccess} onChange={(e) => setS({ ...s, publicAccess: (e.target as HTMLInputElement).checked })} />
            <span><strong>Anyone on the network can read</strong><br /><span class="small muted">Library, search and maps without signing in. Turn off to require the admin password for everything.</span></span>
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.assistantPublic} disabled={!s.publicAccess} onChange={(e) => setS({ ...s, assistantPublic: (e.target as HTMLInputElement).checked })} />
            <span><strong>Visitors can use the assistant</strong><br /><span class="small muted">Answers use this machine's processor; turn off on slow hardware.</span></span>
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.checkForUpdates} onChange={(e) => setS({ ...s, checkForUpdates: (e.target as HTMLInputElement).checked })} />
            <span><strong>Check for updates when online</strong><br /><span class="small muted">Fetches the signed catalog and the latest version number from the Outpost website. Nothing about this node is sent.</span></span>
          </label>
          <div class="row"><button class="btn primary" onClick={save}>Save</button><span class="small muted">{msg}</span></div>
        </section>
        <section class="card">
          <h2>Admin password</h2>
          <form onSubmit={changePw}>
            <label class="field"><span>Current password</span><input type="password" autocomplete="current-password" value={pw.current} onInput={(e) => setPw({ ...pw, current: (e.target as HTMLInputElement).value })} required /></label>
            <label class="field"><span>New password</span><input type="password" autocomplete="new-password" minLength={10} value={pw.next} onInput={(e) => setPw({ ...pw, next: (e.target as HTMLInputElement).value })} required /></label>
            <div class="row"><button class="btn">Change password</button><span class="small muted">{pwMsg}</span></div>
          </form>
        </section>
        <section class="card">
          <h2>Software</h2>
          <table class="facts"><tbody>
            <tr><td>Outpost</td><td class="mono">{status.version}</td></tr>
            <tr><td>Latest release</td><td class="mono">{status.latestVersion || "unknown (offline?)"}</td></tr>
            <tr><td>Catalog</td><td class="mono">{sys?.catalog?.generated || "…"}</td></tr>
            <tr><td>Library server</td><td>{sys?.library?.running ? `running, ${sys.library.books} archives` : "not running"}</td></tr>
            <tr><td>AI service</td><td>{status.aiAvailable ? `Ollama ${status.aiVersion}` : "not installed"}</td></tr>
          </tbody></table>
          {update && <p class="alert ok small" style={{ marginTop: "0.8rem" }}>Version {status.latestVersion} is available. On the node, run the installer again; it keeps your content and settings:</p>}
          <code class="kbd small">curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash</code>
          <div class="row" style={{ marginTop: "0.8rem" }}><button class="btn small" onClick={refreshCatalog}>Check for new content now</button><span class="small muted">{catMsg}</span></div>
        </section>
        <section class="card">
          <h2>System</h2>
          {err && <p class="alert warn">{err}</p>}
          {sys && (
            <table class="facts"><tbody>
              <tr><td>Host</td><td>{sys.system.hostname} ({sys.system.os}/{sys.system.arch})</td></tr>
              <tr><td>Processor</td><td>{sys.system.cpu || "unknown"} · {sys.system.cores} cores</td></tr>
              <tr><td>Memory</td><td class="mono">{bytes(sys.system.memBytes)}</td></tr>
              <tr><td>Graphics</td><td>{sys.system.gpu || "none detected"}</td></tr>
              <tr><td>Storage</td><td><span class="mono">{bytes(sys.disk.free)}</span> free of <span class="mono">{bytes(sys.disk.total)}</span><div class="small muted">{sys.disk.path}</div></td></tr>
              <tr><td>Running for</td><td>{duration(sys.system.uptimeSeconds) || "just started"}</td></tr>
            </tbody></table>
          )}
        </section>
      </div>
    </div>
  );
}
