import { render } from "preact";
import { lazy, Suspense } from "preact/compat";
import { useCallback, useEffect, useState } from "preact/hooks";
import "./styles.css";
import { api, setCSRF, type Status } from "./api";
import { Logo } from "./icons";
import { Downloads } from "./downloads";
import { Assistant, Content, Home, Library, Search, Settings } from "./pages";
import { Link, navigate, usePath } from "./router";
import { Claim, Login, Wizard } from "./wizard";

const Maps = lazy(() => import("./maps"));

function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [err, setErr] = useState("");
  const path = usePath();
  const refresh = useCallback(() => {
    api.get<Status>("/api/status").then(
      (s) => {
        setCSRF(s.csrf);
        setStatus(s);
        setErr("");
        document.title = s.nodeName ? `${s.nodeName} · Outpost` : "Outpost";
      },
      (e) => setErr(e.message),
    );
  }, []);
  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 30000);
    window.addEventListener("outpost:unauthorized", refresh);
    return () => {
      clearInterval(t);
      window.removeEventListener("outpost:unauthorized", refresh);
    };
  }, []);

  if (!status) return <main>{err ? <p class="alert warn">Cannot reach Outpost: {err}</p> : <p class="empty"><span class="spinner" /></p>}</main>;

  if (!status.claimed) {
    // A new install, or one whose owner was reset with "outpost reset-admin".
    return <main><Claim onDone={refresh} /></main>;
  }

  if (!status.setupComplete) {
    return (
      <main>
        {!status.signedIn ? <Login title="Finish setting up" onDone={refresh} /> : <Wizard status={status} refresh={refresh} onDone={() => { refresh(); navigate("/downloads"); }} />}
      </main>
    );
  }

  const needsAdmin = ["/content", "/downloads", "/settings"].includes(path);
  const privateBlocked = !status.publicAccess && !status.signedIn;
  const logout = async () => {
    await api.post("/api/logout").catch(() => {});
    setCSRF("");
    refresh();
    navigate("/");
  };
  const nav: [string, string, boolean][] = [
    ["/", "Home", true],
    ["/library", "Library", status.apps.library],
    ["/search", "Search", status.apps.library],
    ["/maps", "Maps", status.apps.maps],
    ["/assistant", "Assistant", status.apps.assistant],
    ["/content", "Content", status.signedIn],
    ["/downloads", "Downloads", status.signedIn],
    ["/settings", "Settings", status.signedIn],
  ];
  let page;
  if ((needsAdmin || privateBlocked || path === "/login") && !status.signedIn) {
    page = <Login onDone={() => { refresh(); if (path === "/login") navigate("/"); }} />;
  } else {
    switch (path) {
      case "/library": page = <Library status={status} />; break;
      case "/search": page = <Search status={status} />; break;
      case "/maps": page = <Suspense fallback={<p class="empty"><span class="spinner" /></p>}><Maps status={status} /></Suspense>; break;
      case "/assistant": page = <Assistant status={status} />; break;
      case "/content": page = <Content status={status} refresh={refresh} />; break;
      case "/downloads": page = <Downloads status={status} />; break;
      case "/settings": page = <Settings status={status} refresh={refresh} />; break;
      default: page = <Home status={status} />;
    }
  }
  return (
    <>
      <a class="skip" href="#main">Skip to content</a>
      <header class="topbar">
        <div class="topbar-inner">
          <Link href="/" class="brand"><Logo /> <span>{status.nodeName}</span></Link>
          <nav class="nav" aria-label="Main">
            {nav.filter((n) => n[2]).map(([href, label]) => (
              <Link key={href} href={href} aria-current={path === href ? "page" : undefined}>{label}</Link>
            ))}
          </nav>
          {status.signedIn ? <button class="btn small" onClick={logout}>Sign out</button> : <Link class="btn small" href="/login">Sign in</Link>}
        </div>
      </header>
      <main id="main">{page}</main>
    </>
  );
}

render(<App />, document.getElementById("app")!);
