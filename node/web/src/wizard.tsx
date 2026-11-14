import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { api, setCSRF, type Apps, type CatalogResponse, type Selection, type Status } from "./api";
import { bytes } from "./format";
import { Logo } from "./icons";
import { AppsEditor, CollectionsEditor, ModelsEditor, RegionsEditor, StorageBar, WikipediaEditor, defaultSelection, plan } from "./selection";

const CLI = "sudo /opt/outpost/cli/outpost";

/** The installer opens this page with the setup code after the "#", so the
 *  person who just ran it does not have to copy twelve characters across.
 *  It is read once and taken out of the address bar. */
function codeFromAddress(): string {
  const m = location.hash.match(/^#code=([A-Za-z0-9]{4}(?:-[A-Za-z0-9]{4}){2})$/);
  if (!m) return "";
  history.replaceState(null, "", location.pathname + location.search);
  return m[1].toUpperCase();
}

export function Claim({ onDone }: { onDone: () => void }) {
  const [given] = useState(codeFromAddress);
  const [code, setCode] = useState(given);
  const [pw, setPw] = useState("");
  const [pw2, setPw2] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: Event) => {
    e.preventDefault();
    setErr("");
    if (pw.length < 10) return setErr("Use at least 10 characters for the password.");
    if (pw !== pw2) return setErr("The two passwords do not match.");
    setBusy(true);
    try {
      const r = await api.post<{ csrf: string }>("/api/setup/claim", { code, password: pw });
      setCSRF(r.csrf);
      onDone();
    } catch (e: any) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div class="center-card">
      <span class="mark"><Logo /></span>
      <h1>Welcome to Outpost</h1>
      <p class="muted" style={{ marginBottom: "1.5rem" }}>Two things make this Outpost yours: the setup code, and a password you choose.</p>
      <form onSubmit={submit}>
        <label class="field">
          <span>Setup code</span>
          <input type="text" autocomplete="one-time-code" placeholder="XXXX-XXXX-XXXX" value={code} onInput={(e) => setCode((e.target as HTMLInputElement).value)} required />
          <div class="hint">
            {given && code === given ? (
              "Filled in for you by the installer."
            ) : (
              <>
                The installer showed it when it finished. To see it again, run <code>{CLI} setup-code</code> on the computer where Outpost is installed.
              </>
            )}
          </div>
        </label>
        <label class="field">
          <span>Choose a password</span>
          <input type="password" autocomplete="new-password" value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} required minLength={10} autofocus={!!given} />
          <div class="hint">At least 10 characters. You will need it to change what is installed.</div>
        </label>
        <label class="field">
          <span>Repeat the password</span>
          <input type="password" autocomplete="new-password" value={pw2} onInput={(e) => setPw2((e.target as HTMLInputElement).value)} required />
        </label>
        {err && <p class="alert warn" role="alert">{err}</p>}
        <button class="btn primary" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
          {busy ? <span class="spinner" /> : "Start setup"}
        </button>
      </form>
    </div>
  );
}

export function Login({ onDone, title }: { onDone: () => void; title?: string }) {
  const [pw, setPw] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: Event) => {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const r = await api.post<{ csrf: string }>("/api/login", { password: pw });
      setCSRF(r.csrf);
      onDone();
    } catch (e: any) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div class="center-card">
      <h1>{title || "Sign in"}</h1>
      <p class="muted" style={{ marginBottom: "1.5rem" }}>Enter the password for this Outpost.</p>
      <form onSubmit={submit}>
        <label class="field">
          <span>Password</span>
          <input type="password" autocomplete="current-password" value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} required autofocus />
        </label>
        {err && <p class="alert warn" role="alert">{err}</p>}
        <button class="btn primary" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
          {busy ? <span class="spinner" /> : "Sign in"}
        </button>
        <p class="hint" style={{ marginTop: "1rem" }}>
          Forgot it? On the computer where Outpost is installed, run <code>{CLI} reset-admin</code>. Everything you downloaded stays.
        </p>
      </form>
    </div>
  );
}

// suggestedModel is the model setup ticks for you: the best in Outpost's
// assistant tests that runs well in this computer's memory, or else the
// smallest one.
function suggestedModel<T extends { kind: string; ramGB?: number; rank?: number; size: number }>(items: T[], memBytes: number): T | undefined {
  const models = items.filter((i) => i.kind === "model");
  const memGB = memBytes / 1e9;
  const fits = models.filter((m) => m.rank && (!memGB || memGB >= (m.ramGB || 0) * 1.25)).sort((a, b) => (a.rank || 0) - (b.rank || 0));
  return fits[0] || [...models].sort((a, b) => a.size - b.size)[0];
}

const STEPS = ["What it does", "Wikipedia", "Subjects", "Maps", "AI models", "Review"];

export function Wizard({ status, refresh, onDone }: { status: Status; refresh: () => void; onDone: () => void }) {
  const [data, setData] = useState<CatalogResponse | null>(null);
  const [err, setErr] = useState("");
  const [step, setStep] = useState(0);
  const [apps, setApps] = useState<Apps>({ library: true, maps: true, assistant: status.aiAvailable });
  const [sel, setSel] = useState<Selection | null>(null);
  const [name, setName] = useState(status.nodeName || "Outpost");
  const [busy, setBusy] = useState(false);
  const hadAI = useRef(status.aiAvailable);

  // The assistant's program can arrive while this page is open: the first
  // step shows the line that installs it. When it does, switch the assistant
  // on and pick a model, as if it had been there from the start.
  useEffect(() => {
    if (!status.aiAvailable || hadAI.current) return;
    hadAI.current = true;
    setApps((a) => ({ ...a, assistant: true }));
    const pick = data && suggestedModel(data.catalog.items, data.system.memBytes);
    if (pick) setSel((s) => (s && s.models.length === 0 ? { ...s, models: [pick.id] } : s));
  }, [status.aiAvailable, data]);

  useEffect(() => {
    api.get<CatalogResponse>("/api/catalog").then((d) => {
      setData(d);
      const s = defaultSelection(d.catalog);
      if (status.aiAvailable) {
        const pick = suggestedModel(d.catalog.items, d.system.memBytes);
        if (pick) s.models = [pick.id];
      }
      setSel(s);
    }, (e) => setErr(e.message));
  }, []);

  const p = useMemo(() => (data && sel ? plan(data, sel, apps) : null), [data, sel, apps]);
  if (err) return <p class="alert warn">{err}</p>;
  if (!data || !sel || !p) return <p class="empty"><span class="spinner" /> Loading the list of things you can keep…</p>;

  const visible = STEPS.map((s, i) => ({ s, i })).filter(({ s }) => !(s === "Wikipedia" || s === "Subjects" ? !apps.library : s === "Maps" ? !apps.maps : s === "AI models" ? !apps.assistant : false));
  const pos = visible.findIndex((v) => v.i === step);
  const next = () => (setStep(visible[Math.min(pos + 1, visible.length - 1)].i), window.scrollTo(0, 0));
  const back = () => (setStep(visible[Math.max(pos - 1, 0)].i), window.scrollTo(0, 0));
  const over = (data.disk.total - data.disk.free) + p.addBytes - data.disk.partial > data.disk.total * 0.98;

  const finish = async () => {
    setBusy(true);
    setErr("");
    try {
      await api.post("/api/setup/complete", { selection: sel, apps, nodeName: name });
      onDone();
    } catch (e: any) {
      setErr(e.message);
      setBusy(false);
    }
  };

  return (
    <div class="wizard">
      <ol class="steps" aria-label="Setup steps">
        {visible.map((v, k) => (
          <li key={v.s} class={v.i === step ? "current" : k < pos ? "done" : ""} aria-current={v.i === step ? "step" : undefined}>
            {v.s}
          </li>
        ))}
      </ol>
      {step > 0 && <StorageBar data={data} p={p} />}

      {step === 0 && (
        <section class="stack">
          <div>
            <h1>What should your Outpost do?</h1>
            <p class="muted">Pick what you want it to be. Nothing here is final: you can change all of it later.</p>
          </div>
          <label class="field" style={{ maxWidth: 420 }}>
            <span>Give it a name</span>
            <input type="text" value={name} maxLength={60} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
            <div class="hint">Shown at the top of every page, for example “Cabin library”.</div>
          </label>
          <AppsEditor apps={apps} setApps={setApps} aiAvailable={status.aiAvailable} recheck={refresh} />
          <p class="small muted">
            This computer: {data.system.cpu || data.system.arch}, {bytes(data.system.memBytes)} of memory, {bytes(data.disk.free)} free of{" "}
            {bytes(data.disk.total)}
            {data.system.gpu ? `, ${data.system.gpu}` : ""}.
          </p>
        </section>
      )}
      {step === 1 && (
        <section class="stack">
          <div>
            <h1>How much Wikipedia?</h1>
            <p class="muted">From a pocket reference to the whole encyclopedia with pictures.</p>
          </div>
          <WikipediaEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} />
        </section>
      )}
      {step === 2 && (
        <section class="stack">
          <div>
            <h1>Choose your subjects</h1>
            <p class="muted">Each subject comes in three sizes. Essential is small and practical. Comprehensive is everything we have picked.</p>
          </div>
          <CollectionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} />
        </section>
      )}
      {step === 3 && (
        <section class="stack">
          <div>
            <h1>Which maps?</h1>
            <p class="muted">Pick the places you need. A state or province is much smaller than a whole country.</p>
          </div>
          <RegionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} />
        </section>
      )}
      {step === 4 && (
        <section class="stack">
          <div>
            <h1>AI models</h1>
            <p class="muted">A model is the brain of the assistant, and it runs on this computer. Bigger ones answer better but slower. Pick at least one.</p>
          </div>
          <ModelsEditor c={data.catalog} sel={sel} setSel={setSel} memBytes={data.system.memBytes} installed={data.installed} />
        </section>
      )}
      {step === 5 && (
        <section class="stack">
          <div>
            <h1>Ready to download</h1>
            <p class="muted">
              Outpost fetches everything by itself, smallest first, so there is something to read within minutes, and it checks every file. Keep this
              computer on and connected until it finishes. If the connection drops, it carries on from where it stopped.
            </p>
          </div>
          <div class="card">
            <table class="facts">
              <tbody>
                <tr><td>Name</td><td>{name}</td></tr>
                <tr><td>It will be</td><td>{[apps.library && "A library", apps.maps && "Maps", apps.assistant && "An AI assistant"].filter(Boolean).join(", ") || "Nothing yet"}</td></tr>
                <tr><td>Downloads</td><td>{p.add.length}</td></tr>
                <tr><td>Total size</td><td class="mono">{bytes(p.addBytes)}</td></tr>
                <tr><td>Free space now</td><td class="mono">{bytes(data.disk.free)}</td></tr>
              </tbody>
            </table>
          </div>
          {over && <p class="alert warn">This is more than the disk can hold. Go back and choose smaller sizes or fewer maps.</p>}
          {p.add.length === 0 && <p class="alert">Nothing is selected yet. That is fine: you can add things later under Content.</p>}
        </section>
      )}
      {err && <p class="alert warn" role="alert">{err}</p>}
      <div class="wizard-nav spread">
        <button class="btn" onClick={back} disabled={pos === 0}>Back</button>
        {step !== 5 ? (
          <button class="btn primary" onClick={next}>Continue</button>
        ) : (
          <button class="btn primary" onClick={finish} disabled={busy || over}>
            {busy ? <span class="spinner" /> : p.add.length ? "Start downloading" : "Finish setup"}
          </button>
        )}
      </div>
    </div>
  );
}
