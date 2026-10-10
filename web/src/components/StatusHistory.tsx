import type { Snapshot } from "../types";
import { Badge, type Tone } from "../ui/Badge";

// Snapshots are written only when availability *changes* (see
// plan/04-data-model.md), so this is already a list of transitions — exactly
// what matters for an on-sale watcher.
const tone = (a: string | null): Tone =>
  a === "onsale" ? "good" : a === "cancelled" || a === "offsale" ? "warn" : "muted";

export function StatusHistory({ snapshots }: { snapshots: Snapshot[] }) {
  if (snapshots.length === 0) {
    return (
      <p className="text-sm text-muted">
        No changes recorded yet — an entry is added when the event's status changes.
      </p>
    );
  }

  // Newest first: the current state is the thing you look for.
  const rows = [...snapshots].reverse();

  return (
    <ul className="flex flex-col gap-2">
      {rows.map((s, i) => (
        <li key={`${s.checked_at}-${i}`} className="flex items-center gap-2 text-sm">
          <Badge tone={tone(s.availability)}>{s.availability ?? "unknown"}</Badge>
          <span className="text-muted">{new Date(s.checked_at).toLocaleString()}</span>
          {i === 0 && <span className="text-muted">· current</span>}
        </li>
      ))}
    </ul>
  );
}
