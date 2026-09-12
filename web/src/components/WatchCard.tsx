import { useState } from "react";
import { api } from "../api";
import type { Snapshot, Watch } from "../types";
import { StatusHistory } from "./StatusHistory";

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

  const polled = watch.last_polled_at != null;
  const status = watch.availability ?? "unknown";

  // Already on sale when the watch was created: the condition is true from the
  // first poll, so it fires immediately and tells the user nothing they didn't
  // already see in the search results. Worth saying, since ~90% of events are
  // already onsale and this is the default path into a useless alert.
  const alreadyOnSale = polled && status === "onsale";

  return (
    <div className="card">
      <div className="card-head">
        <div>
          <strong>{watch.event_name}</strong>
          <div className="muted">{watch.venue || "—"} · {date}</div>
        </div>
        <span className={`badge ${watch.status}`}>{watch.status}</span>
      </div>

      <div className="card-body">
        <div className="metric">
          <span className="muted">status</span>
          <span className="metric-val">{polled ? status : "checking…"}</span>
        </div>
        <div className="metric">
          <span className="muted">alerts when</span>
          <span>it goes on sale</span>
        </div>
      </div>

      {alreadyOnSale && (
        <div className="notice warn">
          <span>
            <strong>This event is already on sale.</strong> You'll only be alerted if it goes off
            sale and returns. Watches are most useful on events that haven't opened yet.
          </span>
        </div>
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
