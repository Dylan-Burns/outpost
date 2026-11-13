import { useEffect, useState } from "preact/hooks";
import type { ComponentChildren } from "preact";

const listeners = new Set<() => void>();

export function navigate(to: string) {
  if (to === location.pathname + location.search) return;
  history.pushState(null, "", to);
  listeners.forEach((l) => l());
  window.scrollTo(0, 0);
}

export function usePath(): string {
  const [path, setPath] = useState(location.pathname);
  useEffect(() => {
    const f = () => setPath(location.pathname);
    listeners.add(f);
    window.addEventListener("popstate", f);
    return () => {
      listeners.delete(f);
      window.removeEventListener("popstate", f);
    };
  }, []);
  return path;
}

export function Link(props: { href: string; class?: string; style?: any; children: ComponentChildren; "aria-current"?: any }) {
  return (
    <a
      {...props}
      onClick={(e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        navigate(props.href);
      }}
    />
  );
}
