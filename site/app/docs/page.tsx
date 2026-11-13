import type { Metadata } from "next";
import Link from "next/link";
import { releaseKeyFingerprint } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Documentation",
  description: "How Outpost works, how it is secured, and answers to common questions.",
};

export default function Docs() {
  return (
    <>
      <div className="page-head">
        <div className="wrap">
          <h1 className="long">Documentation</h1>
          <p className="lede">How Outpost works, who can use it, how it is secured, and answers to common questions.</p>
        </div>
      </div>
      <section>
        <div className="wrap doc">
          <nav className="index" aria-label="On this page">
            <ol>
              <li><a href="#how">How it works</a></li>
              <li><a href="#access">Access</a></li>
              <li><a href="#security">Security</a></li>
              <li><a href="#commands">Commands</a></li>
              <li><a href="#faq">Questions</a></li>
              <li><a href="#licences">Licences</a></li>
            </ol>
          </nav>
          <article className="prose">
            <h2 id="how">How it works</h2>
            <p>
              Outpost is one program, <code>outpost</code>, that runs as a system service. It serves the dashboard on port 8080 and the library on
              port 8081, and it manages three helpers:
            </p>
            <ul>
              <li><strong>kiwix-serve</strong> reads ZIM archives, the compressed format Kiwix uses for Wikipedia and hundreds of other sites, and provides their full-text search.</li>
              <li><strong>pmtiles</strong> cuts map regions out of the Protomaps planet build, which is made from OpenStreetMap, so you download only the places you chose.</li>
              <li><strong>Ollama</strong>, if you installed the assistant, runs AI models on the machine&apos;s processor or graphics card.</li>
            </ul>
            <p>
              What you can install comes from the <Link href="/catalog">content catalog</Link>, a signed list that names each file, its exact size and its
              SHA-256. Your node fetches a newer catalog from this website when it is online, checks the signature, and offers newer editions of your
              archives. A copy of the catalog is built into the program, so a node that has never been online can still run the setup wizard.
            </p>
            <p>
              Content lives in <code>/var/lib/outpost</code> unless you chose another folder: archives under <code>content/zim</code>, maps under{" "}
              <code>content/maps</code>. AI models live in Ollama&apos;s own folder.
            </p>

            <h2 id="access">Who can use it</h2>
            <p>
              By default anyone on your network can read the library, search, view maps and ask the assistant. Only the administrator can change
              content or settings. In Settings you can require the admin password for everything, or keep reading open and close only the assistant.
            </p>
            <p>
              Outpost serves plain HTTP on your local network, like most home devices. Do not forward its ports to the internet. If you need access from
              outside, use a VPN such as WireGuard or Tailscale.
            </p>

            <h2 id="security">Security</h2>
            <dl>
              <dt>Signed releases and catalog</dt>
              <dd>
                Release checksums and the catalog are signed with an Ed25519 key in OpenSSH&apos;s signature format. The installer verifies releases with{" "}
                <code>ssh-keygen -Y verify</code>; the node verifies catalogs itself and never accepts an older catalog than the one it has. The
                key&apos;s fingerprint is <code>{releaseKeyFingerprint()}</code>.
              </dd>
              <dt>Verified downloads</dt>
              <dd>Every archive must match the SHA-256 published by Kiwix before it is used. Redirects to plain HTTP are refused. kiwix-tools and pmtiles are pinned by checksum in the installer.</dd>
              <dt>Claiming a new node</dt>
              <dd>A fresh node accepts an admin password only together with the setup code printed on the machine, so nobody else on the network can claim it first.</dd>
              <dt>Passwords and sessions</dt>
              <dd>The admin password is stored as an argon2id hash. Sessions use random 256-bit cookies that scripts cannot read, stored only as hashes. Every change requires a per-session token and a same-origin request. Sign-in attempts are limited per client, with an overall budget against guessing from many addresses.</dd>
              <dt>Archives kept apart from the dashboard</dt>
              <dd>The library runs on its own port, which browsers treat as a separate origin, so scripts inside an archive cannot read the dashboard. The dashboard sends a strict content security policy and refuses to be framed.</dd>
              <dt>Map data</dt>
              <dd>Map regions are cut on demand from the Protomaps planet build named in the signed catalog, over HTTPS. Unlike archives, extracted regions have no published checksum, so their integrity rests on that HTTPS connection.</dd>
              <dt>Other accounts on the same machine</dt>
              <dd>kiwix-serve listens on a loopback port of the node itself. In private mode, people with their own login on that machine can still read the library through it; the setting protects the network, not other local users.</dd>
              <dt>A locked-down service</dt>
              <dd>Outpost runs as its own unprivileged account under systemd with a read-only system, no new privileges, no capabilities and write access only to its data folder.</dd>
            </dl>
            <p>Found a problem? Please report it privately to the maintainers rather than in public.</p>

            <h2 id="commands">Commands</h2>
            <pre>{`sudo outpost setup-code     # show the one-time code for a new node
      sudo outpost reset-admin    # forget the admin password, print a new setup code
      outpost version             # installed version
      sudo systemctl restart outpost
      journalctl -u outpost -f    # follow the logs
      sudo nano /etc/outpost/outpost.env   # ports and data folder`}</pre>

            <h2 id="faq">Questions</h2>
            <dl>
              <dt>Does it need the internet?</dt>
              <dd>Only to download. Once your content is on the node, everything works with no connection at all.</dd>
              <dt>What does it cost?</dt>
              <dd>Nothing. You bring the hardware; Outpost and the content are free.</dd>
              <dt>Can I use a Raspberry Pi?</dt>
              <dd>Yes, a Pi 4 or 5 with a 64-bit OS. Put content on an SSD rather than a memory card. The AI assistant is slow on a Pi; the library and maps are fine.</dd>
              <dt>Windows or macOS?</dt>
              <dd>The installer targets Linux. On Windows you can run it inside WSL2 with systemd enabled; on a Mac, use a Linux virtual machine. A spare PC running Linux is the most reliable choice.</dd>
              <dt>How is this different from Kiwix?</dt>
              <dd>Kiwix makes the archives and the reader that Outpost uses. Outpost adds the curated catalog with tiers, the setup wizard, maps, the assistant, verified background downloads and a dashboard for the whole household.</dd>
              <dt>Is the AI reliable?</dt>
              <dd>No language model is. Outpost&apos;s assistant searches your library first and cites what it used; read the sources, especially for medicine, and treat answers without sources with caution.</dd>
              <dt>Can I add my own files?</dt>
              <dd>Not yet. Outpost installs content from its signed catalog so every file can be verified.</dd>
            </dl>

            <h2 id="licences">Licences and credits</h2>
            <p>
              Archives keep their publishers&apos; licences, listed for each item in the <Link href="/catalog">catalog</Link>; several (iFixit, TED, Khan
              Academy) allow only non-commercial use. Map data © OpenStreetMap contributors, available under the Open Database Licence; tiles by Protomaps.
              Fonts: Noto Sans (SIL Open Font Licence). kiwix-tools is GPL-3.0 software by the Kiwix project; pmtiles is BSD-3-Clause; Ollama is MIT.
              AI models carry their makers&apos; licences.
            </p>
          </article>
        </div>
      </section>
    </>
  );
}
