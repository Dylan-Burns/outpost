import type { Metadata } from "next";
import Link from "next/link";
import CopyCommand from "../components/CopyCommand";
import { installerDigest, loadRelease } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Install",
  description: "Install Outpost on a Mac or a Linux PC with one line, then choose what to keep. A step-by-step guide for everyone.",
};

const INSTALL = "curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash";
const CLI = "sudo /opt/outpost/cli/outpost";

export default function Install() {
  const rel = loadRelease();
  return (
    <>
      <div className="page-head">
        <div className="wrap">
          <h1>Install Outpost</h1>
          <p className="lede">
            About ten minutes of your time, then the downloads run on their own. You do not need to be technical: every step is written out
            below.
          </p>
          <dl className="titleblock">
            <div>
              <dt>Runs on</dt>
              <dd>
                Mac + Linux<small>macOS 13 or later, 64-bit Linux</small>
              </dd>
            </div>
            <div>
              <dt>Your time</dt>
              <dd>
                10 min<small>Downloads carry on alone</small>
              </dd>
            </div>
            <div>
              <dt>Cost</dt>
              <dd>
                Free<small>Nothing to buy or subscribe to</small>
              </dd>
            </div>
            <div>
              <dt>Current release</dt>
              <dd>
                {rel.version}
                <small>{rel.date ? `Signed, ${rel.date}` : "Signed"}</small>
              </dd>
            </div>
          </dl>
        </div>
      </div>
      <section>
        <div className="wrap doc">
          <nav className="index" aria-label="On this page">
            <ol>
              <li><a href="#before">What you need</a></li>
              <li><a href="#terminal">1. Open Terminal</a></li>
              <li><a href="#run">2. Paste one line</a></li>
              <li><a href="#claim">3. Choose your content</a></li>
              <li><a href="#offline">4. Go offline</a></li>
              <li><a href="#windows">Windows PCs</a></li>
              <li><a href="#update">Update or remove</a></li>
              <li><a href="#trouble">If something goes wrong</a></li>
            </ol>
          </nav>
          <article className="prose">
            <h2 id="before">What you need</h2>
            <ul>
              <li>
                <strong>A computer that can stay switched on.</strong> A Mac from about 2017 or newer (macOS 13 Ventura or later, Intel or Apple
                Silicon), or a PC or Raspberry Pi running 64-bit Linux: Ubuntu 22.04 or later, Debian 12 or later, Fedora, or Raspberry Pi OS.
              </li>
              <li>
                <strong>Enough disk space</strong> for what you want to keep. The <Link href="/#plan">planner</Link> tells you how much. An
                external drive works too.
              </li>
              <li>
                <strong>Internet for the first download.</strong> After that, none is needed.
              </li>
              <li>
                <strong>The password for that computer.</strong> Installing software asks for it once.
              </li>
            </ul>
            <p>
              Other devices need nothing installed. Phones, tablets and computers of any kind, Windows included, open Outpost in a web browser.
            </p>

            <h2 id="terminal">1. Open Terminal</h2>
            <p>Terminal is an app that comes with the computer. It lets you type an instruction instead of clicking.</p>
            <dl>
              <dt>On a Mac</dt>
              <dd>
                Hold <strong>Command</strong> and press the <strong>space bar</strong>, type <code>Terminal</code>, then press{" "}
                <strong>Return</strong>.
              </dd>
              <dt>On Linux</dt>
              <dd>
                Press <strong>Ctrl + Alt + T</strong>, or pick Terminal from the applications menu. On a machine with no screen, connect to it
                with SSH.
              </dd>
            </dl>

            <h2 id="run">2. Paste one line</h2>
            <p>
              Copy this line, paste it into Terminal and press <strong>Return</strong>. It is the same on a Mac and on Linux.
            </p>
            <CopyCommand command={INSTALL} />
            <p>
              It asks for the computer&apos;s password. Nothing shows while you type it; that is normal. Type it and press <strong>Return</strong>.
            </p>
            <p>
              The installer downloads Outpost and two helpers, checks that each is the genuine article, and sets Outpost to start by itself
              whenever the computer is on. When it finishes it shows an address to open and a one-time setup code:
            </p>
            <pre>{`Outpost ${rel.version} is running.

  Open the dashboard:  http://192.168.1.40:8080
  On this machine:     http://localhost:8080

  Setup code:          K7QF-M2XP-9HTD`}</pre>

            <h3>Want the AI assistant too?</h3>
            <p>
              It is optional, and it runs on your own computer through a free program called Ollama. A machine with 8 GB of memory or more is
              best.
            </p>
            <dl>
              <dt>On Linux</dt>
              <dd>Use this line instead of the one above. It installs Ollama from ollama.com for you.</dd>
            </dl>
            <CopyCommand command={`${INSTALL} -s -- --with-ai`} />
            <dl>
              <dt>On a Mac</dt>
              <dd>
                Download the Ollama app from <a href="https://ollama.com/download">ollama.com/download</a> and open it once. Outpost finds it
                by itself, and the assistant appears in the dashboard.
              </dd>
            </dl>

            <h3>For the technical</h3>
            <dl>
              <dt><code>--data-dir /mnt/library</code></dt>
              <dd>Keep content somewhere else, such as a large external drive. The default is <code>/var/lib/outpost</code>.</dd>
              <dt><code>--port 8080</code> and <code>--library-port 8081</code></dt>
              <dd>Change the dashboard and library ports if something else uses them.</dd>
              <dt><code>--version 1.1.0</code></dt>
              <dd>Install a particular release instead of the latest.</dd>
              <dt><code>--no-open</code></dt>
              <dd>On a Mac, do not open the setup page in the browser when the install finishes.</dd>
              <dt><code>--no-firewall</code></dt>
              <dd>Leave the firewall alone. Otherwise the installer opens the two ports in ufw or firewalld, or allows Outpost in the macOS firewall, when one is on.</dd>
            </dl>
            <p>
              On Linux, Outpost runs as a systemd service under its own <code>outpost</code> account. On a Mac it is a launchd job under a hidden{" "}
              <code>_outpost</code> account. Either way it starts at boot, before anyone signs in, and can write only to its content folder.
            </p>
            <p>
              Prefer to read before you run? <a href="/install.sh">Download install.sh</a>, look it over, then run{" "}
              <code>sudo bash install.sh</code>. The copy this site serves today has this SHA-256; compare it with{" "}
              <code>shasum -a 256 install.sh</code> on a Mac or <code>sha256sum install.sh</code> on Linux.
            </p>
            <pre>{installerDigest()}</pre>

            <h2 id="claim">3. Choose your content</h2>
            <p>From here on there is no more Terminal. Everything happens on ordinary web pages.</p>
            <ol>
              <li>
                <strong>Open the setup page.</strong> On a Mac it opens in your browser by itself when the installer finishes, with the setup
                code already filled in. On Linux, or from another device on the same Wi-Fi, open the address the installer showed and type the
                setup code.
              </li>
              <li>
                Choose a password and select <strong>Start setup</strong>. This makes you the owner.
              </li>
              <li>Pick what you want it to be: a library, maps, an assistant, or all three.</li>
              <li>Choose how much Wikipedia, a size for each subject, your map regions and any AI models. A bar shows what fits on the drive.</li>
              <li>
                Select <strong>Start downloading</strong>. The Downloads page shows every file as it arrives: what it is doing, how far it has
                got, how fast, and how long is left. The smallest come first, so there is something to read within minutes.
              </li>
            </ol>
            <p>You can close the page, or the laptop you opened it on. The Outpost computer keeps downloading by itself.</p>
            <p>
              Lost the setup code? In Terminal on the Outpost computer, run <code>{CLI} setup-code</code>.
            </p>

            <h2 id="offline">4. Go offline</h2>
            <p>
              When the Downloads page says <strong>Everything is ready</strong>, you can unplug the internet. Everyone on your Wi-Fi opens the
              same address. Tell them to save it as a bookmark.
            </p>
            <p>
              Outpost starts by itself after a restart or a power cut. If a download was interrupted, it carries on from where it stopped the
              next time there is internet.
            </p>
            <p>
              <strong>Keep the computer awake.</strong> A sleeping computer cannot answer. On a Mac, open System Settings and turn off automatic
              sleep (under Energy on a desktop Mac, or Battery then Options on a laptop). On a Linux desktop, turn off automatic suspend in the
              power settings.
            </p>

            <h2 id="windows">Windows PCs</h2>
            <p>
              Any Windows computer can <em>use</em> an Outpost: open its address in a browser. What Windows cannot do yet is <em>run</em> one.
              There is no Outpost installer for Windows today.
            </p>
            <p>If the spare computer you have in mind is a Windows PC, there are two good routes:</p>
            <ul>
              <li>
                <strong>Put Ubuntu on it.</strong> Ubuntu is a free system that replaces Windows on that machine. Download it from{" "}
                <a href="https://ubuntu.com/download/desktop">ubuntu.com</a>, write it to a USB stick with balenaEtcher, start the PC from the
                stick and follow the installer. This erases the PC, so copy anything you want to keep first. Then come back to step 1.
              </li>
              <li>
                <strong>Use a Mac or a small Linux computer instead.</strong> A used mini PC or a Raspberry Pi 5 costs little and uses very little
                power.
              </li>
            </ul>

            <h2 id="update">Update or remove</h2>
            <dl>
              <dt>Update</dt>
              <dd>
                Paste the install line again. It replaces the program, keeps your content and settings, and restarts Outpost. Newer editions of
                your libraries show up in the dashboard under Content as “Update all”.
              </dd>
              <dt>Remove Outpost, keep the content</dt>
              <dd><code>{INSTALL} -s -- --uninstall</code></dd>
              <dt>Remove everything, content included</dt>
              <dd><code>{INSTALL} -s -- --uninstall --purge</code></dd>
            </dl>

            <h2 id="trouble">If something goes wrong</h2>
            <dl>
              <dt>The address does not open from my phone</dt>
              <dd>
                Check that the phone is on the same Wi-Fi as the Outpost computer and that the computer is awake. On a Mac whose firewall is set
                to “block all incoming connections”, turn that option off under System Settings, Network, Firewall, Options.
              </dd>
              <dt>I forgot the password</dt>
              <dd>
                On the Outpost computer, run <code>{CLI} reset-admin</code>. It shows a new setup code. Your content stays.
              </dd>
              <dt>A download failed</dt>
              <dd>
                Open Downloads and select Retry. A dropped connection retries by itself. If Outpost says a file did not match, it was not the
                file the catalog names, and Outpost refuses to use it.
              </dd>
              <dt>The installer says the command was not put in /usr/local/bin</dt>
              <dd>
                On some Macs that folder can be changed without a password, so Outpost keeps its command in a folder that cannot. Use the full
                name shown, <code>{CLI}</code>.
              </dd>
              <dt>Where are the logs?</dt>
              <dd>
                Linux: <code>journalctl -u outpost -f</code>. Mac: <code>tail -f /var/log/outpost.log</code>.
              </dd>
            </dl>
          </article>
        </div>
      </section>
    </>
  );
}
