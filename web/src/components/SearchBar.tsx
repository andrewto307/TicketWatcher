import { useState, type FormEvent } from "react";
import { api } from "../api";
import type { EventResult } from "../types";
import { formatSaleTime } from "../saleState";

export function SearchBar({ onWatchCreated }: { onWatchCreated: () => void }) {
  const [q, setQ] = useState("");
  const [results, setResults] = useState<EventResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  // Default on: ~90% of unfiltered results are already on sale, where a watch
  // fires instantly and tells the user nothing they didn't just read.
  const [upcomingOnly, setUpcomingOnly] = useState(true);

  async function doSearch(e: FormEvent) {
    e.preventDefault();
    if (!q.trim()) return;
    setLoading(true);
    setErr(null);
    try {
      setResults(await api.search(q, upcomingOnly));
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

      <label className="filter-row">
        <input
          type="checkbox"
          checked={upcomingOnly}
          onChange={(e) => setUpcomingOnly(e.target.checked)}
        />
        <span>
          Only events that haven't gone on sale yet
          <span className="muted"> — these are the ones worth watching</span>
        </span>
      </label>

      {err && <div className="error">{err}</div>}

      {results !== null && results.length === 0 && (
        <p className="muted">
          No events found.{" "}
          {upcomingOnly && "Try unchecking the filter above to include events already on sale."}
        </p>
      )}

      {results !== null && results.length > 0 && (
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
  const [added, setAdded] = useState(false);

  const alreadyOnSale = ev.availability === "onsale";
  const onsaleAt = formatSaleTime(ev.public_onsale_start);

  // What we can tell the user about this event before they commit to watching it.
  let note: string;
  if (alreadyOnSale) {
    // Accurate as of the milestone rework: milestones already true when you
    // subscribe are suppressed, so watching this sends nothing until something
    // actually changes.
    note = "Already on sale — you'll only hear from us if this changes (goes off sale, is rescheduled, or the sale closes).";
  } else if (onsaleAt) {
    note = `Official sale opens ${onsaleAt}.`;
  } else if (ev.onsale_tbd) {
    note = "Onsale date not announced yet — we'll alert you when it is.";
  } else {
    note = "Not currently on sale.";
  }

  async function addWatch() {
    setBusy(true);
    try {
      await api.createWatch({ tm_event_id: ev.tm_event_id, condition_type: "becomes_available" });
      setAdded(true);
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
        <span className={alreadyOnSale ? "warn-text" : "status"}>
          {alreadyOnSale && "⚠ "}
          {note}
        </span>
        {ev.presale_count > 0 && (
          <span className="muted" style={{ fontSize: "0.8rem" }}>
            {ev.presale_count} presale{ev.presale_count === 1 ? "" : "s"}
            {ev.earliest_presale_start && <> · earliest {formatSaleTime(ev.earliest_presale_start)}</>}
          </span>
        )}
      </div>
      <div className="result-actions">
        <button onClick={addWatch} disabled={busy || added}>
          {added ? "✓ Watching" : busy ? "…" : "＋ Watch"}
        </button>
      </div>
    </li>
  );
}
