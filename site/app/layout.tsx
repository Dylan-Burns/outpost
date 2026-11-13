import type { Metadata, Viewport } from "next";
import Link from "next/link";
import { Barlow_Condensed, Literata } from "next/font/google";
import "./globals.css";

const display = Barlow_Condensed({ subsets: ["latin"], weight: ["500", "600", "700"], variable: "--font-display" });
const body = Literata({ subsets: ["latin"], weight: ["400", "500", "600"], style: ["normal", "italic"], variable: "--font-body" });

const SITE = "https://outpost-offline.vercel.app";

export const metadata: Metadata = {
  metadataBase: new URL(SITE),
  title: { default: "Outpost: the offline library for hardware you own", template: "%s · Outpost" },
  description:
    "Turn a spare computer into an offline server for Wikipedia, medical references, repair guides, street maps and a local AI. One command to install, a setup wizard to choose your content.",
  openGraph: {
    title: "Outpost: the offline library for hardware you own",
    description: "Wikipedia, medicine, repair guides, maps and a local AI on a computer you already have. Works when the internet doesn't.",
    url: SITE,
    siteName: "Outpost",
    type: "website",
  },
  icons: { icon: "/icon.svg" },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f1f3ec" },
    { media: "(prefers-color-scheme: dark)", color: "#141a17" },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${display.variable} ${body.variable}`}>
      <body>
        <a className="skip" href="#main">Skip to content</a>
        <header className="site-header">
          <Link href="/" className="wordmark" aria-label="Outpost home">
            <svg viewBox="0 0 32 32" aria-hidden="true">
              <path d="M16 4 29 27H3Z" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinejoin="round" />
              <path d="M16 14v13" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" />
            </svg>
            Outpost
          </Link>
          <nav aria-label="Main">
            <Link href="/catalog">Content</Link>
            <Link href="/install">Install</Link>
            <Link href="/docs">Docs</Link>
          </nav>
        </header>
        <main id="main">{children}</main>
        <footer className="site-footer">
          <div>
            <p>
              Outpost installs open content from its publishers: archives packaged by{" "}
              <a href="https://kiwix.org">Kiwix</a>, maps from <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a> via{" "}
              <a href="https://protomaps.com">Protomaps</a>, and AI models through <a href="https://ollama.com">Ollama</a>. Each keeps its own licence.
            </p>
            <p>No accounts, no telemetry. Your node only talks to the internet to download what you choose.</p>
          </div>
          <nav aria-label="Footer">
            <Link href="/catalog">Content catalog</Link>
            <Link href="/install">Install guide</Link>
            <Link href="/docs">Documentation</Link>
            <Link href="/docs#security">Security</Link>
          </nav>
        </footer>
      </body>
    </html>
  );
}
