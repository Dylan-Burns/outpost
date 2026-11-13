import Link from "next/link";
import CopyCommand from "./components/CopyCommand";
import Globe from "./components/Globe";
import Planner from "./components/Planner";
import { bytes, loadCatalog, loadRelease, plannerData, releaseKeyFingerprint } from "@/lib/catalog";

const INSTALL = "curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash";

export default function Home() {
  const c = loadCatalog();
  const release = loadRelease();
  const archives = c.items.filter((i) => i.kind === "zim").length;
  const regions = c.items.filter((i) => i.kind === "map-region").length;
  const wikiAll = c.items.find((i) => i.id === "wikipedia_en_all_maxi");
  const fingerprint = releaseKeyFingerprint();
  return (
    <>
      <section className="hero">
        <Globe />
        <div className="wrap">
          <div className="hero-body">
            <h1>
              <span className="line">
                <span>Network</span> <span>down.</span>
              </span>{" "}
              <span className="line">
                <span>Library</span> <span>open.</span>
              </span>
            </h1>
            <p className="lede">
              Outpost turns a computer you already own into an offline server for Wikipedia, medical references, repair guides, street maps and a
              local AI. Every phone and laptop on your network can use it, with or without a connection to the outside world.
            </p>
            <div className="hero-actions">
              <CopyCommand command={INSTALL} />
              <div className="row">
                <Link href="/install">Step-by-step install guide</Link>
                <a href="#plan">See what fits on your drive</a>
              </div>
            </div>
          </div>
          <dl className="titleblock">
            <div>
              <dt>Release</dt>
              <dd>
                {release.version}
                <small>Signed{release.date ? `, ${release.date}` : ""}</small>
              </dd>
            </div>
            <div>
              <dt>Archives</dt>
              <dd>
                {archives}
                <small>Wikipedia to repair manuals</small>
              </dd>
            </div>
            <div>
              <dt>Map regions</dt>
              <dd>
                {regions}
                <small>Street level, worldwide</small>
              </dd>
            </div>
            <div>
              <dt>Accounts needed</dt>
              <dd>
                0<small>No sign-up, no telemetry</small>
              </dd>
            </div>
          </dl>
        </div>
      </section>

      <section id="how">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>How it works</h2>
            <p>From spare computer to field library in an afternoon.</p>
          </div>
          <ol className="cells steps">
            <li>
              <h3>Install with one command</h3>
              <p>
                Run the installer on any 64-bit Linux machine: an old laptop, a mini PC, a Raspberry Pi 5. It checks every download against a
                signed checksum and sets Outpost up to start at boot.
              </p>
            </li>
            <li>
              <h3>Pick your content</h3>
              <p>
                Open the dashboard from any browser, enter the setup code the installer printed, and choose collections in three sizes, the map
                regions you need and an AI model. A storage bar keeps you inside your disk.
              </p>
            </li>
            <li>
              <h3>Pull the plug</h3>
              <p>
                Outpost downloads in the background and verifies each file. Once it finishes, unplug the internet. The library, search, maps and
                assistant keep answering on your local network.
              </p>
            </li>
          </ol>
        </div>
      </section>

      <section id="plan" className="paper">
        <div className="wrap">
          <div className="sec-head">
            <h2>Plan your library</h2>
            <p>
              These are the same choices the setup wizard offers, with today&apos;s file sizes: {archives} curated archives, {regions} map regions
              and AI models from 1.4 to 8 GB. Change your mind later; the node adds and removes content without starting over.
            </p>
          </div>
          <Planner data={plannerData()} />
        </div>
      </section>

      <section id="inside">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>What runs on your node</h2>
            <p>One machine on your network. Every phone and laptop reaches it with a browser.</p>
          </div>
          <div className="inside">
            <div
              className="topology"
              role="img"
              aria-label="Diagram: phones, laptops and tablets on your network connect to the Outpost node, the dashboard on port 8080 and the library on port 8081. The node needs the internet only to download."
            >
              <div className="topo-net">
                <span className="topo-box dashed">Internet</span>
                <span className="topo-cut">
                  <span>Needed only to download</span>
                </span>
              </div>
              <div className="topo-node paper">
                <p>
                  <strong>Outpost node</strong>
                  <span>Your hardware</span>
                </p>
                <ul>
                  <li>
                    <b>:8080</b> Dashboard, maps, assistant
                  </li>
                  <li>
                    <b>:8081</b> Library
                  </li>
                </ul>
                <p className="topo-helpers">
                  <span>kiwix-serve</span>
                  <span>pmtiles</span>
                  <span>Ollama</span>
                </p>
              </div>
              <div className="topo-bus">
                <span>Your network</span>
              </div>
              <div className="topo-devices">
                <span className="topo-box">Phone</span>
                <span className="topo-box">Laptop</span>
                <span className="topo-box">Tablet</span>
              </div>
            </div>
            <div className="features">
              <div className="feature">
                <p className="spec">Port 8081 // Full-text search</p>
                <h3>A library you can search</h3>
                <p>
                  Wikipedia from a {bytes(c.items.find((i) => i.id === "wikipedia_en_top_mini")?.size || 0)} pocket edition up to the full{" "}
                  {bytes(wikiAll?.size || 0)} with pictures, plus WikiMed, iFixit, Project Gutenberg, Khan Academy, LibreTexts and practical Stack
                  Exchange sites. Full-text search runs across all of it at once.
                </p>
              </div>
              <div className="feature">
                <p className="spec">{regions} regions // Zoom 15</p>
                <h3>Street maps without a signal</h3>
                <p>
                  Choose countries, US states, Canadian provinces or whole continents, cut at street-level detail from the current OpenStreetMap
                  planet build. A world overview is always included so you can pan between them.
                </p>
              </div>
              <div className="feature">
                <p className="spec">Local model // Cited answers</p>
                <h3>An assistant that cites its sources</h3>
                <p>
                  Optional local AI through Ollama. It searches your library first and answers with numbered links to the articles it used, so you
                  can read the original. On medical and electrical questions it points you to the text instead of paraphrasing doses or steps.
                </p>
              </div>
              <div className="feature">
                <p className="spec">Any browser // No app</p>
                <h3>Made for the whole household</h3>
                <p>
                  Anyone on your Wi-Fi opens it in a browser, no app needed. You decide whether visitors can read freely or need the admin
                  password. Content management always needs it.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="hardware">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>Bring your own hardware</h2>
            <p>Outpost has no box to buy. Anything that runs 64-bit Linux works; what you get depends on what you bring.</p>
          </div>
          <table className="hw">
            <thead>
              <tr>
                <th scope="col">Setup</th>
                <th scope="col">Typical machine</th>
                <th scope="col">Good for</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <th scope="row">Pocket</th>
                <td data-label="Typical machine">Raspberry Pi 5 (8 GB) or an old laptop, 256 GB SSD or USB drive</td>
                <td data-label="Good for">Essential collections, Wikipedia text, a few map regions. Skip the AI or use the 1.7B model.</td>
              </tr>
              <tr>
                <th scope="row">Household</th>
                <td data-label="Typical machine">Used mini PC (Intel 8th gen or Ryzen 5, 16 GB RAM), 1 TB SSD</td>
                <td data-label="Good for">
                  Standard collections, Wikipedia with pictures, your country&apos;s maps, a 3–4B model at conversational speed.
                </td>
              </tr>
              <tr>
                <th scope="row">Basecamp</th>
                <td data-label="Typical machine">Recent mini PC or desktop with 32 GB RAM or a GPU, 2–4 TB SSD</td>
                <td data-label="Good for">Everything: Khan Academy, all of Gutenberg, continents of maps, 8–12B models for the whole family.</td>
              </tr>
            </tbody>
          </table>
          <p className="hw-note">
            A small solar panel and battery keep a mini PC (15–35 W) running through an outage. Pair it with a Wi-Fi access point and everyone
            nearby can connect.
          </p>
        </div>
      </section>

      <section id="trust">
        <div className="wrap">
          <div className="sec-head">
            <h2>Built to be trusted offline</h2>
            <p>Every release is signed, every file is checked, and nothing reports back.</p>
          </div>
          <div className="trust-grid">
            <ul className="trust">
              <li>
                <h3>Signed from end to end</h3>
                <p>
                  Releases and the content catalog are signed with an Ed25519 key; the installer and the node refuse anything that does not verify.
                </p>
              </li>
              <li>
                <h3>Every file checked</h3>
                <p>
                  Each archive is compared with the SHA-256 its publisher lists before it is used. Interrupted downloads resume instead of
                  restarting.
                </p>
              </li>
              <li>
                <h3>Yours from the first minute</h3>
                <p>A one-time setup code printed on the machine claims the node, so a neighbour on the same Wi-Fi cannot get there first.</p>
              </li>
              <li>
                <h3>Nothing phones home</h3>
                <p>
                  No accounts, analytics or telemetry. The node contacts the internet only to download what you picked and, if you allow it, to
                  check for updates.
                </p>
              </li>
            </ul>
            <aside className="paper file" aria-label="What Outpost holds on you">
              <p className="file-head">
                <span>Subject file</span>
                <span>You</span>
              </p>
              <h3>What we hold on you</h3>
              <dl>
                <div>
                  <dt>Account</dt>
                  <dd>None exists</dd>
                </div>
                <div>
                  <dt>Email</dt>
                  <dd>Never asked</dd>
                </div>
                <div>
                  <dt>Searches</dt>
                  <dd>Stay on your node</dd>
                </div>
                <div>
                  <dt>Reading history</dt>
                  <dd>Stays on your node</dd>
                </div>
                <div>
                  <dt>Analytics</dt>
                  <dd>None</dd>
                </div>
                <div>
                  <dt>Telemetry</dt>
                  <dd>None</dd>
                </div>
              </dl>
              <p className="stamp">No record</p>
            </aside>
          </div>
          {fingerprint && (
            <p className="keyline">
              <span>Release signing key, Ed25519</span>
              <code>{fingerprint}</code>
              <Link href="/docs#security">How Outpost is secured</Link>
            </p>
          )}
        </div>
      </section>

      <section className="cta paper">
        <div className="wrap">
          <h2>Start with the computer in your closet</h2>
          <p className="lede">Ubuntu, Debian, Fedora or Raspberry Pi OS, 64-bit. Add the AI assistant with one more flag.</p>
          <CopyCommand command={INSTALL} />
          <p className="cta-note">
            With the AI assistant: <code>… | sudo bash -s -- --with-ai</code>
            <Link href="/install">Full install guide</Link>
          </p>
        </div>
      </section>
    </>
  );
}
