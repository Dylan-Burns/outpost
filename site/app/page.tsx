import Link from "next/link";
import Contours from "./components/Contours";
import CopyCommand from "./components/CopyCommand";
import Planner from "./components/Planner";
import { bytes, loadCatalog, plannerData } from "@/lib/catalog";

const INSTALL = "curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash";

export default function Home() {
  const c = loadCatalog();
  const archives = c.items.filter((i) => i.kind === "zim").length;
  const regions = c.items.filter((i) => i.kind === "map-region").length;
  const wikiAll = c.items.find((i) => i.id === "wikipedia_en_all_maxi");
  return (
    <>
      <section className="hero">
        <Contours />
        <div className="wrap">
          <h1>The library that keeps working when the internet doesn&apos;t.</h1>
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
      </section>

      <section id="how">
        <div className="wrap">
          <h2>From spare computer to field library in an afternoon</h2>
          <ol className="steps">
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

      <section id="plan">
        <div className="wrap">
          <h2>Plan your library</h2>
          <p className="lede">
            These are the same choices the setup wizard offers, with today&apos;s file sizes: {archives} curated archives, {regions} map regions and
            AI models from 1.4 to 8 GB. Change your mind later; the node adds and removes content without starting over.
          </p>
          <Planner data={plannerData()} />
        </div>
      </section>

      <section id="inside">
        <div className="wrap">
          <h2>What runs on your node</h2>
          <div className="features">
            <div className="feature">
              <h3>A library you can search</h3>
              <p>
                Wikipedia from a {bytes(c.items.find((i) => i.id === "wikipedia_en_top_mini")?.size || 0)} pocket edition up to the full{" "}
                {bytes(wikiAll?.size || 0)} with pictures, plus WikiMed, iFixit, Project Gutenberg, Khan Academy, LibreTexts and practical Stack
                Exchange sites. Full-text search runs across all of it at once.
              </p>
            </div>
            <div className="feature">
              <h3>Street maps without a signal</h3>
              <p>
                Choose countries, US states, Canadian provinces or whole continents, cut at street-level detail from the current OpenStreetMap planet
                build. A world overview is always included so you can pan between them.
              </p>
            </div>
            <div className="feature">
              <h3>An assistant that cites its sources</h3>
              <p>
                Optional local AI through Ollama. It searches your library first and answers with numbered links to the articles it used, so you can
                read the original. On medical and electrical questions it points you to the text instead of paraphrasing doses or steps.
              </p>
            </div>
            <div className="feature">
              <h3>Made for the whole household</h3>
              <p>
                Anyone on your Wi-Fi opens it in a browser, no app needed. You decide whether visitors can read freely or need the admin password.
                Content management always needs it.
              </p>
            </div>
          </div>
        </div>
      </section>

      <section id="hardware">
        <div className="wrap">
          <h2>Bring your own hardware</h2>
          <p className="lede">Outpost has no box to buy. Anything that runs 64-bit Linux works; what you get depends on what you bring.</p>
          <div className="table-scroll">
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
                  <td>Raspberry Pi 5 (8 GB) or an old laptop, 256 GB SSD or USB drive</td>
                  <td>Essential collections, Wikipedia text, a few map regions. Skip the AI or use the 1.7B model.</td>
                </tr>
                <tr>
                  <th scope="row">Household</th>
                  <td>Used mini PC (Intel 8th gen or Ryzen 5, 16 GB RAM), 1 TB SSD</td>
                  <td>Standard collections, Wikipedia with pictures, your country&apos;s maps, a 3–4B model at conversational speed.</td>
                </tr>
                <tr>
                  <th scope="row">Basecamp</th>
                  <td>Recent mini PC or desktop with 32 GB RAM or a GPU, 2–4 TB SSD</td>
                  <td>Everything: Khan Academy, all of Gutenberg, continents of maps, 8–12B models for the whole family.</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p className="quiet" style={{ marginTop: "1.2rem" }}>
            A small solar panel and battery keep a mini PC (15–35 W) running through an outage. Pair it with a Wi-Fi access point and everyone nearby
            can connect.
          </p>
        </div>
      </section>

      <section id="trust">
        <div className="wrap">
          <h2>Built to be trusted offline</h2>
          <ul className="trust">
            <li>
              <strong>Signed from end to end</strong>
              Releases and the content catalog are signed with an Ed25519 key; the installer and the node refuse anything that does not verify.
            </li>
            <li>
              <strong>Every file checked</strong>
              Each archive is compared with the SHA-256 its publisher lists before it is used. Interrupted downloads resume instead of restarting.
            </li>
            <li>
              <strong>Yours from the first minute</strong>
              A one-time setup code printed on the machine claims the node, so a neighbour on the same Wi-Fi cannot get there first.
            </li>
            <li>
              <strong>Nothing phones home</strong>
              No accounts, analytics or telemetry. The node contacts the internet only to download what you picked and, if you allow it, to check
              for updates.
            </li>
          </ul>
          <p style={{ marginTop: "1.5rem" }}>
            <Link href="/docs#security">How Outpost is secured</Link>
          </p>
        </div>
      </section>

      <section className="cta">
        <div className="wrap">
          <h2>Start with the computer in your closet</h2>
          <p className="lede">Ubuntu, Debian, Fedora or Raspberry Pi OS, 64-bit. Add the AI assistant with one more flag.</p>
          <CopyCommand command={INSTALL} />
          <p style={{ marginTop: "1rem" }}>
            With the AI assistant: <code>… | sudo bash -s -- --with-ai</code> · <Link href="/install">Full install guide</Link>
          </p>
        </div>
      </section>
    </>
  );
}
