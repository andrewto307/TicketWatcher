import { useState } from "react";
import { api } from "../api";
import type { Snapshot, Watch } from "../types";
import { PriceChart } from "./PriceChart";

export function WatchCard({ watch, onChanged }: { watch: Watch; onChanged: () => void }) {
  const [open, setOpen] = useState(false);
  const [history, setHistory] = useState<Snapshot[] | null>(null);
  const [editing, setEditing] = useState(false);
  const [thr, setThr] = useState(watch.threshold?.toString() ?? "");

  async function toggleChart() {
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

  async function saveThreshold() {
    await api.updateWatch(watch.id, { threshold: Number(thr) });
    setEditing(false);
    onChanged();
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
          <span className="muted">current</span>
          <span className="metric-val">
            {watch.current_min_price != null ? `$${watch.current_min_price.toFixed(2)}` : "—"}
          </span>
          <span className="muted">{watch.availability ?? ""}</span>
        </div>
        <div className="metric">
          <span className="muted">condition</span>
          {watch.condition_type === "price_below" ? (
            editing ? (
              <span className="edit-thr">
                below $
                <input className="thr" type="number" value={thr} onChange={(e) => setThr(e.target.value)} />
                <button onClick={saveThreshold}>Save</button>
              </span>
            ) : (
              <span>
                below <strong>${watch.threshold?.toFixed(2)}</strong>{" "}
                <button className="link" onClick={() => setEditing(true)}>edit</button>
              </span>
            )
          ) : (
            <span>becomes available</span>
          )}
        </div>
      </div>

      <div className="card-actions">
        <button onClick={toggleChart}>{open ? "Hide history" : "Price history"}</button>
        <button onClick={togglePause}>{watch.status === "paused" ? "Resume" : "Pause"}</button>
        <button className="danger" onClick={remove}>Delete</button>
      </div>

      {open && (
        <div className="chart-wrap">
          {history === null ? (
            <p className="muted">Loading…</p>
          ) : history.length === 0 ? (
            <p className="muted">No price history yet — a point is recorded when the price changes.</p>
          ) : (
            <PriceChart data={history} threshold={watch.threshold} />
          )}
        </div>
      )}
    </div>
  );
}
