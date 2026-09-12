import type { Snapshot } from "../types";

// Replaces the old PriceChart. Snapshots are written only when availability
// *changes* (see plan/04-data-model.md), so this is already a list of
// transitions — exactly what matters for an on-sale watcher. "Went on sale at
// 10:02" is the product; a chart would be the wrong shape for it.
export function StatusHistory({ snapshots }: { snapshots: Snapshot[] }) {
  if (snapshots.length === 0) {
    return (
      <p className="muted">
        No changes recorded yet — an entry is added when the event's status changes.
      </p>
    );
  }

  // Newest first: the current state is the thing you look for.
  const rows = [...snapshots].reverse();

  return (
    <ul className="status-history">
      {rows.map((s, i) => (
        <li key={`${s.checked_at}-${i}`}>
          <span className={`badge status-${s.availability ?? "unknown"}`}>
            {s.availability ?? "unknown"}
          </span>
          <span className="muted">{new Date(s.checked_at).toLocaleString()}</span>
          {i === 0 && <span className="muted">· current</span>}
        </li>
      ))}
    </ul>
  );
}
