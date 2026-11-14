// Thin client for the node's JSON API. Writes carry the session's CSRF token.

export type Apps = { library: boolean; maps: boolean; assistant: boolean };

export type Status = {
  version: string;
  nodeName: string;
  setupComplete: boolean;
  claimed: boolean;
  signedIn: boolean;
  publicAccess: boolean;
  assistantPublic: boolean;
  apps: Apps;
  libraryPort: number;
  libraryRunning: boolean;
  aiAvailable: boolean;
  aiVersion: string;
  latestVersion: string;
  csrf?: string;
};

export type Item = {
  id: string;
  kind: "zim" | "map-region" | "map-world" | "map-assets" | "model";
  title: string;
  description?: string;
  publisher?: string;
  license?: string;
  size: number;
  date?: string;
  notice?: string;
  group?: string;
  bbox?: number[];
  model?: string;
  ramGB?: number;
  rank?: number; // models: 1 is best in Outpost's assistant tests
};

export type Category = {
  id: string;
  name: string;
  description: string;
  icon?: string;
  kind: "tiered" | "single";
  tiers?: Record<string, string[]>;
  options?: { id: string; label: string; description?: string; items: string[] }[];
  default?: string;
};

export type Catalog = { schema: number; sequence: number; generated: string; categories: Category[]; items: Item[] };

export type Selection = {
  wikipedia: string;
  tiers: Record<string, string>;
  regions: string[];
  models: string[];
  extra?: string[];
};

export type Installed = { id: string; kind: string; file?: string; size: number; date?: string; installedAt: string };

export type SystemInfo = { os: string; arch: string; cpu: string; cores: number; memBytes: number; gpu?: string; hostname: string; uptimeSeconds: number };

export type CatalogResponse = {
  catalog: Catalog;
  selection: Selection;
  apps: Apps;
  installed: Record<string, Installed>;
  outdated: string[] | null;
  disk: { free: number; total: number; used: number; partial: number };
  system: SystemInfo;
};

export type Job = {
  id: string;
  itemId: string;
  action: "install" | "remove";
  state: "queued" | "running" | "waiting" | "done" | "failed" | "cancelled";
  error?: string;
  attempts: number;
  created: string;
  updated: string;
  title: string;
  kind: string;
  size: number;
  live?: { done: number; total: number; speed: number; phase: string };
};

let csrf = "";
export function setCSRF(token: string | undefined) {
  csrf = token || "";
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET" && csrf) headers["X-CSRF-Token"] = csrf;
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: "same-origin",
  });
  const text = await res.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    /* not JSON */
  }
  if (res.status === 401 && path !== "/api/login" && path !== "/api/setup/claim") {
    // The session ended (signed out elsewhere, expired, password changed).
    window.dispatchEvent(new Event("outpost:unauthorized"));
  }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || `Request failed (${res.status})`);
  return data as T;
}

export const api = {
  get: <T>(p: string) => request<T>("GET", p),
  post: <T>(p: string, b?: unknown) => request<T>("POST", p, b ?? {}),
  put: <T>(p: string, b: unknown) => request<T>("PUT", p, b),
};

/** Streams server-sent events from a POST endpoint. */
export async function streamChat(
  body: unknown,
  onEvent: (event: string, data: any) => void,
  signal: AbortSignal,
): Promise<void> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (csrf) headers["X-CSRF-Token"] = csrf;
  const res = await fetch("/api/assistant/chat", { method: "POST", headers, body: JSON.stringify(body), signal, credentials: "same-origin" });
  if (!res.ok || !res.body) {
    let msg = `The assistant answered ${res.status}`;
    try {
      msg = (await res.json()).error || msg;
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, msg);
  }
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    let i;
    while ((i = buf.indexOf("\n\n")) >= 0) {
      const chunk = buf.slice(0, i);
      buf = buf.slice(i + 2);
      let ev = "message";
      let data = "";
      for (const line of chunk.split("\n")) {
        if (line.startsWith("event: ")) ev = line.slice(7);
        else if (line.startsWith("data: ")) data += line.slice(6);
      }
      try {
        onEvent(ev, data ? JSON.parse(data) : null);
      } catch {
        /* ignore malformed event */
      }
    }
  }
}
