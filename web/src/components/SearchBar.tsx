import { useState, type FormEvent } from "react";
import { api } from "../api";
import type { EventResult } from "../types";

export function SearchBar({ onWatchCreated }: { onWatchCreated: () => void }) {
  const [q, setQ] = useState("");
  const [results, setResults] = useState<EventResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function doSearch(e: FormEvent) {
    e.preventDefault();
    if (!q.trim()) return;
    setLoading(true);
    setErr(null);
    try {
      setResults(await api.search(q));
    } catch (e) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <form onSubmit={doSearch} className="searchbar">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search artists, teams, shows…"
        />
        <button type="submit" disabled={loading}>
          {loading ? "Searching…" : "Search"}
        </button>
      </form>
      {err && <div className="error">{err}</div>}
      {results.length > 0 && (
        <ul className="results">
          {results.map((ev) => (
            <SearchResult key={ev.tm_event_id} ev={ev} onWatchCreated={onWatchCreated} />
          ))}
        </ul>
      )}
    </div>
  );
}

function SearchResult({ ev, onWatchCreated }: { ev: EventResult; onWatchCreated: () => void }) {
  // With no published price, a price_below watch can never trigger. Default such
  // events to the condition that does work instead of letting someone set up a
  // watch that silently does nothing.
  const hasPrice = ev.min_price != null;
  const [condition, setCondition] = useState(hasPrice ? "price_below" : "becomes_available");
  const [threshold, setThreshold] = useState("");
  const [busy, setBusy] = useState(false);

  async function addWatch() {
    setBusy(true);
    try {
      await api.createWatch({
        tm_event_id: ev.tm_event_id,
        condition_type: condition,
        threshold: condition === "price_below" ? Number(threshold) : undefined,
      });
      onWatchCreated();
    } catch (e) {
      alert("Could not create watch: " + String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="result">
      <div className="result-info">
        <strong>{ev.name}</strong>
        <span className="muted">
          {ev.venue || "—"} · {ev.event_date ? new Date(ev.event_date).toLocaleDateString() : "date TBA"}
        </span>
        <span className="price">
          {hasPrice ? `from $${ev.min_price!.toFixed(2)}` : "no price published"} · {ev.availability}
        </span>
        {!hasPrice && condition === "price_below" && (
          <span className="warn-text">
            ⚠ Ticketmaster publishes no price for this event, so a price alert can't trigger. Use
            “Becomes available” instead.
          </span>
        )}
      </div>
      <div className="result-actions">
        <select value={condition} onChange={(e) => setCondition(e.target.value)}>
          <option value="price_below">Price below{hasPrice ? "" : " (no price data)"}</option>
          <option value="becomes_available">Becomes available</option>
        </select>
        {condition === "price_below" && (
          <input
            className="thr"
            type="number"
            value={threshold}
            onChange={(e) => setThreshold(e.target.value)}
            placeholder="$"
          />
        )}
        <button
          onClick={addWatch}
          disabled={busy || (condition === "price_below" && !threshold)}
        >
          ＋ Watch
        </button>
      </div>
    </li>
  );
}
