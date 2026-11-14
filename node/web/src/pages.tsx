import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { api, streamChat, type Apps, type CatalogResponse, type Selection, type Status } from "./api";
import { JobSummary } from "./downloads";
import { bytes, count, duration, libraryBase, newer } from "./format";
import { CopyLine } from "./copyline";
import { usePoll } from "./poll";
import { Icon } from "./icons";
import { CollectionsEditor, ModelsEditor, RegionsEditor, StorageBar, WikipediaEditor, plan, AppsEditor } from "./selection";
import { navigate, Link } from "./router";

// The installer does it all: run again it updates, with a flag it removes.
const UPDATE = "curl -fsSL https://projectoutpost.org/install.sh | sudo bash";
const REMOVE_ALL = `${UPDATE} -s -- --uninstall --purge`;
const REMOVE_PROGRAM = `${UPDATE} -s -- --uninstall`;

type Book = { id: string; title: string; description: string; path: string; size: number; date: string; category: string; notice?: string };

export function Home({ status }: { status: Status }) {
  const [q, setQ] = useState("");
  const [lib] = usePoll<{ books: Book[] }>("/api/library", 0);
  const [maps] = usePoll<{ regions: unknown[] }>("/api/maps", 0);
  const update = !!status.latestVersion && newer(status.latestVersion, status.version);
  const tiles = [
    { href: "/library", icon: "book", title: "Library", text: lib ? `${count(lib.books.length, "title")}: encyclopedias, manuals and lessons.` : "Encyclopedias, manuals and lessons.", show: status.apps.library },
    { href: "/search", icon: "search", title: "Search", text: "One search box for everything in the library.", show: status.apps.library },
    { href: "/maps", icon: "map", title: "Maps", text: maps ? `${count(maps.regions.length, "region")}, no internet needed.` : "Street maps, no internet needed.", show: status.apps.maps },
    { href: "/assistant", icon: "chat", title: "Assistant", text: "Ask a question; the answer shows its sources.", show: status.apps.assistant },
    { href: "/content", icon: "sliders", title: "Content", text: "Add or remove subjects, maps and AI models.", show: status.signedIn },
    { href: "/settings", icon: "gear", title: "Settings", text: "Who can read, your password, updates.", show: status.signedIn },
  ];
  return (
    <div>
      {status.signedIn && update && (
        <div class="alert ok" style={{ marginBottom: "1rem" }}>
          A newer Outpost is out: {status.latestVersion} (this one is {status.version}). <Link href="/settings">Settings</Link> shows how to update.
        </div>
      )}
      {status.signedIn && <JobSummary status={status} />}
      <div class="hero">
        <div>
          <h1>{status.nodeName}</h1>
          <p class="muted" style={{ margin: 0 }}>A library, maps and tools on your own network. No internet needed.</p>
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
          Is this your Outpost? <Link href="/login">Sign in</Link> to change what it keeps.
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
          <p class="muted" style={{ margin: 0 }}>{count(data.books.length, "title")} on this Outpost.</p>
        </div>
        <a class="btn" href={base + "/"} target="_blank" rel="noopener">Browse the whole library</a>
      </div>
      {!data.running && data.books.length > 0 && <p class="alert warn">The library is starting up. Titles open again within a few seconds.</p>}
      {data.books.length === 0 ? (
        <div class="card empty">
          <p>Nothing here yet.</p>
          {status.signedIn ? <Link class="btn primary" href="/content">Choose what to keep</Link> : <p class="small">The owner of this Outpost can add things.</p>}
        </div>
      ) : (
        <>
          <input type="search" placeholder="Filter titles…" value={filter} onInput={(e) => setFilter((e.target as HTMLInputElement).value)} aria-label="Filter titles" style={{ maxWidth: 420 }} />
          {[...groups.entries()].sort().map(([g, list]) => (
            <section key={g}>
              <h2>{g}</h2>
              <div class="grid cols-3">
                {list.map((b) => (
                  <a key={b.id} class="card book" href={base + b.path + "/"} target="_blank" rel="noopener" style={{ textDecoration: "none", color: "inherit" }}>
                    <strong>{b.title}</strong>
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
          <p class="small muted">Medical titles are references, not a substitute for a doctor or nurse. Each title keeps its publisher's licence.</p>
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
          {res.results.length === 0 && <p class="empty">Nothing found. Try fewer or simpler words. A few titles (some collections of PDFs) cannot be searched this way; open them from the Library.</p>}
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

type Source = { label: string; title: string; book: string; path: string; note?: string };
type Turn = { q: string; a: string; sources: Source[]; ungrounded: boolean; unsupported?: string[]; error: string; status: string; done: boolean; model?: string };

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
        else if (ev === "unsupported") upd((t) => (t.unsupported = data || []));
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
        <p>{!info.available ? "The AI program (Ollama) is not running on this computer." : "No AI model is installed yet."}</p>
        {status.signedIn ? <Link class="btn primary" href="/content">Add a model</Link> : <p class="small">The owner of this Outpost can add one.</p>}
      </div>
    );
  // Answers are plain text with [S1] marks; models still add **bold** and
  // "# headings" now and then, which would otherwise show as symbols.
  const linkify = (text: string, sources: Source[]) =>
    text.replace(/^#{1,6}[ \t]+/gm, "").split(/(\[S\d+\]|\*\*[^*\n]+\*\*)/g).map((part, i) => {
      const m = part.match(/^\[S(\d+)\]$/);
      if (m) {
        const s = sources[Number(m[1]) - 1];
        if (!s) return part;
        // An Outpost note has no library page; it opens below the answer.
        return s.note ? <abbr key={i} title={s.title}>{part}</abbr> : <a key={i} href={base + s.path} target="_blank" rel="noopener" title={s.title}>{part}</a>;
      }
      const b = part.match(/^\*\*([^*\n]+)\*\*$/);
      return b ? <strong key={i}>{b[1]}</strong> : part;
    });
  const cited = (t: Turn) => {
    const n = new Set([...t.a.matchAll(/\[S(\d+)\]/g)].map((m) => Number(m[1])));
    return t.sources.filter((_, i) => n.has(i + 1));
  };
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Assistant</h1>
          <p class="muted small" style={{ margin: 0 }}>It runs on this computer and it can be wrong. Check the sources it lists, above all on medical or safety questions.</p>
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
              {t.ungrounded && <p class="alert warn small" style={{ whiteSpace: "normal" }}>Nothing in your library matched, so this answer comes from the AI model alone, with no source to check.</p>}
              {t.a ? linkify(t.a, t.sources) : !t.error && <span class="muted"><span class="spinner" /> {t.status}</span>}
              {t.error && <p class="alert warn" style={{ whiteSpace: "normal" }}>{t.error}</p>}
              {t.unsupported && t.unsupported.length > 0 && (
                <p class="alert warn small" style={{ whiteSpace: "normal" }}>
                  Not found in the sources: {t.unsupported.join(", ")}. The model may have made {t.unsupported.length > 1 ? "these" : "this"} up; check the source before relying on it.
                </p>
              )}
              {t.sources.length > 0 && (() => {
                // Once the answer is done, list what it rests on; the other
                // passages it was shown are a footnote, not evidence.
                const used = t.done ? cited(t) : t.sources;
                const shown = used.length > 0 ? used : t.sources;
                const rest = t.sources.filter((s) => !shown.includes(s));
                return (
                  <>
                    <div class="sources">
                      {shown.map((s) =>
                        s.note ? (
                          <details key={s.label} class="note-src">
                            <summary>[{s.label}] {s.title} · {s.book}</summary>
                            <p>{s.note}</p>
                          </details>
                        ) : (
                          <a key={s.label} href={base + s.path} target="_blank" rel="noopener">[{s.label}] {s.title} · {s.book}</a>
                        ),
                      )}
                    </div>
                    {rest.length > 0 && (
                      <p class="muted small" style={{ margin: 0 }}>
                        Also searched: {rest.map((s, i) => <>{i > 0 && " · "}{s.note ? <span key={s.label}>{s.title}</span> : <a key={s.label} href={base + s.path} target="_blank" rel="noopener">{s.title}</a>}</>)}
                      </p>
                    )}
                  </>
                );
              })()}
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
      setMsg("Saved. It is downloading and removing now.");
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
  const tabs: [string, string][] = [["apps", "What it does"], ["collections", "Subjects"], ["wikipedia", "Wikipedia"], ["maps", "Maps"], ["models", "AI models"]];
  const outdated = data.outdated || [];
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Content</h1>
          <p class="muted" style={{ margin: 0 }}>Change what your Outpost keeps. Nothing happens until you press Apply changes.</p>
        </div>
      </div>
      {outdated.length > 0 && (
        <div class="alert ok spread">
          <span>Newer editions are out for {outdated.length} title{outdated.length > 1 ? "s" : ""}: {outdated.slice(0, 4).map((id) => items.get(id)?.title || id).join(", ")}{outdated.length > 4 ? "…" : ""}</span>
          <button class="btn small" onClick={updateAll}>Update all</button>
        </div>
      )}
      <StorageBar data={data} p={p} />
      <div class="nav" role="tablist" style={{ flex: "none", borderBottom: "1px solid var(--line-2)" }}>
        {tabs.map(([id, label]) => (
          <a href="#" role="tab" key={id} aria-current={tab === id ? "page" : undefined} aria-selected={tab === id} style={{ padding: "0.7rem 0" }} onClick={(e) => (e.preventDefault(), setTab(id))}>{label}</a>
        ))}
      </div>
      {tab === "apps" && <AppsEditor apps={apps} setApps={setApps} aiAvailable={status.aiAvailable} recheck={refresh} />}
      {tab === "collections" && (apps.library ? <CollectionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">The library is switched off. Turn it on under “What it does”.</p>)}
      {tab === "wikipedia" && (apps.library ? <WikipediaEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">The library is switched off. Turn it on under “What it does”.</p>)}
      {tab === "maps" && (apps.maps ? <RegionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} /> : <p class="alert">Maps are switched off. Turn them on under “What it does”.</p>)}
      {tab === "models" && (apps.assistant ? <ModelsEditor c={data.catalog} sel={sel} setSel={setSel} memBytes={data.system.memBytes} installed={data.installed} /> : <p class="alert">The assistant is switched off{status.aiAvailable ? ". Turn it on under “What it does”." : ", and Ollama, the program it needs, is not on this computer yet. “What it does” shows the line that installs it."}</p>)}
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
      setCatMsg(r.changed ? "A newer list arrived. See Content." : "Nothing new.");
      reload();
    } catch (e: any) {
      setCatMsg(e.message);
    }
  };
  const update = !!status.latestVersion && newer(status.latestVersion, status.version);
  return (
    <div class="stack">
      <h1>Settings</h1>
      <div class="grid cols-2">
        <section class="card">
          <h2>Who can read</h2>
          <label class="field">
            <span>Name of this Outpost</span>
            <input type="text" value={s.nodeName} maxLength={60} onInput={(e) => setS({ ...s, nodeName: (e.target as HTMLInputElement).value })} />
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.publicAccess} onChange={(e) => setS({ ...s, publicAccess: (e.target as HTMLInputElement).checked })} />
            <span><strong>Anyone on your Wi-Fi can read</strong><br /><span class="small muted">The library, search and maps open without signing in. Turn this off to ask for the password for everything.</span></span>
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.assistantPublic} disabled={!s.publicAccess} onChange={(e) => setS({ ...s, assistantPublic: (e.target as HTMLInputElement).checked })} />
            <span><strong>Guests can use the assistant</strong><br /><span class="small muted">Every answer keeps this computer busy for a while. Turn this off on a slow machine.</span></span>
          </label>
          <label class="switch">
            <input type="checkbox" checked={s.checkForUpdates} onChange={(e) => setS({ ...s, checkForUpdates: (e.target as HTMLInputElement).checked })} />
            <span><strong>Look for updates when there is internet</strong><br /><span class="small muted">Fetches the list of what is available and the newest version number from the Outpost website. Nothing about this Outpost is sent.</span></span>
          </label>
          <div class="row"><button class="btn primary" onClick={save}>Save</button><span class="small muted">{msg}</span></div>
        </section>
        <section class="card">
          <h2>Password</h2>
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
            <tr><td>Newest release</td><td class="mono">{status.latestVersion || "unknown (no internet?)"}</td></tr>
            <tr><td>Catalog</td><td class="mono">{sys?.catalog?.generated || "…"}</td></tr>
            <tr><td>Library</td><td>{sys?.library?.running ? `running, ${sys.library.books} titles` : "not running"}</td></tr>
            <tr><td>AI program</td><td>{status.aiAvailable ? `Ollama ${status.aiVersion}` : "not installed"}</td></tr>
          </tbody></table>
          <p class={`small ${update ? "alert ok" : "muted"}`} style={{ marginTop: "0.8rem" }}>
            {update ? `Version ${status.latestVersion} is out. ` : ""}To update, paste this line into Terminal on the Outpost computer. It keeps everything you downloaded and all your settings.
          </p>
          <CopyLine text={UPDATE} />
          <div class="row" style={{ marginTop: "0.8rem" }}><button class="btn small" onClick={refreshCatalog}>Look for new titles now</button><span class="small muted">{catMsg}</span></div>
        </section>
        <section class="card">
          <h2>This computer</h2>
          {err && <p class="alert warn">{err}</p>}
          {sys && (
            <table class="facts"><tbody>
              <tr><td>Name on the network</td><td>{sys.system.hostname} ({sys.system.os}/{sys.system.arch})</td></tr>
              <tr><td>Processor</td><td>{sys.system.cpu || "unknown"} · {sys.system.cores} cores</td></tr>
              <tr><td>Memory</td><td class="mono">{bytes(sys.system.memBytes)}</td></tr>
              <tr><td>Graphics</td><td>{sys.system.gpu || "none detected"}</td></tr>
              <tr><td>Storage</td><td><span class="mono">{bytes(sys.disk.free)}</span> free of <span class="mono">{bytes(sys.disk.total)}</span><div class="small muted">{sys.disk.path}</div></td></tr>
              <tr><td>Running for</td><td>{duration(sys.system.uptimeSeconds) || "just started"}</td></tr>
            </tbody></table>
          )}
        </section>
        <section class="card whole-row">
          <h2>Remove Outpost</h2>
          <p class="small">
            <strong>Remove everything.</strong> Paste this line into Terminal on the Outpost computer. It takes away the program, everything you
            downloaded{sys ? <> (kept in <span class="mono">{sys.disk.path}</span>)</> : ""}, your password and your settings. It asks once before it
            deletes. Installing again afterwards starts from the beginning, with a new setup code.
          </p>
          <CopyLine text={REMOVE_ALL} />
          <p class="small" style={{ marginTop: "1.1rem" }}>
            <strong>Remove the program, keep the downloads.</strong> The library, the maps and your settings stay on the drive. Installing again
            picks them up, with the same password.
          </p>
          <CopyLine text={REMOVE_PROGRAM} />
          <p class="small muted" style={{ marginTop: "0.8rem", marginBottom: 0 }}>
            Ollama, the separate program behind the AI assistant, is not removed by either line.
          </p>
        </section>
      </div>
    </div>
  );
}
