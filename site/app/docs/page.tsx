import type { Metadata } from "next";
import Link from "next/link";
import { releaseKeyFingerprint } from "@/lib/catalog";
import { REPO } from "@/lib/site";

export const metadata: Metadata = {
  title: "Help and answers",
  description: "What Outpost is in plain words, answers to common questions, and the technical detail for those who want it.",
};

const COMMANDS = `sudo /opt/outpost/cli/outpost setup-code    # show the one-time setup code
sudo /opt/outpost/cli/outpost reset-admin   # forget the password, show a new code
/opt/outpost/cli/outpost version            # which release is installed
sudo nano /etc/outpost/outpost.env          # ports and content folder

# Linux
sudo systemctl restart outpost
journalctl -u outpost -f                    # follow the logs

# Mac
sudo launchctl kickstart -k system/org.outpost-offline.node
tail -f /var/log/outpost.log                # follow the logs`;

export default function Docs() {
  return (
    <>
      <div className="page-head">
        <div className="wrap">
          <h1>Help and answers</h1>
          <p className="lede">
            Start at the top if you are new. The further down the page you go, the more technical it gets, and none of it is needed to use
            Outpost.
          </p>
        </div>
      </div>
      <section>
        <div className="wrap doc">
          <nav className="index" aria-label="On this page">
            <ol>
              <li><a href="#plain">In plain words</a></li>
              <li><a href="#faq">Common questions</a></li>
              <li><a href="#access">Who can use it</a></li>
              <li><a href="#how">How it works inside</a></li>
              <li><a href="#security">Security</a></li>
              <li><a href="#commands">Commands</a></li>
              <li><a href="#licences">Licences</a></li>
            </ol>
          </nav>
          <article className="prose">
            <h2 id="plain">In plain words</h2>
            <p>
              Outpost is a free program that turns a computer you own into a small library for your home. You choose what it keeps: Wikipedia,
              medical guides, repair manuals, school lessons, street maps. It downloads them once and stores them on that computer.
            </p>
            <p>
              From then on, any phone, tablet or laptop on your Wi-Fi can read and search all of it in a web browser, whether or not the internet
              is working. If you want, an AI assistant answers questions using that same library and shows you where each answer came from.
            </p>
            <p>
              People use it to be ready for storms and outages, in cabins, boats and RVs, in classrooms with a slow connection, and simply to
              look things up without being tracked. <Link href="/install">Setting it up</Link> takes about ten minutes.
            </p>

            <h2 id="faq">Common questions</h2>
            <dl>
              <dt>Do I need to be good with computers?</dt>
              <dd>
                No. You paste one line into an app called Terminal, once. After that everything is an ordinary web page: you tick what you want
                and watch it download. The <Link href="/install">install guide</Link> writes out every step.
              </dd>
              <dt>Does it need the internet?</dt>
              <dd>Only to download. Once your content is on the Outpost computer, everything works with no connection at all.</dd>
              <dt>What does it cost?</dt>
              <dd>Nothing. You bring the computer; Outpost and the content are free.</dd>
              <dt>Which computers can run it?</dt>
              <dd>
                Macs from about 2017 on (macOS 13 or later, Intel or Apple Silicon), and PCs or Raspberry Pis running 64-bit Linux. It cannot be
                installed on Windows yet; the <Link href="/install#windows">install guide</Link> explains the options for a Windows PC.
              </dd>
              <dt>Which devices can read it?</dt>
              <dd>
                Anything with a web browser on the same Wi-Fi: iPhones, Android phones, tablets, Windows PCs, Macs, Chromebooks. Nothing has to be
                installed on them.
              </dd>
              <dt>How much disk space do I need?</dt>
              <dd>
                From a few gigabytes for the essentials to several terabytes for everything. The <Link href="/#plan">planner</Link> adds it up as
                you choose and suggests a drive size.
              </dd>
              <dt>Can I reach it when I am away from home?</dt>
              <dd>
                Not by itself: it answers on your own network only, on purpose. If you want it from outside, use a private VPN such as Tailscale
                or WireGuard. Do not open its ports to the internet.
              </dd>
              <dt>Can I use a Raspberry Pi?</dt>
              <dd>
                Yes, a Pi 4 or 5 with a 64-bit system. Put the content on an SSD rather than a memory card. The AI assistant is slow on a Pi; the
                library and maps are fine.
              </dd>
              <dt>Is the AI reliable?</dt>
              <dd>
                No AI model is. Outpost&apos;s assistant searches your library first and shows what it used. For common emergencies it also
                reads Outpost&apos;s own first-aid notes, short summaries of NHS, Red Cross and CDC guidance that open beside the answer, and it
                warns you when an answer gives a dose. In our tests on emergency questions, Llama 3.2 3B and Phi-4 mini answered more accurately
                than the smallest model. Read the sources, especially for medicine, and be wary of any answer that comes without them.
              </dd>
              <dt>Can I add my own files?</dt>
              <dd>Not yet. Outpost installs content only from its signed catalog, so that every file can be checked.</dd>
              <dt>How is this different from Kiwix?</dt>
              <dd>
                Kiwix makes the libraries and the reader that Outpost uses. Outpost adds the catalog with three sizes per subject, the setup
                pages, maps, the assistant, checked downloads that run in the background, and one dashboard for the whole household.
              </dd>
            </dl>

            <h2 id="access">Who can use it</h2>
            <p>
              To begin with, anyone on your Wi-Fi can read the library, search, look at maps and ask the assistant. Only you, the owner, can
              change what is installed or how it is set up. In Settings you can ask for the password for everything, or keep reading open and
              close only the assistant.
            </p>
            <p>
              Outpost uses plain HTTP inside your home network, as most home devices do. That is fine at home and not fine on the open internet,
              which is why its ports should never be forwarded there.
            </p>

            <h2 id="how">How it works inside</h2>
            <p>
              Outpost is one program, <code>outpost</code>, that the system starts at boot. It serves the dashboard on port 8080 and the library
              on port 8081, and it looks after three helpers:
            </p>
            <ul>
              <li>
                <strong>kiwix-serve</strong> reads ZIM archives, the compressed format Kiwix uses for Wikipedia and hundreds of other sites, and
                provides the full-text search.
              </li>
              <li>
                <strong>pmtiles</strong> cuts map regions out of the Protomaps planet build, which is made from OpenStreetMap, so you download
                only the places you chose.
              </li>
              <li>
                <strong>Ollama</strong>, if you set up the assistant, runs AI models on the machine&apos;s processor or graphics card.
              </li>
            </ul>
            <p>
              What you can install comes from the <Link href="/catalog">content catalog</Link>, a signed list that names each file, its exact size
              and its SHA-256. Outpost fetches a newer catalog from this website when it is online, checks the signature, and offers newer
              editions of what you have. A copy of the catalog is built into the program, so an Outpost that has never been online can still show
              the setup pages.
            </p>
            <p>
              Content lives in <code>/var/lib/outpost</code> unless you chose another folder: archives under <code>content/zim</code>, maps under{" "}
              <code>content/maps</code>. AI models live in Ollama&apos;s own folder. On a Mac the content folder is left out of Time Machine
              backups, since everything in it can be downloaded again.
            </p>

            <h2 id="security">Security</h2>
            <dl>
              <dt>Source you can read</dt>
              <dd>
                Everything Outpost runs on your computer is published: the installer, the program, the dashboard and the script that builds
                each release. Read it, or build it yourself, at <a href={REPO} rel="noopener">github.com/Dylan-Burns/outpost</a>. Releases
                are built so that the same source gives the same file, byte for byte, so anyone can check that a download matches the code.
              </dd>
              <dt>Signed releases and catalog</dt>
              <dd>
                Release checksums and the catalog are signed with an Ed25519 key in OpenSSH&apos;s signature format. The installer verifies
                releases with <code>ssh-keygen -Y verify</code>; Outpost verifies catalogs itself and never accepts an older catalog than the
                one it has. The key&apos;s fingerprint is <code>{releaseKeyFingerprint()}</code>.
              </dd>
              <dt>Verified downloads</dt>
              <dd>
                Every archive must match the SHA-256 published by Kiwix before it is used. Redirects to plain HTTP are refused. kiwix-tools and
                pmtiles are pinned by checksum in the installer, separately for each system and processor.
              </dd>
              <dt>Claiming a new Outpost</dt>
              <dd>
                A fresh install accepts an owner&apos;s password only together with the setup code printed on the machine, so nobody else on the
                network can claim it first. On a Mac the installer also hands the code to the browser of the person sitting at that Mac, in the
                part of the address after the “#”, which a browser keeps to itself and never sends anywhere.
              </dd>
              <dt>Passwords and sessions</dt>
              <dd>
                The password is stored as an argon2id hash. Sessions use random 256-bit cookies that scripts cannot read, stored only as hashes.
                Every change requires a per-session token and a same-origin request. Sign-in attempts are limited per client, with an overall
                budget against guessing from many addresses.
              </dd>
              <dt>Archives kept apart from the dashboard</dt>
              <dd>
                The library runs on its own port, which browsers treat as a separate origin, so scripts inside an archive cannot read the
                dashboard. The dashboard sends a strict content security policy and refuses to be framed.
              </dd>
              <dt>Map data</dt>
              <dd>
                Map regions are cut on demand from the Protomaps planet build named in the signed catalog, over HTTPS. Unlike archives,
                extracted regions have no published checksum, so their integrity rests on that HTTPS connection.
              </dd>
              <dt>Other accounts on the same machine</dt>
              <dd>
                kiwix-serve listens on a loopback port of the machine itself. In private mode, people with their own login on that machine can
                still read the library through it; the setting protects the network, not other local users.
              </dd>
              <dt>A service with few rights</dt>
              <dd>
                Outpost never runs as an administrator. On Linux it has its own account under systemd, with a read-only system, no new
                privileges, no capabilities and write access only to its content folder. On a Mac it has its own hidden account under launchd,
                which owns its content folder and its log and nothing else; macOS offers no equivalent of the further systemd restrictions.
              </dd>
              <dt>The command you run as an administrator</dt>
              <dd>
                The <code>outpost</code> command lives in <code>/opt/outpost/cli</code>, which only an administrator can change, and the program
                files are owned the same way. A short name in <code>/usr/local/bin</code> is added only when that folder is equally protected.
                On many Macs it is not, so this guide always writes the command out in full.
              </dd>
            </dl>
            <p>Found a problem? Please report it privately to the maintainers rather than in public.</p>

            <h2 id="commands">Commands</h2>
            <p>For the Terminal on the Outpost computer.</p>
            <pre>{COMMANDS}</pre>

            <h2 id="licences">Licences and credits</h2>
            <p>
              Archives keep their publishers&apos; licences, listed for each item in the <Link href="/catalog">catalog</Link>; several (iFixit, TED,
              Khan Academy) allow only non-commercial use. Map data © OpenStreetMap contributors, available under the Open Database Licence;
              tiles by Protomaps. Fonts: Noto Sans on the maps, Archivo and Martian Mono in the dashboard (all SIL Open Font Licence). kiwix-tools is GPL-3.0 software by the Kiwix project; pmtiles is
              BSD-3-Clause; Ollama is MIT. AI models carry their makers&apos; licences.
            </p>
          </article>
        </div>
      </section>
    </>
  );
}
