const P = (d: string) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d={d} />
  </svg>
);
export const icons: Record<string, () => preact.JSX.Element> = {
  book: () => P("M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5v14zM20 17v4H6.5A2.5 2.5 0 0 1 4 19.5"),
  search: () => P("M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35"),
  map: () => P("M9 3 3 6v15l6-3 6 3 6-3V3l-6 3-6-3zM9 3v15M15 6v15"),
  chat: () => P("M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"),
  download: () => P("M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"),
  sliders: () => P("M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"),
  globe: () => P("M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20zM2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20"),
  medical: () => P("M12 3v18M3 12h18"),
  shield: () => P("M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"),
  wrench: () => P("M14.7 6.3a4 4 0 0 0 5 5L22 14l-8 8-2.3-2.3a4 4 0 0 0-5-5L2 10l8-8z"),
  sprout: () => P("M7 20h10M12 20v-8M12 12C12 8 9 5 4 5c0 4 3 7 8 7zM12 12c0-4 3-7 8-7 0 4-3 7-8 7z"),
  school: () => P("M22 10 12 5 2 10l10 5 10-5zM6 12v5c3 2 9 2 12 0v-5"),
  atom: () => P("M12 12h.01M20.2 20.2c2-2-.5-7.8-5.6-12.8S3.8 0 1.8 1.8s.5 7.8 5.6 12.8 10.8 7.6 12.8 5.6zM3.8 20.2c-2-2 .5-7.8 5.6-12.8S20.2 0 22.2 1.8s-.5 7.8-5.6 12.8S5.8 22.2 3.8 20.2z"),
  terminal: () => P("M4 17l6-6-6-6M12 19h8"),
  library: () => P("M4 4v16M8 4v16M12 4l4 16M18 4v16"),
  gear: () => P("M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.6 1.6 0 0 0-1-1.5 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.6 1.6 0 0 0 1.5-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1z"),
};

export function Logo() {
  return (
    <svg viewBox="0 0 64 64" aria-hidden="true">
      <rect width="64" height="64" rx="14" fill="var(--accent)" />
      <path d="M32 12 50 46H14Z" fill="none" stroke="var(--bg)" stroke-width="5" stroke-linejoin="round" />
      <path d="M32 30v16" stroke="var(--bg)" stroke-width="5" stroke-linecap="round" />
    </svg>
  );
}

export function Icon({ name }: { name?: string }) {
  const f = icons[name || "book"] || icons.book;
  return f();
}
