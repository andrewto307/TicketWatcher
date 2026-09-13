import { useState } from "react";
import { api } from "../api";
import type { Snapshot, Watch } from "../types";
import { StatusHistory } from "./StatusHistory";
import { saleCopy, formatSaleTime } from "../saleState";

export function WatchCard({ watch, onChanged }: { watch: Watch; onChanged: () => void }) {
  const [open, setOpen] = useState(false);
  const [history, setHistory] = useState<Snapshot[] | null>(null);

  async function toggleHistory() {
    const next = !open;
    setOpen(next);
    if (next && history === null) {
      try {
        setHistory(await api.history(watch.id));
      } catch {
        setHistory([]);
      }
    }
  }

  async function togglePause() {
    await api.updateWatch(watch.id, { status: watch.status === "paused" ? "active" : "paused" });
    onChanged();
  }

  async function remove() {
    if (!confirm(`Delete watch on "${watch.event_name}"?`)) return;
    await api.deleteWatch(watch.id);
    onChanged();
  }

  const date = watch.event_date ? new Date(watch.event_date).toLocaleDateString() : "date TBA";
  const copy = saleCopy(watch);

  return (
    <div className="card">
      <div className="card-head">
        <div>
          <strong>{watch.event_name}</strong>
          <div className="muted">{watch.venue || "—"} · {date}</div>
        </div>
        <span className={`badge sale-${copy.tone}`}>{copy.label}</span>
      </div>

      <div className={`sale-box sale-${copy.tone}`}>
        <p className="sale-headline">{copy.headline}</p>
        {copy.guidance && <p className="sale-guidance">{copy.guidance}</p>}
        {copy.cta && watch.event_url && (
          <a className="sale-cta" href={watch.event_url} target="_blank" rel="noopener noreferrer">
            {copy.cta} →
          </a>
        )}
      </div>

      {/* The presale list is the closest thing we have to a resale signal: if a
          presale has already opened, tickets are likely circulating. */}
      {watch.presale_count > 0 && (
        <p className="muted sale-meta">
          {watch.presale_count} presale{watch.presale_count === 1 ? "" : "s"}
          {watch.earliest_presale && <> · earliest {formatSaleTime(watch.earliest_presale)}</>}
          {watch.earliest_presale_name && <> ({watch.earliest_presale_name})</>}
        </p>
      )}

      <div className="card-actions">
        <button onClick={toggleHistory}>{open ? "Hide history" : "Status history"}</button>
        <button onClick={togglePause}>{watch.status === "paused" ? "Resume" : "Pause"}</button>
        <button className="danger" onClick={remove}>Delete</button>
      </div>

      {open && (
        <div className="card-history">
          {history === null ? <p className="muted">Loading…</p> : <StatusHistory snapshots={history} />}
        </div>
      )}
    </div>
  );
}
