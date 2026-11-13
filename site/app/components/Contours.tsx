// A topographic contour field drawn from a few summed sine "hills", traced
// as iso-lines with marching squares at build time. Pure SVG, no runtime JS.
function field(x: number, y: number) {
  const hills = [
    [0.72, 0.38, 0.22, 1.0],
    [0.88, 0.78, 0.16, 0.7],
    [0.5, 0.95, 0.25, 0.55],
    [0.98, 0.1, 0.12, 0.5],
    [0.35, 0.2, 0.18, 0.25],
  ];
  let v = 0;
  for (const [cx, cy, r, h] of hills) {
    const d = ((x - cx) ** 2 + (y - cy) ** 2) / (r * r);
    v += h * Math.exp(-d);
  }
  return v + 0.04 * Math.sin(x * 11) * Math.cos(y * 9);
}

function contours(w: number, h: number, levels: number[], n = 90) {
  const m = Math.round((n * h) / w);
  const g: number[][] = [];
  for (let j = 0; j <= m; j++) {
    g.push([]);
    for (let i = 0; i <= n; i++) g[j].push(field(i / n, j / m));
  }
  const sx = w / n;
  const sy = h / m;
  return levels.map((lv) => {
    let d = "";
    for (let j = 0; j < m; j++)
      for (let i = 0; i < n; i++) {
        const a = g[j][i], b = g[j][i + 1], c = g[j + 1][i + 1], e = g[j + 1][i];
        const pts: [number, number][] = [];
        const lerp = (p: number, q: number) => (lv - p) / (q - p);
        if ((a > lv) !== (b > lv)) pts.push([(i + lerp(a, b)) * sx, j * sy]);
        if ((b > lv) !== (c > lv)) pts.push([(i + 1) * sx, (j + lerp(b, c)) * sy]);
        if ((e > lv) !== (c > lv)) pts.push([(i + lerp(e, c)) * sx, (j + 1) * sy]);
        if ((a > lv) !== (e > lv)) pts.push([i * sx, (j + lerp(a, e)) * sy]);
        if (pts.length >= 2) d += `M${pts[0][0].toFixed(1)} ${pts[0][1].toFixed(1)}L${pts[1][0].toFixed(1)} ${pts[1][1].toFixed(1)}`;
        if (pts.length === 4) d += `M${pts[2][0].toFixed(1)} ${pts[2][1].toFixed(1)}L${pts[3][0].toFixed(1)} ${pts[3][1].toFixed(1)}`;
      }
    return d;
  });
}

export default function Contours() {
  const w = 1200;
  const h = 720;
  const levels = Array.from({ length: 16 }, (_, i) => 0.06 + i * 0.065);
  const paths = contours(w, h, levels);
  return (
    <svg className="contours" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="xMidYMid slice" aria-hidden="true">
      {paths.map((d, i) => (
        <path key={i} d={d} className={i % 5 === 4 ? "index" : ""} style={{ animationDelay: `${i * 70}ms` }} />
      ))}
      <g className="survey">
        <circle cx="864" cy="274" r="5" />
        <path d="M852 274h24M864 262v24" />
        <text x="878" y="268">BM 1,284</text>
      </g>
    </svg>
  );
}
