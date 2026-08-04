import type { Snapshot } from "../types";

// A dependency-free SVG step-line chart of min price over time, with a dashed
// reference line at the watch's threshold. Step-line fits our on-change snapshots
// (the price holds until the next recorded change).
export function PriceChart({ data, threshold }: { data: Snapshot[]; threshold: number | null }) {
  const points = data
    .filter((d) => d.min_price != null)
    .map((d) => ({ t: new Date(d.checked_at).getTime(), v: d.min_price as number }));

  if (points.length === 0) {
    return <p className="muted">No priced points to chart yet.</p>;
  }

  const W = 560;
  const H = 180;
  const pad = 34;

  const ts = points.map((p) => p.t);
  const vs = points.map((p) => p.v);
  if (threshold != null) vs.push(threshold);

  const tMin = Math.min(...ts);
  const tMax = Math.max(...ts);
  const vMin = Math.min(...vs);
  const vMax = Math.max(...vs);

  const x = (t: number) =>
    pad + (tMax === tMin ? 0.5 : (t - tMin) / (tMax - tMin)) * (W - 2 * pad);
  const y = (v: number) =>
    H - pad - (vMax === vMin ? 0.5 : (v - vMin) / (vMax - vMin)) * (H - 2 * pad);

  let d = `M ${x(points[0].t)} ${y(points[0].v)}`;
  for (let i = 1; i < points.length; i++) {
    d += ` L ${x(points[i].t)} ${y(points[i - 1].v)} L ${x(points[i].t)} ${y(points[i].v)}`;
  }

  return (
    <div className="chart-scroll">
      <svg className="chart" viewBox={`0 0 ${W} ${H}`} width="100%" role="img" aria-label="price history">
        {threshold != null && (
          <>
            <line x1={pad} y1={y(threshold)} x2={W - pad} y2={y(threshold)} className="threshold-line" />
            <text x={W - pad} y={y(threshold) - 5} textAnchor="end" className="threshold-label">
              threshold ${threshold.toFixed(0)}
            </text>
          </>
        )}
        <path d={d} className="price-line" fill="none" />
        {points.map((p, i) => (
          <circle key={i} cx={x(p.t)} cy={y(p.v)} r={2.5} className="price-dot" />
        ))}
        <text x={pad} y={H - 10} className="axis">low ${vMin.toFixed(0)}</text>
        <text x={W - pad} y={pad - 8} textAnchor="end" className="axis">high ${vMax.toFixed(0)}</text>
      </svg>
    </div>
  );
}
