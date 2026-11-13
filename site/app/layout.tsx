import type { Metadata, Viewport } from "next";
import Link from "next/link";
import { Archivo, Martian_Mono } from "next/font/google";
import NavLinks from "./components/NavLinks";
import { loadCatalog, loadRelease } from "@/lib/catalog";
import { SITE } from "@/lib/site";
import "./globals.css";

// One family in two widths: wide for display, normal for reading.
const sans = Archivo({ subsets: ["latin"], axes: ["wdth"], variable: "--font-sans", display: "swap" });
const mono = Martian_Mono({ subsets: ["latin"], axes: ["wdth"], variable: "--font-mono", display: "swap" });

export const metadata: Metadata = {
  metadataBase: new URL(SITE),
  title: { default: "Outpost: the offline library for hardware you own", template: "%s · Outpost" },
  description:
    "Outpost turns a spare Mac or Linux PC into an offline library for your home: Wikipedia, medical guides, repair manuals, street maps and an AI assistant that keep working when the internet does not.",
  openGraph: {
    title: "Outpost: the offline library for hardware you own",
    description: "When the world goes dark, information is power. Wikipedia, medical guides, repair manuals, maps and an AI assistant on a computer you already own.",
    url: SITE,
    siteName: "Outpost",
    type: "website",
  },
  twitter: { card: "summary_large_image" },
  icons: { icon: "/icon.svg" },
};

export const viewport: Viewport = { themeColor: "#000000", colorScheme: "dark" };

function Marking({ release, catalog }: { release: string; catalog: string }) {
  // The strip that runs along the top and bottom of every page of a file.
  return (
    <div className="marking" aria-hidden="true">
      <span>Release {release}</span>
      <span>Unclassified // Cleared for public release</span>
      <span>Catalog {catalog}</span>
    </div>
  );
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const release = loadRelease().version;
  const catalog = loadCatalog().generated.slice(0, 10);
  return (
    <html lang="en" className={`${sans.variable} ${mono.variable}`}>
      <body>
        <a className="skip" href="#main">Skip to content</a>
        <Marking release={release} catalog={catalog} />
        <header className="site-header">
          <div className="wrap">
            <Link href="/" className="wordmark" aria-label="Outpost home">
              <svg viewBox="0 0 32 32" aria-hidden="true">
                <path d="M16 3 31 29H1Z" fill="currentColor" />
                <path d="M16 15v14" stroke="var(--bg)" strokeWidth="2.6" />
              </svg>
              Outpost
            </Link>
            <NavLinks />
          </div>
        </header>
        <main id="main">{children}</main>
        <footer className="site-footer">
          <div className="wrap">
            <div className="foot-grid">
              <div>
                <p>
                  Outpost installs open content from its publishers: archives packaged by <a href="https://kiwix.org">Kiwix</a>, maps from{" "}
                  <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a> via <a href="https://protomaps.com">Protomaps</a>,
                  and AI models through <a href="https://ollama.com">Ollama</a>. Each keeps its own licence.
                </p>
                <p>No accounts, no telemetry. Your node only talks to the internet to download what you choose.</p>
              </div>
              <nav aria-label="Footer">
                <Link href="/catalog">Content catalog</Link>
                <Link href="/install">Install guide</Link>
                <Link href="/docs">Help and answers</Link>
                <Link href="/docs#security">Security</Link>
              </nav>
            </div>
            <p className="foot-mark" aria-hidden="true">Outpost</p>
          </div>
        </footer>
        <Marking release={release} catalog={catalog} />
      </body>
    </html>
  );
}
