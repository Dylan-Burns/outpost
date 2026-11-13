import { useEffect, useMemo, useState } from "preact/hooks";
import { api, setCSRF, type Apps, type CatalogResponse, type Selection, type Status } from "./api";
import { bytes } from "./format";
import { Logo } from "./icons";
import { AppsEditor, CollectionsEditor, ModelsEditor, RegionsEditor, StorageBar, WikipediaEditor, defaultSelection, plan } from "./selection";

export function Claim({ onDone }: { onDone: () => void }) {
  const [code, setCode] = useState("");
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
    <div class="center-card card">
      <div class="row" style={{ marginBottom: "1rem" }}>
        <span style={{ width: 44, height: 44 }}><Logo /></span>
        <div>
          <h1 style={{ margin: 0 }}>Welcome to Outpost</h1>
          <p class="muted" style={{ margin: 0 }}>Let's set up this node.</p>
        </div>
      </div>
      <form onSubmit={submit}>
        <label class="field">
          <span>Setup code</span>
          <input type="text" autocomplete="one-time-code" placeholder="XXXX-XXXX-XXXX" value={code} onInput={(e) => setCode((e.target as HTMLInputElement).value)} required />
          <div class="hint">
            The installer printed it. To see it again, run <code>sudo /opt/outpost/cli/outpost setup-code</code> on the computer where Outpost is installed.
          </div>
        </label>
        <label class="field">
          <span>Choose an admin password</span>
          <input type="password" autocomplete="new-password" value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} required minLength={10} />
          <div class="hint">At least 10 characters. You need it to change content and settings.</div>
        </label>
        <label class="field">
          <span>Repeat the password</span>
          <input type="password" autocomplete="new-password" value={pw2} onInput={(e) => setPw2((e.target as HTMLInputElement).value)} required />
        </label>
        {err && <p class="alert warn" role="alert">{err}</p>}
        <button class="btn primary" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
          {busy ? <span class="spinner" /> : "Claim this node"}
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
    <div class="center-card card">
      <h1>{title || "Sign in"}</h1>
      <p class="muted">Enter the admin password of this node.</p>
      <form onSubmit={submit}>
        <label class="field">
          <span>Password</span>
          <input type="password" autocomplete="current-password" value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} required autofocus />
        </label>
        {err && <p class="alert warn" role="alert">{err}</p>}
        <button class="btn primary" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
          {busy ? <span class="spinner" /> : "Sign in"}
        </button>
        <p class="hint" style={{ marginTop: "1rem" }}>Forgot it? On the computer where Outpost is installed, run <code>sudo /opt/outpost/cli/outpost reset-admin</code>. Your content stays.</p>
      </form>
    </div>
  );
}

const STEPS = ["Apps", "Wikipedia", "Collections", "Maps", "AI models", "Review"];

export function Wizard({ status, onDone }: { status: Status; onDone: () => void }) {
  const [data, setData] = useState<CatalogResponse | null>(null);
  const [err, setErr] = useState("");
  const [step, setStep] = useState(0);
  const [apps, setApps] = useState<Apps>({ library: true, maps: true, assistant: status.aiAvailable });
  const [sel, setSel] = useState<Selection | null>(null);
  const [name, setName] = useState(status.nodeName || "Outpost");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.get<CatalogResponse>("/api/catalog").then((d) => {
      setData(d);
      const s = defaultSelection(d.catalog);
      if (status.aiAvailable) {
        const small = d.catalog.items.find((i) => i.kind === "model");
        if (small) s.models = [small.id];
      }
      setSel(s);
    }, (e) => setErr(e.message));
  }, []);

  const p = useMemo(() => (data && sel ? plan(data, sel, apps) : null), [data, sel, apps]);
  if (err) return <p class="alert warn">{err}</p>;
  if (!data || !sel || !p) return <p class="empty"><span class="spinner" /> Loading the catalog…</p>;

  const visible = STEPS.map((s, i) => ({ s, i })).filter(({ s }) => !(s === "Wikipedia" || s === "Collections" ? !apps.library : s === "Maps" ? !apps.maps : s === "AI models" ? !apps.assistant : false));
  const pos = visible.findIndex((v) => v.i === step);
  const next = () => setStep(visible[Math.min(pos + 1, visible.length - 1)].i);
  const back = () => setStep(visible[Math.max(pos - 1, 0)].i);
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
            {k + 1}. {v.s}
          </li>
        ))}
      </ol>
      {step > 0 && <StorageBar data={data} p={p} />}

      {step === 0 && (
        <section class="stack">
          <div>
            <h1>What should this node do?</h1>
            <p class="muted">Pick the apps you want. You can change everything later from the dashboard.</p>
          </div>
          <label class="field" style={{ maxWidth: 420 }}>
            <span>Name of this node</span>
            <input type="text" value={name} maxLength={60} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
            <div class="hint">Shown at the top of every page, e.g. “Cabin library”.</div>
          </label>
          <AppsEditor apps={apps} setApps={setApps} aiAvailable={status.aiAvailable} />
          <div class="alert small">
            <strong>{data.system.cpu || data.system.arch}</strong> · {data.system.cores} cores · {bytes(data.system.memBytes)} memory · {bytes(data.disk.free)} free of {bytes(data.disk.total)}
            {data.system.gpu ? ` · ${data.system.gpu}` : ""}
          </div>
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
            <h1>Choose your collections</h1>
            <p class="muted">Each collection has three sizes. Essential is small and practical; Comprehensive takes everything we curate.</p>
          </div>
          <CollectionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} />
        </section>
      )}
      {step === 3 && (
        <section class="stack">
          <div>
            <h1>Which maps?</h1>
            <p class="muted">Pick the places you need. States and provinces are smaller than whole countries.</p>
          </div>
          <RegionsEditor c={data.catalog} sel={sel} setSel={setSel} installed={data.installed} />
        </section>
      )}
      {step === 4 && (
        <section class="stack">
          <div>
            <h1>AI models</h1>
            <p class="muted">Models run on this machine. Bigger ones are smarter but slower; pick at least one.</p>
          </div>
          <ModelsEditor c={data.catalog} sel={sel} setSel={setSel} memBytes={data.system.memBytes} installed={data.installed} />
        </section>
      )}
      {step === 5 && (
        <section class="stack">
          <div>
            <h1>Ready to download</h1>
            <p class="muted">Outpost downloads everything in the background, smallest first, and checks each file against its published checksum. Keep this machine online until it finishes; interrupted downloads resume.</p>
          </div>
          <div class="card">
            <table class="facts">
              <tbody>
                <tr><td>Node name</td><td>{name}</td></tr>
                <tr><td>Apps</td><td>{[apps.library && "Library", apps.maps && "Maps", apps.assistant && "AI assistant"].filter(Boolean).join(", ") || "None"}</td></tr>
                <tr><td>Items</td><td>{p.want.size} ({p.add.length} to download)</td></tr>
                <tr><td>Download size</td><td class="mono">{bytes(p.addBytes)}</td></tr>
                <tr><td>Free space now</td><td class="mono">{bytes(data.disk.free)}</td></tr>
              </tbody>
            </table>
          </div>
          {over && <p class="alert warn">This selection does not fit on the disk. Go back and choose smaller tiers or fewer regions.</p>}
          {p.add.length === 0 && <p class="alert">Nothing selected yet. That's fine: you can add content later from Content.</p>}
        </section>
      )}
      {err && <p class="alert warn" role="alert">{err}</p>}
      <div class="wizard-nav spread">
        <button class="btn" onClick={back} disabled={pos === 0}>Back</button>
        {step !== 5 ? (
          <button class="btn primary" onClick={next}>Continue</button>
        ) : (
          <button class="btn primary" onClick={finish} disabled={busy || over}>
            {busy ? <span class="spinner" /> : "Complete setup"}
          </button>
        )}
      </div>
    </div>
  );
}
