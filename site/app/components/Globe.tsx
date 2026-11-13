"use client";
import { useEffect, useRef } from "react";

// A line-drawn globe made from the catalog's own map region outlines.
// Decorative: the page reads the same without it, so it is hidden from
// assistive tech and never blocks the content from loading.

type Packed = { scale: number; regions: number; rings: number[][] };
type LineSet = { xyz: Float32Array; starts: Uint32Array };

const RAD = Math.PI / 180;
const SPIN = -2 * RAD; // per second: one turn in three minutes, turning east as the Earth does
const FRAME = 1000 / 31; // the turn is slow; half the display rate is indistinguishable
const MAX_TILT = 62 * RAD;

/** Build line sets from runs of [lon, lat] in degrees. Long steps are split so
 *  a border that follows a parallel curves with the globe instead of cutting
 *  straight across it. */
function lineSet(runs: number[][]): LineSet {
  const pts: number[] = [];
  const starts: number[] = [];
  const push = (lon: number, lat: number) => {
    const c = Math.cos(lat * RAD);
    pts.push(c * Math.sin(lon * RAD), Math.sin(lat * RAD), c * Math.cos(lon * RAD));
  };
  for (const run of runs) {
    let open = false;
    for (let i = 0; i < run.length; i += 2) {
      const lon = run[i];
      const lat = run[i + 1];
      if (i > 0) {
        const pl = run[i - 2];
        const pt = run[i - 1];
        // Outlines are cut at the date line; an edge that runs along it, or
        // jumps across it, is not a real border.
        const seam = (Math.abs(lon) > 179.9 && Math.abs(pl) > 179.9) || Math.abs(lon - pl) > 180;
        if (seam) {
          open = false;
        } else {
          const n = Math.ceil(Math.max(Math.abs(lon - pl), Math.abs(lat - pt)) / 2.5);
          for (let s = 1; s < n; s++) push(pl + ((lon - pl) * s) / n, pt + ((lat - pt) * s) / n);
        }
      }
      if (!open) {
        starts.push(pts.length / 3);
        open = true;
      }
      push(lon, lat);
    }
  }
  starts.push(pts.length / 3);
  return { xyz: new Float32Array(pts), starts: new Uint32Array(starts) };
}

function unpack(d: Packed): LineSet {
  return lineSet(
    d.rings.map((r) => {
      const run: number[] = [];
      let x = 0;
      let y = 0;
      for (let i = 0; i < r.length; i += 2) {
        x += r[i];
        y += r[i + 1];
        run.push(x / d.scale, y / d.scale);
      }
      return run;
    }),
  );
}

function graticule(): LineSet {
  const runs: number[][] = [];
  for (let lon = -180; lon < 180; lon += 15) {
    const run: number[] = [];
    for (let lat = -84; lat <= 84; lat += 3) run.push(lon, lat);
    runs.push(run);
  }
  for (let lat = -75; lat <= 75; lat += 15) {
    const run: number[] = [];
    for (let lon = -179.5; lon <= 179.5; lon += 3.59) run.push(lon, lat);
    run.push(-179.5, lat);
    runs.push(run);
  }
  return lineSet(runs);
}

function coordinate(deg: number, pos: string, neg: string, width: number) {
  const v = Math.abs(deg).toFixed(1).padStart(width, "0");
  return `${v}°${deg >= 0 ? pos : neg}`;
}

export default function Globe() {
  const box = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const latRef = useRef<HTMLSpanElement>(null);
  const lonRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const holder = box.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !holder || !ctx) return;

    const still = window.matchMedia("(prefers-reduced-motion: reduce)");
    const grid = graticule();
    let land: LineSet | null = null;
    let lon = -24 * RAD;
    let tilt = 26 * RAD;
    let speed = still.matches ? 0 : SPIN;
    let dragging = false;
    let visible = true;
    let frame = 0;
    let last = 0;
    let lastReadout = 0;
    let size = 0; // CSS pixels
    let dpr = 1;
    let gone = false;

    const stroke = (set: LineSet, r: number, cx: number, cy: number, m: number[], front: boolean) => {
      const { xyz, starts } = set;
      const [cl, sl, ct, st] = m;
      ctx.beginPath();
      for (let s = 0; s < starts.length - 1; s++) {
        let pen = false;
        for (let i = starts[s] * 3, end = starts[s + 1] * 3; i < end; i += 3) {
          const x = xyz[i];
          const y = xyz[i + 1];
          const z = xyz[i + 2];
          const x1 = x * cl - z * sl;
          const z1 = z * cl + x * sl;
          const z2 = y * st + z1 * ct;
          if (z2 > 0 === front) {
            const px = cx + x1 * r;
            const py = cy - (y * ct - z1 * st) * r;
            if (pen) ctx.lineTo(px, py);
            else {
              ctx.moveTo(px, py);
              pen = true;
            }
          } else pen = false;
        }
      }
      ctx.stroke();
    };

    const draw = () => {
      const w = canvas.width;
      if (!w) return;
      const c = w / 2;
      const bezel = 26 * dpr;
      const r = c - bezel;
      const m = [Math.cos(lon), Math.sin(lon), Math.cos(tilt), Math.sin(tilt)];
      ctx.clearRect(0, 0, w, w);
      ctx.lineWidth = dpr;
      ctx.lineJoin = "round";

      // Far side first, faint, so the sphere reads as glass.
      if (land) {
        ctx.strokeStyle = "rgba(255,255,255,0.11)";
        stroke(land, r, c, c, m, false);
      }
      ctx.strokeStyle = "rgba(255,255,255,0.15)";
      stroke(grid, r, c, c, m, true);
      if (land) {
        ctx.strokeStyle = "rgba(255,255,255,0.92)";
        stroke(land, r, c, c, m, true);
      }

      // Limb, then an instrument bezel: a tick every 5 degrees, longer every 30.
      ctx.strokeStyle = "rgba(255,255,255,0.7)";
      ctx.beginPath();
      ctx.arc(c, c, r, 0, Math.PI * 2);
      ctx.stroke();
      ctx.strokeStyle = "rgba(255,255,255,0.5)";
      ctx.beginPath();
      for (let a = 0; a < 360; a += 5) {
        const len = (a % 90 === 0 ? 16 : a % 30 === 0 ? 10 : 4) * dpr;
        const ca = Math.cos(a * RAD);
        const sa = Math.sin(a * RAD);
        const r0 = r + 7 * dpr;
        ctx.moveTo(c + ca * r0, c + sa * r0);
        ctx.lineTo(c + ca * (r0 + len), c + sa * (r0 + len));
      }
      // Centre mark for the position readout.
      const k = 9 * dpr;
      const gap = 3 * dpr;
      ctx.moveTo(c - k, c);
      ctx.lineTo(c - gap, c);
      ctx.moveTo(c + gap, c);
      ctx.lineTo(c + k, c);
      ctx.moveTo(c, c - k);
      ctx.lineTo(c, c - gap);
      ctx.moveTo(c, c + gap);
      ctx.lineTo(c, c + k);
      ctx.stroke();
    };

    const readout = (now: number) => {
      if (now - lastReadout < 120) return;
      lastReadout = now;
      const deg = ((((lon / RAD + 180) % 360) + 360) % 360) - 180;
      if (latRef.current) latRef.current.textContent = coordinate(tilt / RAD, "N", "S", 4);
      if (lonRef.current) lonRef.current.textContent = coordinate(deg, "E", "W", 5);
    };

    const moving = () => dragging || Math.abs(speed) > 0.0006;

    const tick = (now: number) => {
      frame = 0;
      if (!dragging && now - last < FRAME) {
        frame = requestAnimationFrame(tick);
        return;
      }
      const dt = Math.min(0.08, (now - last) / 1000 || 0);
      last = now;
      if (!dragging) {
        lon += speed * dt;
        const rest = still.matches ? 0 : SPIN;
        speed += (rest - speed) * (1 - Math.exp(-dt * 1.6));
      }
      draw();
      readout(now);
      if (visible && moving()) frame = requestAnimationFrame(tick);
    };

    const wake = () => {
      if (gone || frame) return;
      last = performance.now();
      frame = requestAnimationFrame(tick);
    };

    const resize = () => {
      const next = Math.round(holder.clientWidth);
      if (!next || next === size) return;
      size = next;
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = canvas.height = Math.round(size * dpr);
      draw();
      wake();
    };

    const ro = new ResizeObserver(resize);
    ro.observe(holder);
    const io = new IntersectionObserver(([e]) => {
      visible = e.isIntersecting;
      if (visible) wake();
    });
    io.observe(holder);

    // Drag to turn it.
    let px = 0;
    let py = 0;
    let pt = 0;
    const down = (e: PointerEvent) => {
      dragging = true;
      px = e.clientX;
      py = e.clientY;
      pt = performance.now();
      speed = 0;
      canvas.setPointerCapture(e.pointerId);
      holder.dataset.dragging = "";
      wake();
    };
    const move = (e: PointerEvent) => {
      if (!dragging) return;
      const now = performance.now();
      const per = 1 / Math.max(1, size / 2 - 26);
      const dx = (e.clientX - px) * per;
      lon -= dx;
      tilt = Math.max(-MAX_TILT, Math.min(MAX_TILT, tilt + (e.clientY - py) * per));
      const dt = Math.max(0.008, (now - pt) / 1000);
      speed = Math.max(-3, Math.min(3, -dx / dt));
      px = e.clientX;
      py = e.clientY;
      pt = now;
    };
    const up = () => {
      if (!dragging) return;
      dragging = false;
      delete holder.dataset.dragging;
      if (performance.now() - pt > 120) speed = 0; // held still before letting go
      wake();
    };
    canvas.addEventListener("pointerdown", down);
    canvas.addEventListener("pointermove", move);
    canvas.addEventListener("pointerup", up);
    canvas.addEventListener("pointercancel", up);

    const motion = () => {
      if (!still.matches) wake();
    };
    still.addEventListener("change", motion);

    fetch("/globe.json")
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((d: Packed) => {
        if (gone) return;
        land = unpack(d);
      })
      .catch(() => {
        /* the grid alone still draws */
      })
      .finally(() => {
        if (gone) return;
        holder.dataset.ready = "";
        draw();
        wake();
      });

    resize();
    return () => {
      gone = true;
      cancelAnimationFrame(frame);
      ro.disconnect();
      io.disconnect();
      still.removeEventListener("change", motion);
      canvas.removeEventListener("pointerdown", down);
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerup", up);
      canvas.removeEventListener("pointercancel", up);
    };
  }, []);

  return (
    <div className="globe" ref={box} aria-hidden="true">
      <canvas ref={canvasRef} />
      <p className="globe-readout">
        <span>Lat</span> <span ref={latRef}>26.0°N</span>
        <span>Lon</span> <span ref={lonRef}>024.0°W</span>
      </p>
    </div>
  );
}
