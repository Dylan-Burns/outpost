import { useEffect, useState } from "preact/hooks";
import { api } from "./api";

/** Loads path now and then every ms milliseconds (0: once). Returns the data,
 *  the last error and a function that loads again at once. */
export function usePoll<T>(path: string | null, ms: number): [T | null, string, () => void] {
  const [data, setData] = useState<T | null>(null);
  const [err, setErr] = useState("");
  const [tick, setTick] = useState(0);
  useEffect(() => {
    if (!path) return;
    let alive = true;
    const load = () =>
      api.get<T>(path).then(
        (d) => alive && (setData(d), setErr("")),
        (e) => alive && setErr(e.message),
      );
    load();
    const t = ms ? setInterval(load, ms) : 0;
    return () => {
      alive = false;
      if (t) clearInterval(t);
    };
  }, [path, ms, tick]);
  return [data, err, () => setTick((t) => t + 1)];
}
