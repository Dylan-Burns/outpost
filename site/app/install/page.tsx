import type { Metadata } from "next";
import Link from "next/link";
import CopyCommand from "../components/CopyCommand";
import { loadRelease } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Install",
  description: "Install Outpost on a Linux computer in one command, then choose your content in the setup wizard.",
};

const INSTALL = "curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash";

export default function Install() {
  const rel = loadRelease();
  return (
    <div className="wrap narrow prose">
      <div className="page-head">
        <h1>Install Outpost</h1>
        <p className="lede">
          About ten minutes of your time, then the downloads run on their own. Current release: {rel.version}
          {rel.date ? `, ${rel.date}` : ""}.
        </p>
      </div>

      <h2 id="before">Before you start</h2>
      <ul>
        <li>A 64-bit computer (Intel/AMD or ARM, such as a Raspberry Pi 4 or 5) running Ubuntu 22.04 or later, Debian 12 or later, Fedora, or Raspberry Pi OS 64-bit.</li>
        <li>Enough disk for what you want to keep. The <Link href="/#plan">planner</Link> tells you how much. An external USB drive works too.</li>
        <li>An internet connection for the downloads. After that, none is needed.</li>
        <li>A user that can run <code>sudo</code>.</li>
      </ul>
      <p>No spare Linux machine? Install Ubuntu Desktop or Ubuntu Server on an old laptop or a mini PC first: write the image to a USB stick with balenaEtcher or Raspberry Pi Imager, boot from it and follow the installer.</p>

      <h2 id="run">1. Run the installer</h2>
      <p>Open a terminal on the machine (or connect with SSH) and run:</p>
      <CopyCommand command={INSTALL} />
      <p>To add the optional AI assistant, which installs Ollama from ollama.com:</p>
      <CopyCommand command={`${INSTALL} -s -- --with-ai`} />
      <p>
        The installer downloads Outpost, kiwix-serve and the pmtiles tool, checks the release signature and every checksum, creates an{" "}
        <code>outpost</code> service account and starts the service. At the end it prints the dashboard address and a one-time setup code:
      </p>
      <pre>{`Outpost 1.0.0 is running.

  Open the dashboard:  http://192.168.1.40:8080
  On this machine:     http://localhost:8080

  Setup code:          K7QF-M2XP-9HTD`}</pre>

      <h3>Useful options</h3>
      <dl>
        <dt>--data-dir /mnt/library</dt>
        <dd>Keep content on another disk, such as a large USB drive mounted at boot.</dd>
        <dt>--port 8080 and --library-port 8081</dt>
        <dd>Change the dashboard and library ports if something else uses them.</dd>
        <dt>--uninstall</dt>
        <dd>Remove Outpost but keep your content. Add --purge to delete content too.</dd>
      </dl>
      <p>
        Prefer to read before you run? <a href="/install.sh">Download install.sh</a>, look it over, then run <code>sudo bash install.sh</code>.
      </p>

      <h2 id="claim">2. Claim your node and choose content</h2>
      <ol>
        <li>Open the dashboard address from any device on the same network.</li>
        <li>Enter the setup code and choose an admin password. Lost the code? Run <code>sudo outpost setup-code</code> on the machine.</li>
        <li>Pick the apps you want: library, maps, assistant.</li>
        <li>Choose how much Wikipedia, a size for each collection, your map regions and AI models. The storage bar shows what fits.</li>
        <li>Select <strong>Complete setup</strong>. Downloads start at once, smallest first, so the node is useful within minutes.</li>
      </ol>

      <h2 id="offline">3. Go offline</h2>
      <p>
        Watch progress on the Downloads page. When everything is done you can disconnect the internet. Everyone on your network opens the same
        address: the dashboard on port 8080, the library on 8081. The node starts by itself after a reboot or power cut, and an interrupted
        download carries on where it stopped the next time the internet is there.
      </p>

      <h2 id="update">Updating</h2>
      <p>
        Run the installer again. It replaces the program, keeps your content and settings, and restarts the service. Newer editions of your
        archives show up under Content as “Update all”.
      </p>

      <h2 id="trouble">If something goes wrong</h2>
      <dl>
        <dt>The dashboard does not open from another device</dt>
        <dd>Check that both are on the same network and that ports 8080 and 8081 are allowed. The installer opens them in ufw and firewalld when those are active.</dd>
        <dt>I forgot the admin password</dt>
        <dd>Run <code>sudo outpost reset-admin</code> on the machine. It prints a new setup code; your content stays.</dd>
        <dt>A download failed</dt>
        <dd>Open Downloads and select Retry. Failures from a dropped connection retry by themselves; a checksum mismatch means the file was not the one the catalog names, and Outpost refuses it.</dd>
        <dt>Where are the logs?</dt>
        <dd><code>journalctl -u outpost -f</code></dd>
      </dl>
    </div>
  );
}
