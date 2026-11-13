"use client";
import { useEffect, useRef } from "react";

// The hero's moving picture, and the headline acted out: a globe drawn from
// the catalog's own country outlines turns while links between cities carry
// traffic. Then the links fail one by one and the world goes dim, and the
// library's mark keeps pulsing: each pulse lights the coast it passes over.
// Then the lights come back.
//
// The stylesheet says which shape to draw, through the --shape property. On a
// phone it is a whole globe in a square box. On a desktop it is the top half
// of one, as wide as the screen, filling the hero with the words laid over it.
//
// It is an illustration, so the drawing is hidden from assistive tech; the
// one real control, the pause button, is not.

type Packed = { scale: number; regions: number; rings: number[][] };
type LineSet = { xyz: Float32Array; starts: Uint32Array };
type Link = { xyz: Float32Array; cutAt: number; backAt: number; speed: number; phase: number };

const RAD = Math.PI / 180;
const SPIN = -6 * RAD; // per second: a full turn each minute, eastward like the Earth
const MAX_TILT = 62 * RAD;

// One loop of the story, in seconds.
const CYCLE = 19;
const CUT = 7; // links start to fail
const DARK = 10; // the last one is gone
const BACK = 15; // they start to return
const ARC_STEPS = 36;

// [latitude, longitude] of the cities the links run between.
const CITIES: [number, number][] = [
  [40.7, -74.0], // 0 New York
  [34.1, -118.2], // 1 Los Angeles
  [19.4, -99.1], // 2 Mexico City
  [-23.5, -46.6], // 3 Sao Paulo
  [-34.6, -58.4], // 4 Buenos Aires
  [51.5, -0.1], // 5 London
  [48.9, 2.4], // 6 Paris
  [6.5, 3.4], // 7 Lagos
  [30.0, 31.2], // 8 Cairo
  [-26.2, 28.0], // 9 Johannesburg
  [55.8, 37.6], // 10 Moscow
  [25.2, 55.3], // 11 Dubai
  [19.1, 72.9], // 12 Mumbai
  [1.4, 103.8], // 13 Singapore
  [35.7, 139.7], // 14 Tokyo
  [-33.9, 151.2], // 15 Sydney
  [39.9, 116.4], // 16 Beijing
  [-1.3, 36.8], // 17 Nairobi
  [61.2, -149.9], // 18 Anchorage
  [21.3, -157.9], // 19 Honolulu
  [64.1, -21.9], // 20 Reykjavik
];
const ROUTES: [number, number][] = [
  [0, 5], [0, 1], [0, 3], [2, 0], [1, 14], [1, 19], [19, 15], [5, 6], [5, 7], [5, 10], [6, 8], [8, 11], [11, 12],
  [12, 13], [13, 14], [13, 15], [14, 16], [7, 9], [17, 11], [3, 7], [4, 3], [10, 16], [5, 20], [18, 14], [0, 20], [9, 17],
];

function unit(lat: number, lon: number): [number, number, number] {
  const c = Math.cos(lat * RAD);
  return [c * Math.sin(lon * RAD), Math.sin(lat * RAD), c * Math.cos(lon * RAD)];
}

/** Build line sets from runs of [lon, lat] in degrees. Long steps are split so
 *  a border that follows a parallel curves with the globe instead of cutting
 *  straight across it. */
function lineSet(runs: number[][]): LineSet {
  const pts: number[] = [];
  const starts: number[] = [];
  const push = (lon: number, lat: number) => pts.push(...unit(lat, lon));
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

/** Arcs between cities: great circles lifted off the surface, higher the
 *  farther they go. Each gets its own moment to fail and to return. */
function links(): Link[] {
  const n = ROUTES.length;
  return ROUTES.map(([from, to], i) => {
    const a = unit(...CITIES[from]);
    const b = unit(...CITIES[to]);
    const angle = Math.acos(Math.max(-1, Math.min(1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2])));
    const lift = 0.05 + 0.2 * (angle / Math.PI);
    const xyz = new Float32Array((ARC_STEPS + 1) * 3);
    for (let s = 0; s <= ARC_STEPS; s++) {
      const t = s / ARC_STEPS;
      const wa = Math.sin((1 - t) * angle) / Math.sin(angle);
      const wb = Math.sin(t * angle) / Math.sin(angle);
      const r = 1 + lift * Math.sin(Math.PI * t);
      xyz[s * 3] = (a[0] * wa + b[0] * wb) * r;
      xyz[s * 3 + 1] = (a[1] * wa + b[1] * wb) * r;
      xyz[s * 3 + 2] = (a[2] * wa + b[2] * wb) * r;
    }
    // A fixed shuffle (11 shares no factor with the count), so the order
    // looks unplanned but is the same on every loop.
    const slot = ((i * 11) % n) / n;
    return {
      xyz,
      cutAt: CUT + slot * (DARK - CUT - 0.5),
      backAt: BACK + slot * (CYCLE - BACK - 1.2),
      speed: 0.2 + 0.6 / (3 + angle * 5), // short hops look quicker
      phase: (i * 0.37) % 1,
    };
  });
}

const smooth = (x: number) => (x <= 0 ? 0 : x >= 1 ? 1 : x * x * (3 - 2 * x));

export default function Globe() {
  const box = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const netRef = useRef<HTMLSpanElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const toggle = useRef<() => void>(() => {});

  useEffect(() => {
    const canvas = canvasRef.current;
    const holder = box.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !holder || !ctx) return;

    const still = window.matchMedia("(prefers-reduced-motion: reduce)");
    const grid = graticule();
    const routes = links();
    const cities = new Float32Array(CITIES.flatMap(([lat, lon]) => unit(lat, lon)));
    let land: LineSet | null = null;
    let lon = -30 * RAD;
    let tilt = 24 * RAD;
    let run = !still.matches; // people who asked for less motion start it themselves
    let speed = run ? SPIN : 0;
    let clock = 1.5; // seconds into the loop
    let dragging = false;
    let visible = true;
    let frame = 0;
    let last = 0;
    let wide = 0; // the box, in CSS pixels
    let tall = 0;
    let dpr = 1;
    let half = false; // drawing the top half only
    let turned = false; // someone has dragged it
    // The circle, in canvas pixels.
    let cx = 0;
    let cy = 0;
    let r = 0;
    let gone = false;
    let shownDown: boolean | null = null;
    const p = [0, 0];
    const pulses: number[] = [];
    // Keeping up: see pace().
    let coarse = 1; // the share of the full pixel density in use
    let settled = false;
    let gaps = 0; // frame gaps being averaged, in milliseconds
    let counted = 0;
    let skip = 90; // frames left out of the count while the page settles
    let before = 0; // the average before the last step down, while it is on trial

    /** Which shape the stylesheet asks for at this width. */
    const wantsHalf = () => getComputedStyle(holder).getPropertyValue("--shape").trim() === "half";

    const stroke = (set: LineSet, m: number[], front: boolean) => {
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

    /** Puts the screen position of the point at index i into p, or returns
     *  false when the globe hides it. A point above the surface can still
     *  show past the edge. */
    const place = (xyz: Float32Array, i: number, m: number[]) => {
      const [cl, sl, ct, st] = m;
      const x = xyz[i];
      const y = xyz[i + 1];
      const z = xyz[i + 2];
      const x1 = x * cl - z * sl;
      const z1 = z * cl + x * sl;
      const y2 = y * ct - z1 * st;
      if (y * st + z1 * ct < 0 && x1 * x1 + y2 * y2 < 1) return false;
      p[0] = cx + x1 * r;
      p[1] = cy - y2 * r;
      return true;
    };

    /** The paint for the half globe's map: white at strength `base`, lifted by
     *  `glow` in the wake of each pulse, and thinning toward the globe's edge,
     *  where the surface turns away and its lines crowd together. One paint
     *  does all three so the map is one stroke. */
    const mapPaint = (base: number, glow: number) => {
      const g = ctx.createRadialGradient(cx, cy, 0, cx, cy, r);
      const wake = Math.min(0.24, (150 * dpr) / r);
      const stops = [0, 0.8, 1];
      if (glow > 0) for (const at of pulses) stops.push(Math.max(0, at - wake), at, Math.min(1, at + 0.003));
      for (const x of stops) {
        let lift = 0;
        for (const at of pulses) if (x <= at && x > at - wake) lift = Math.max(lift, Math.sqrt(1 - at) * (1 - (at - x) / wake));
        const edge = x <= 0.8 ? 1 : 1 - 4.25 * (x - 0.8);
        g.addColorStop(x, `rgba(255,255,255,${Math.min(1, base + glow * lift) * edge})`);
      }
      return g;
    };

    const draw = () => {
      const w = canvas.width;
      const h = canvas.height;
      if (!w || !h || r <= 0) return; // a box too small to hold a globe
      const m = [Math.cos(lon), Math.sin(lon), Math.cos(tilt), Math.sin(tilt)];
      const t = clock % CYCLE;
      // How lit the world is: 1 in daylight, 0 once everything has failed.
      const lit = t < CUT ? 1 : t < DARK ? 1 - smooth((t - CUT) / (DARK - CUT)) : t < BACK ? 0 : smooth((t - BACK) / (CYCLE - BACK - 0.6));
      ctx.clearRect(0, 0, w, h);
      ctx.lineJoin = "round";

      // The library sends out a steady pulse from the globe's centre: the
      // middle of the whole globe, the foot of the half one.
      const count = half ? 3 : 2;
      const reach = half ? r : r * 0.4;
      for (let i = 0; i < count; i++) pulses[i] = (clock / (half ? 5.4 : 2.6) + i / count) % 1;
      pulses.length = count;

      if (half) {
        // The half globe is as wide as the window, and a browser's cost for a
        // stroke that spans the canvas grows with the canvas, not with the
        // line. Measured in Chrome on a 2019 MacBook Pro at 2880 pixels wide:
        // about 3ms for a stroke one canvas pixel wide, 14ms for a wider one.
        // So the map is two strokes, grid and land, one canvas pixel wide
        // whatever the screen's density; on a dense screen that is finer, so
        // it is given a little more strength there. The far side is left out:
        // behind a page of words it only added noise. In the dark each pulse
        // lights the coast it has just passed over, and that is in the paint.
        const ink = Math.min(1, 0.85 + 0.15 * (dpr - 1));
        ctx.lineWidth = 1;
        ctx.strokeStyle = mapPaint((0.08 + 0.07 * lit) * ink, 0);
        stroke(grid, m, true);
        if (land) {
          ctx.strokeStyle = mapPaint((0.24 + 0.68 * lit) * ink, 0.9 * (1 - lit));
          stroke(land, m, true);
        }
      } else {
        // The whole globe: far side first, faint, so the sphere reads as glass.
        ctx.lineWidth = dpr;
        if (land) {
          ctx.strokeStyle = `rgba(255,255,255,${0.04 + 0.07 * lit})`;
          stroke(land, m, false);
        }
        ctx.strokeStyle = `rgba(255,255,255,${0.08 + 0.07 * lit})`;
        stroke(grid, m, true);
        if (land) {
          ctx.strokeStyle = `rgba(255,255,255,${0.24 + 0.68 * lit})`;
          stroke(land, m, true);
        }
      }

      // Limb, then an instrument bezel: a tick every 5 degrees, longer every 30.
      ctx.lineWidth = dpr;
      ctx.strokeStyle = "rgba(255,255,255,0.7)";
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.stroke();
      ctx.strokeStyle = "rgba(255,255,255,0.5)";
      ctx.beginPath();
      for (let a = 0; a < 360; a += 5) {
        const len = (a % 90 === 0 ? 16 : a % 30 === 0 ? 10 : 4) * dpr;
        const ca = Math.cos(a * RAD);
        const sa = Math.sin(a * RAD);
        const r0 = r + 7 * dpr;
        ctx.moveTo(cx + ca * r0, cy + sa * r0);
        ctx.lineTo(cx + ca * (r0 + len), cy + sa * (r0 + len));
      }
      ctx.stroke();

      // The links, each in its own state: carrying traffic, failing, gone, or
      // being drawn back in. Each is its own short stroke, which stays cheap.
      ctx.lineWidth = 1.25 * dpr;
      ctx.fillStyle = "#fff";
      for (const link of routes) {
        let alpha = 0.8;
        let upTo = 1;
        if (t >= link.cutAt && t < link.backAt) {
          const since = t - link.cutAt;
          if (since > 0.45) continue;
          alpha = Math.floor(since * 22) % 2 ? 0.12 : 0.9; // a flicker before it drops
        } else if (t >= link.backAt && t < link.backAt + 0.8) {
          upTo = (t - link.backAt) / 0.8;
        }
        ctx.strokeStyle = `rgba(255,255,255,${alpha})`;
        ctx.beginPath();
        let pen = false;
        const stop = Math.floor(upTo * ARC_STEPS);
        for (let s = 0; s <= stop; s++) {
          if (place(link.xyz, s * 3, m)) {
            if (pen) ctx.lineTo(p[0], p[1]);
            else {
              ctx.moveTo(p[0], p[1]);
              pen = true;
            }
          } else pen = false;
        }
        ctx.stroke();
        // Traffic: one bright mark running the length of a working link.
        if (upTo === 1 && alpha > 0.5) {
          const at = Math.floor(((clock * link.speed + link.phase) % 1) * ARC_STEPS);
          if (place(link.xyz, at * 3, m)) {
            const k = 2.2 * dpr;
            ctx.fillRect(p[0] - k, p[1] - k, k * 2, k * 2);
          }
        }
      }
      // The cities stay on the map; they just have nothing to talk to.
      ctx.fillStyle = `rgba(255,255,255,${0.35 + 0.65 * lit})`;
      for (let i = 0; i < cities.length; i += 3) {
        if (place(cities, i, m)) {
          const k = 1.6 * dpr;
          ctx.fillRect(p[0] - k, p[1] - k, k * 2, k * 2);
        }
      }

      // The pulses themselves, brightest in the dark, and the library's mark:
      // at the centre of the whole globe, at the foot of the half one.
      ctx.lineWidth = dpr;
      for (const at of pulses) {
        ctx.strokeStyle = `rgba(255,255,255,${(1 - at) * (0.3 + 0.6 * (1 - lit))})`;
        ctx.beginPath();
        ctx.arc(cx, cy, at * reach, 0, Math.PI * 2);
        ctx.stroke();
      }
      const k = 3.5 * dpr;
      const by = half ? h - 12 * dpr : cy;
      ctx.fillStyle = "#fff";
      ctx.fillRect(cx - k, by - k, k * 2, k * 2);

      const down = t >= CUT + 0.4 && t < BACK + 0.6;
      if (down !== shownDown && netRef.current) {
        shownDown = down;
        netRef.current.textContent = down ? "Down" : "Up";
        netRef.current.dataset.state = down ? "down" : "up";
      }
    };

    const moving = () => run || dragging || Math.abs(speed) > 0.0006;

    /** The half globe's canvas is big. Where frames come slowly for a while it
     *  is made coarser, since the time goes with its size. If that does not
     *  help (a browser saving power draws at half speed whatever it is given)
     *  the canvas is put back and left alone. */
    const pace = (gap: number) => {
      if (settled || !half || gap <= 0 || gap > 100) return; // a long gap is a hidden tab, not a slow machine
      if (skip > 0) {
        skip--;
        return;
      }
      gaps += gap;
      if (++counted < 45) return;
      const mean = gaps / counted;
      gaps = counted = 0;
      if (before) {
        if (mean > before * 0.9) {
          coarse /= 0.75;
          settled = true;
          before = 0;
          fit();
          return;
        }
        before = 0;
      }
      if (mean > 24 && dpr > 1) {
        before = mean;
        coarse *= 0.75;
        fit();
      }
    };

    const tick = (now: number) => {
      frame = 0;
      pace(now - last);
      const dt = Math.min(0.06, (now - last) / 1000 || 0);
      last = now;
      if (!dragging) {
        lon += speed * dt;
        speed += ((run ? SPIN : 0) - speed) * (1 - Math.exp(-dt * 1.6));
      }
      if (run) clock += dt;
      draw();
      if (visible && moving()) frame = requestAnimationFrame(tick);
    };

    const wake = () => {
      if (gone || frame) return;
      last = performance.now();
      frame = requestAnimationFrame(tick);
    };

    // The button is labelled here rather than through React state: the
    // animation owns whether it is running, and nothing else needs to know.
    const label = () => {
      const b = buttonRef.current;
      if (!b) return;
      b.textContent = run ? "Pause" : "Play";
      b.setAttribute("aria-label", run ? "Pause the animation" : "Play the animation");
    };
    toggle.current = () => {
      run = !run;
      label();
      wake();
    };
    label();

    /** Sizes the canvas for the box and places the circle in it. */
    const fit = () => {
      // Sharp on a dense screen, but never more than 2880 pixels across: the
      // half globe is as wide as the window, and a 4K canvas redrawn sixty
      // times a second is work nobody would see.
      dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 2, 2880 / wide) * coarse);
      canvas.width = Math.round(wide * dpr);
      canvas.height = Math.round(tall * dpr);
      const edge = 26 * dpr; // room for the bezel's ticks
      cx = canvas.width / 2;
      if (half) {
        // As wide as the box. It sits on the bottom edge when the box is tall
        // enough to show the whole half; otherwise it hangs from the top and
        // the bottom edge cuts it a little short.
        r = canvas.width / 2 - edge;
        cy = Math.max(canvas.height, 12 * dpr + edge + r);
      } else {
        r = Math.min(canvas.width, canvas.height) / 2 - edge;
        cy = canvas.height / 2;
      }
      skip = Math.max(skip, 12);
      gaps = counted = 0;
      draw();
    };

    const resize = () => {
      const w = Math.round(holder.clientWidth);
      const h = Math.round(holder.clientHeight);
      const shape = wantsHalf();
      if (!w || !h || (w === wide && h === tall && shape === half)) return;
      // The half globe is seen from nearer the equator, so the half that shows
      // holds most of the land people live on, with the pole tipped into view.
      if (!turned && (!wide || shape !== half)) tilt = (shape ? 9 : 24) * RAD;
      wide = w;
      tall = h;
      half = shape;
      fit();
      wake();
    };

    const ro = new ResizeObserver(resize);
    ro.observe(holder);
    const io = new IntersectionObserver(([e]) => {
      visible = e.isIntersecting;
      if (visible) wake();
    });
    io.observe(holder);

    // Drag to turn it. The canvas is wider than the globe, so only the globe
    // itself takes hold.
    let px = 0;
    let py = 0;
    let pt = 0;
    const onGlobe = (e: PointerEvent) => {
      const b = canvas.getBoundingClientRect();
      const x = (e.clientX - b.left) * dpr - cx;
      const y = (e.clientY - b.top) * dpr - cy;
      const hold = r + 30 * dpr;
      return x * x + y * y < hold * hold;
    };
    const down = (e: PointerEvent) => {
      if (!onGlobe(e)) return;
      dragging = true;
      turned = true;
      px = e.clientX;
      py = e.clientY;
      pt = performance.now();
      speed = 0;
      canvas.setPointerCapture(e.pointerId);
      holder.dataset.dragging = "";
      wake();
    };
    const move = (e: PointerEvent) => {
      if (!dragging) {
        // Show the hand only where a drag would take hold.
        if (onGlobe(e) !== "over" in holder.dataset) {
          if ("over" in holder.dataset) delete holder.dataset.over;
          else holder.dataset.over = "";
        }
        return;
      }
      const now = performance.now();
      const per = dpr / Math.max(1, r);
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
    const leave = () => delete holder.dataset.over;
    canvas.addEventListener("pointerleave", leave);

    // The half globe is several times the size of the whole one and gets
    // outlines cut finer to match.
    fetch(wantsHalf() ? "/globe-wide.json" : "/globe.json")
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(String(res.status)))))
      .then((d: Packed) => {
        if (gone) return;
        land = unpack(d);
      })
      .catch(() => {
        /* the grid and the links still draw */
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
      canvas.removeEventListener("pointerdown", down);
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerup", up);
      canvas.removeEventListener("pointercancel", up);
      canvas.removeEventListener("pointerleave", leave);
    };
  }, []);

  return (
    <>
      <div className="globe" ref={box}>
        <canvas ref={canvasRef} aria-hidden="true" />
      </div>
      <div className="globe-panel">
        <p className="globe-readout" aria-hidden="true">
          <span>Internet</span>
          <span ref={netRef} data-state="up">
            Up
          </span>
          <span>Library</span>
          <span>Open</span>
        </p>
        <button ref={buttonRef} type="button" className="globe-toggle" aria-label="Pause the animation" onClick={() => toggle.current()}>
          Pause
        </button>
      </div>
    </>
  );
}
