// The Downloads page and the strip on the start page that points at it.
// Both ask the server once a second, so what they show is what is happening.
import { useEffect } from "preact/hooks";
import { api, type Job, type Status } from "./api";
import { bytes, duration } from "./format";
import { usePoll } from "./poll";
import { Link } from "./router";

/** What every download adds up to. Removals and cancelled jobs are left out. */
export function totals(jobs: Job[]) {
  const t = { count: 0, ready: 0, failed: 0, active: 0, size: 0, done: 0, speed: 0, left: 0 };
  for (const j of jobs) {
    if (j.action !== "install" || j.state === "cancelled") continue;
    const size = j.live?.total || j.size || 0;
    t.count++;
    t.size += size;
    if (j.state === "done") {
      t.ready++;
      t.done += size;
      continue;
    }
    t.done += Math.min(j.live?.done || 0, size);
    if (j.state === "failed") t.failed++;
    else t.active++;
    // Only bytes arriving from the network count as speed; rereading a file
    // that is already here is much faster and would promise too much.
    if (j.state === "running" && j.live?.phase === "downloading") t.speed += j.live.speed || 0;
  }
  t.left = Math.max(0, t.size - t.done);
  return t;
}

const KIND: Record<string, string> = { zim: "Title", "map-region": "Map", "map-world": "Map", "map-assets": "Map", model: "AI model" };
// Which rows come first: what is moving, then what is stuck, then what is finished.
const RANK: Record<string, number> = { running: 0, waiting: 1, queued: 2, failed: 3, cancelled: 4, done: 5 };

/** One row in plain words: its state, the line under its name, how far it is. */
function describe(j: Job): { label: string; detail: string; pct: number } {
  const size = j.live?.total || j.size || 0;
  const done = size ? Math.min(j.live?.done || 0, size) : j.live?.done || 0;
  // Only a finished download says 100%: a size in the catalog can be a
  // little under what finally arrives, above all for maps.
  const pct = j.state === "done" ? 100 : size ? Math.min(99, (done / size) * 100) : 0;
  const sofar = `${bytes(done)} of ${bytes(size)}`;
  if (j.action === "remove") {
    const label = j.state === "done" ? "Removed" : j.state === "running" ? "Removing" : j.state === "failed" ? "Did not work" : j.state === "cancelled" ? "Cancelled" : "Waiting its turn";
    return { label, detail: "", pct };
  }
  switch (j.state) {
    case "queued":
      return { label: "Waiting its turn", detail: bytes(size), pct };
    case "waiting":
      return { label: "Will try again", detail: `${sofar} so far // attempt ${j.attempts}`, pct };
    case "failed":
      return { label: "Did not work", detail: bytes(size), pct };
    case "cancelled":
      return { label: "Cancelled", detail: bytes(size), pct };
    case "done":
      return { label: "Ready", detail: `${bytes(j.size)} // checked`, pct: 100 };
  }
  const speed = j.live?.speed || 0;
  switch (j.live?.phase) {
    case "checking":
      return { label: "Checking", detail: `Reading back what was already here // ${sofar}`, pct };
    case "downloading": {
      const left = size - done;
      const pace = speed > 0 ? ` // ${bytes(speed)}/s // ${left > 0 ? `about ${duration(left / speed)} left` : "nearly there"}` : "";
      return { label: "Downloading", detail: sofar + pace, pct };
    }
    case "installing":
    case "unpacking":
      return { label: "Finishing", detail: "Putting it in place", pct };
    default:
      return { label: "Getting ready", detail: bytes(size), pct };
  }
}

function Bar({ pct, label, big }: { pct: number; label: string; big?: boolean }) {
  return (
    <div class={`progress${big ? " big" : ""}`} role="progressbar" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100} aria-label={label}>
      <div style={{ width: `${pct}%` }} />
    </div>
  );
}

/** The strip on the start page while anything is still coming down. */
export function JobSummary({ status }: { status: Status }) {
  const [data] = usePoll<{ jobs: Job[] }>(status.signedIn ? "/api/jobs" : null, 2000);
  if (!data) return null;
  const t = totals(data.jobs);
  if (!t.active && !t.failed) return null;
  const pct = t.size ? (t.done / t.size) * 100 : 0;
  return (
    <Link href="/downloads" class="banner">
      <div class="spread">
        <strong>
          {t.active > 0 && `Downloading: ${t.ready} of ${t.count} ready, ${bytes(t.left)} to go`}
          {t.failed > 0 && `${t.active ? ". " : ""}${t.failed} did not work`}
        </strong>
        <span class="small">{t.speed > 0 ? `${bytes(t.speed)}/s, about ${duration(t.left / t.speed)} left` : "See the downloads"}</span>
      </div>
      <Bar pct={pct} label="All downloads" />
    </Link>
  );
}

export function Downloads({ status }: { status: Status }) {
  const [data, err, reload] = usePoll<{ jobs: Job[] }>("/api/jobs", 1000);
  const act = (path: string) => api.post(path).then(reload, reload);
  const t = data ? totals(data.jobs) : null;
  const pct = t && t.size ? Math.min(100, (t.done / t.size) * 100) : 0;
  const busy = !!t && t.active > 0;

  // Someone who switches to another tab can still see how far it is.
  useEffect(() => {
    const base = status.nodeName ? `${status.nodeName} · Outpost` : "Outpost";
    document.title = busy ? `${Math.floor(pct)}% downloaded · ${base}` : base;
    return () => {
      document.title = base;
    };
  }, [busy, Math.floor(pct), status.nodeName]);

  if (err && !data) return <p class="alert warn">{err}</p>;
  if (!data || !t) return <p class="empty"><span class="spinner" /></p>;
  const rows = data.jobs.map((j, i) => ({ j, i })).sort((a, b) => RANK[a.j.state] - RANK[b.j.state] || a.i - b.i);
  const finished = t.count > 0 && t.active === 0 && t.failed === 0;
  const clearable = data.jobs.some((j) => j.state === "done" || j.state === "cancelled" || j.state === "failed");
  return (
    <div class="stack">
      <div class="spread">
        <div>
          <h1>Downloads</h1>
          <p class="muted" style={{ margin: 0 }}>Each file is checked against its publisher's fingerprint before Outpost uses it.</p>
        </div>
        {clearable && <button class="btn small" onClick={() => act("/api/jobs/clear")}>Clear finished</button>}
      </div>
      {err && <p class="alert warn" role="alert">Cannot reach Outpost just now. Still trying. ({err})</p>}

      {t.count > 0 &&
        (finished ? (
          <section class="overall" role="status">
            <div class="done-row">
              <div>
                <h2>Everything is ready</h2>
                <p class="muted small" style={{ margin: "0.3rem 0 0" }}>
                  {t.ready} download{t.ready === 1 ? "" : "s"}, {bytes(t.size)}, all checked. The internet can go now.
                </p>
              </div>
              <div class="row">
                {status.apps.library && <Link class="btn primary" href="/library">Open the library</Link>}
                {status.apps.maps && <Link class={`btn${status.apps.library ? "" : " primary"}`} href="/maps">Open the maps</Link>}
              </div>
            </div>
          </section>
        ) : (
          <section class="overall">
            <dl>
              <div>
                <dt>Ready</dt>
                <dd>
                  {t.ready} of {t.count}
                </dd>
              </div>
              <div>
                <dt>Downloaded</dt>
                <dd>
                  {bytes(t.done)}
                  <small>of {bytes(t.size)}</small>
                </dd>
              </div>
              <div>
                <dt>Speed</dt>
                <dd>{t.speed > 0 ? `${bytes(t.speed)}/s` : "–"}</dd>
              </div>
              <div>
                <dt>Time left</dt>
                <dd>{t.speed > 0 ? duration(t.left / t.speed) : "–"}</dd>
              </div>
            </dl>
            <Bar pct={pct} label="All downloads" big />
            <p>
              {t.failed > 0
                ? `${t.failed} did not work. Use “Try again” on ${t.failed === 1 ? "it" : "them"} below; the rest carry on.`
                : "You can close this page. Downloads carry on by themselves, and pick up where they stopped if the connection drops."}
            </p>
          </section>
        ))}

      {data.jobs.length === 0 ? (
        <div class="card empty">
          <p>Nothing is downloading.</p>
          <Link class="btn primary" href="/content">Choose what to keep</Link>
        </div>
      ) : (
        <div class="jobs">
          {rows.map(({ j }) => {
            const d = describe(j);
            const moving = j.state === "queued" || j.state === "running" || j.state === "waiting";
            return (
              <div class={`job is-${j.state}`} key={j.id}>
                <div class="job-main">
                  <strong>
                    <span class="kind">{j.action === "remove" ? "Remove" : KIND[j.kind] || "File"}</span>
                    {j.title}
                  </strong>
                  {d.detail && <span class="job-detail">{d.detail}</span>}
                  {j.error && <span class="job-error">{j.error}</span>}
                </div>
                <span class="job-state">{d.label}</span>
                <span class="job-pct">{j.action === "install" && j.state !== "cancelled" && j.state !== "queued" ? `${Math.floor(d.pct)}%` : ""}</span>
                <div class="job-actions">
                  {moving && <button class="btn small" onClick={() => act(`/api/jobs/${j.id}/cancel`)} aria-label={`Cancel ${j.title}`}>Cancel</button>}
                  {(j.state === "failed" || j.state === "cancelled") && (
                    <button class="btn small primary" onClick={() => act(`/api/jobs/${j.id}/retry`)} aria-label={`Try ${j.title} again`}>Try again</button>
                  )}
                </div>
                {/* A bar only where there is something to show on it. */}
                {j.action === "install" && (j.state === "running" || ((j.state === "waiting" || j.state === "failed") && d.pct > 0)) && <Bar pct={d.pct} label={j.title} />}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
