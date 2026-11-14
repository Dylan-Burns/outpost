import Link from "next/link";
import CopyCommand from "./components/CopyCommand";
import Globe from "./components/Globe";
import Planner from "./components/Planner";
import Screens from "./components/Screens";
import { bytes, loadCatalog, plannerData, releaseKeyFingerprint } from "@/lib/catalog";
import { INSTALL } from "@/lib/site";

export default function Home() {
  const c = loadCatalog();
  const archives = c.items.filter((i) => i.kind === "zim").length;
  const regions = c.items.filter((i) => i.kind === "map-region").length;
  const models = c.items.filter((i) => i.kind === "model").length;
  const wikiAll = c.items.find((i) => i.id === "wikipedia_en_all_maxi");
  const fingerprint = releaseKeyFingerprint();
  return (
    <>
      <section className="hero">
        {/* The stage holds the globe and the words laid over it; the title block sits under it. */}
        <div className="hero-stage">
          <Globe />
          <div className="wrap">
            <div className="hero-body">
              <h1>
                <span className="setup">When the world goes dark,</span> <span className="line">information</span>{" "}
                <span className="line">is power.</span>
              </h1>
              <p className="lede">
                <span>
                  A tool for emergency preparedness and off-grid living. Outpost keeps Wikipedia, medical and survival guides, repair manuals,
                  street maps and an AI assistant on a computer you own, and shares them with every phone and laptop on your network. No
                  internet required.
                </span>
              </p>
              <div className="hero-actions">
                <p className="hero-or">To install, paste this line into Terminal on a Mac or a Linux PC:</p>
                <CopyCommand command={INSTALL} />
              </div>
            </div>
          </div>
        </div>
        <div className="wrap">
          <dl className="titleblock">
            <div>
              <dt>Cost</dt>
              <dd>
                Free<small>No subscription, no box to buy</small>
              </dd>
            </div>
            <div>
              <dt>Runs on</dt>
              <dd>
                Mac + Linux<small>A spare computer you own</small>
              </dd>
            </div>
            <div>
              <dt>Inside</dt>
              <dd>
                {archives} titles
                <small>
                  Plus {regions} maps and {models} AI models
                </small>
              </dd>
            </div>
            <div>
              <dt>Sign-ups</dt>
              <dd>
                None<small>No account, no tracking</small>
              </dd>
            </div>
          </dl>
        </div>
      </section>

      <section id="why">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>What it&apos;s for</h2>
            <p>The things you look up every day, kept where a dead connection can&apos;t take them away.</p>
          </div>
          <div className="cells uses">
            <div>
              <h3>When the internet goes out</h3>
              <p>Storms, power cuts, a cable dug up down the road. First aid, how to fix the generator, the back road to the next town: still there.</p>
            </div>
            <div>
              <h3>Where there is no signal</h3>
              <p>Cabins, boats, RVs, farms and job sites far from a tower. Take the library with you and leave the connection behind.</p>
            </div>
            <div>
              <h3>Where internet is slow or costly</h3>
              <p>Classrooms, clinics and community halls. Download once, and everyone in the room reads at full speed without using any data.</p>
            </div>
            <div>
              <h3>When it&apos;s nobody else&apos;s business</h3>
              <p>What you search for and what you ask stays inside your home. Nothing is sent to us or to anyone else.</p>
            </div>
          </div>
        </div>
      </section>

      <section id="how">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>How it works</h2>
            <p>From spare computer to home library in an afternoon. Most of that time is the downloads, which run by themselves.</p>
          </div>
          <ol className="cells steps">
            <li>
              <h3>Install it on a spare computer</h3>
              <p>
                An old laptop, a Mac mini or a small PC will do. Open the Terminal app, paste one line and press Return. That is the only typing
                there is: the installer does the rest, and on a Mac it opens the setup page for you.
              </p>
              <p className="spec">macOS 13+ or 64-bit Linux // signed release</p>
            </li>
            <li>
              <h3>Choose what to keep</h3>
              <p>
                On the setup page, from your phone or laptop, pick what you want: how much of Wikipedia, which medical and repair guides, which
                maps. A bar shows what fits on your drive.
              </p>
              <p className="spec">Three sizes per subject // change any time</p>
            </li>
            <li>
              <h3>Unplug and carry on</h3>
              <p>
                Outpost downloads everything once, shows you each file arriving, and checks every one. After that it needs no internet at all.
                The library, the maps and the assistant keep answering on your Wi-Fi.
              </p>
              <p className="spec">Starts by itself after a power cut</p>
            </li>
          </ol>
        </div>
      </section>

      <section id="screens">
        <div className="wrap">
          <div className="sec-head">
            <h2>The rest is point and click</h2>
            <p>
              After that one pasted line there is no more Terminal. Setup, and everything after it, happens on ordinary web pages like these, from
              your phone or your laptop.
            </p>
          </div>
          <Screens />
        </div>
      </section>

      <section id="plan" className="paper">
        <div className="wrap">
          <div className="sec-head">
            <h2>Plan your library</h2>
            <p>
              Try it here first. These are the same choices you get during setup, with the real size of today&apos;s files: {archives} titles,{" "}
              {regions} map regions and {models} AI models. You can change your mind later without starting over.
            </p>
          </div>
          <Planner data={plannerData()} />
        </div>
      </section>

      <section id="inside">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>What you get</h2>
            <p>One computer in your home does the work. Every phone, tablet and laptop on your Wi-Fi opens it in a browser, with nothing to install.</p>
          </div>
          <div className="inside">
            <div
              className="topology"
              role="img"
              aria-label="Diagram: phones, laptops and tablets on your Wi-Fi connect to the Outpost computer, the dashboard on port 8080 and the library on port 8081. Outpost needs the internet only to download."
            >
              <div className="topo-net">
                <span className="topo-box dashed">Internet</span>
                <span className="topo-cut">
                  <span>Needed only to download</span>
                </span>
              </div>
              <div className="topo-node paper">
                <p>
                  <strong>Outpost</strong>
                  <span>Your computer</span>
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
                <span>Your Wi-Fi</span>
              </div>
              <div className="topo-devices">
                <span className="topo-box">Phone</span>
                <span className="topo-box">Laptop</span>
                <span className="topo-box">Tablet</span>
              </div>
            </div>
            <div className="features">
              <div className="feature">
                <p className="spec">One search box // every book at once</p>
                <h3>A library you can search</h3>
                <p>
                  Wikipedia, from a {bytes(c.items.find((i) => i.id === "wikipedia_en_top_mini")?.size || 0)} pocket edition up to all{" "}
                  {bytes(wikiAll?.size || 0)} of it with pictures. Medical encyclopedias, iFixit repair guides, classic books, Khan Academy
                  lessons, textbooks and practical how-to sites. Type a question once and it looks through all of them.
                </p>
              </div>
              <div className="feature">
                <p className="spec">{regions} regions // down to the street</p>
                <h3>Street maps without a signal</h3>
                <p>
                  Pick the countries, US states, Canadian provinces or whole continents you care about. The maps go down to individual streets,
                  and a map of the whole world always comes along so you can find your way between them.
                </p>
              </div>
              <div className="feature">
                <p className="spec">Optional // runs on your computer</p>
                <h3>An AI assistant that shows its sources</h3>
                <p>
                  Ask a question in plain words. It reads your library first and answers with links to the articles it used, so you can check
                  them yourself. For common emergencies it also reads short first-aid notes from NHS, Red Cross and CDC guidance, and it
                  warns you whenever an answer gives a dose.
                </p>
              </div>
              <div className="feature">
                <p className="spec">Any browser // no app, no account</p>
                <h3>For everyone in the house</h3>
                <p>
                  Anyone on your Wi-Fi can open it. You decide whether guests read freely or need the password. Changing what is installed
                  always needs it.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="hardware">
        <div className="wrap flush">
          <div className="sec-head">
            <h2>Bring your own computer</h2>
            <p>
              There is nothing to buy from us. Outpost runs on a Mac, or on a PC running Linux, a free system. How much you can keep depends on the
              disk and memory you bring.
            </p>
          </div>
          <table className="hw">
            <thead>
              <tr>
                <th scope="col">Setup</th>
                <th scope="col">Typical computer</th>
                <th scope="col">Good for</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <th scope="row">Pocket</th>
                <td data-label="Typical computer">An old laptop or a Raspberry Pi 5, with a 256 GB drive</td>
                <td data-label="Good for">The essentials, Wikipedia without pictures and a few map regions. Skip the AI or use the smallest model.</td>
              </tr>
              <tr>
                <th scope="row">Household</th>
                <td data-label="Typical computer">A Mac mini or a used mini PC with 16 GB of memory and a 1 TB drive</td>
                <td data-label="Good for">
                  The standard collections, Wikipedia with pictures, your country&apos;s maps and a mid-size AI model that answers at talking speed.
                </td>
              </tr>
              <tr>
                <th scope="row">Basecamp</th>
                <td data-label="Typical computer">A recent Mac or PC with 32 GB of memory and a 2–4 TB drive</td>
                <td data-label="Good for">Everything: Khan Academy, all of Project Gutenberg, whole continents of maps and the larger AI models.</td>
              </tr>
            </tbody>
          </table>
          <div className="hw-notes">
            <p>
              <strong>Which computers?</strong> Macs from about 2017 on (macOS 13 or later, Intel or Apple Silicon), and PCs or Raspberry Pis
              running 64-bit Linux such as Ubuntu.
            </p>
            <p>
              <strong>Only have Windows?</strong> You can read an Outpost from Windows, like from any other device. Running one on Windows
              itself is not possible yet. A spare PC becomes an Outpost by putting Ubuntu on it, which is free; the{" "}
              <Link href="/install#windows">install guide</Link> shows how.
            </p>
            <p>
              <strong>Power cuts?</strong> A small solar panel and battery keep a mini PC (15–35 W) running. Add a Wi-Fi router and everyone
              nearby can connect.
            </p>
          </div>
        </div>
      </section>

      <section id="trust">
        <div className="wrap">
          <div className="sec-head">
            <h2>Built to be trusted</h2>
            <p>You are putting this on your own network. Here is what stops it, and us, from doing anything you did not ask for.</p>
          </div>
          <div className="trust-grid">
            <ul className="trust">
              <li>
                <h3>You get what we published</h3>
                <div>
                  <p>Every release carries a digital signature. The installer checks it and refuses anything that was changed on the way to you.</p>
                  <p className="spec">Ed25519 signatures on releases and the catalog</p>
                </div>
              </li>
              <li>
                <h3>Every file is checked</h3>
                <div>
                  <p>
                    Each download is compared with the fingerprint its publisher lists before it is used. A dropped connection picks up where
                    it stopped.
                  </p>
                  <p className="spec">SHA-256 on every archive</p>
                </div>
              </li>
              <li>
                <h3>Yours from the first minute</h3>
                <div>
                  <p>
                    To become the owner you need a one-time code shown on the computer itself, so a neighbour on the same Wi-Fi cannot get there
                    first.
                  </p>
                  <p className="spec">Setup code, then a password only you know</p>
                </div>
              </li>
              <li>
                <h3>Nothing reports back</h3>
                <div>
                  <p>
                    No accounts, no analytics, no tracking. Outpost goes online only to download what you picked and, if you allow it, to check
                    for updates.
                  </p>
                  <p className="spec">No telemetry, by design</p>
                </div>
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
                  <dd>Stay on your Outpost</dd>
                </div>
                <div>
                  <dt>Reading history</dt>
                  <dd>Stays on your Outpost</dd>
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
              <span>For the technical: release signing key, Ed25519</span>
              <code>{fingerprint}</code>
              <Link href="/docs#security">How Outpost is secured</Link>
            </p>
          )}
        </div>
      </section>

      <section className="cta paper">
        <div className="wrap">
          <h2>Start with the computer in your closet</h2>
          <p className="lede">A Mac or a Linux PC, about ten minutes of your time, and nothing to pay. The guide walks you through every step.</p>
          <div className="row">
            <Link href="/install" className="btn">
              Open the install guide
            </Link>
            <Link href="/catalog" className="btn ghost">
              See everything you can keep
            </Link>
          </div>
          <p className="hero-or">Or paste this into Terminal:</p>
          <CopyCommand command={INSTALL} />
        </div>
      </section>
    </>
  );
}
