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
  const [busy, setBusy] = useState(false);

  // Watching an event that's already on sale fires on the first poll and tells
  // the user nothing they can't see right here. Flag it rather than letting them
  // set up an alert that arrives 15 seconds later saying "it's on sale".
  const alreadyOnSale = ev.availability === "onsale";

  async function addWatch() {
    setBusy(true);
    try {
      await api.createWatch({
        tm_event_id: ev.tm_event_id,
        condition_type: "becomes_available",
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
        <span className="status">{ev.availability}</span>
        {alreadyOnSale && (
          <span className="warn-text">
            ⚠ Already on sale — you'd only be alerted if it goes off sale and returns.
          </span>
        )}
      </div>
      <div className="result-actions">
        <button onClick={addWatch} disabled={busy}>
          ＋ Watch for on-sale
        </button>
      </div>
    </li>
  );
}
