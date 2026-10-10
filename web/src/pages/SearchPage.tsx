import { useMemo, useState, type FormEvent } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { api } from "../api";
import type { EventResult } from "../types";
import { useAppData } from "../AppData";
import { SearchBar } from "../components/SearchBar";
import { SearchResultCard } from "../components/SearchResultCard";
import { Wordmark } from "../components/Wordmark";
import { Card } from "../ui/Card";
import { Spinner } from "../ui/Spinner";

/** One attraction with every date it is playing. */
interface Group {
  key: string;
  name: string;
  venue: string;
  events: EventResult[];
}

// Ticketmaster returns residency runs as one row per night: a search for "rock"
// comes back as fourteen separate John Mayer entries. Collapsing them by name
// keeps the result list readable.
function group(events: EventResult[]): Group[] {
  const out: Group[] = [];
  const index = new Map<string, Group>();
  for (const ev of events) {
    const key = ev.name.trim().toLowerCase();
    const existing = index.get(key);
    if (existing) {
      existing.events.push(ev);
    } else {
      const g: Group = { key, name: ev.name, venue: ev.venue, events: [ev] };
      index.set(key, g);
      out.push(g);
    }
  }
  return out;
}

export function SearchPage() {
  const { trackedIds, refresh } = useAppData();
  const [q, setQ] = useState("");
  const [upcomingOnly, setUpcomingOnly] = useState(true);
  const [results, setResults] = useState<EventResult[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  // Drives the layout, and is set on submit so a zero-result search still moves
  // the bar up instead of looking like the button did nothing.
  const [searched, setSearched] = useState(false);

  const groups = useMemo(() => (results ? group(results) : []), [results]);

  async function doSearch(e: FormEvent) {
    e.preventDefault();
    if (!q.trim()) return;
    setSearched(true);
    setLoading(true);
    setErr(null);
    try {
      setResults(await api.search(q, upcomingOnly));
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
      setResults([]);
    } finally {
      setLoading(false);
    }
  }

  const bar = (
    <SearchBar
      value={q}
      onChange={setQ}
      upcomingOnly={upcomingOnly}
      onUpcomingChange={setUpcomingOnly}
      onSubmit={doSearch}
      loading={loading}
      compact={searched}
    />
  );

  // Before any search the bar is the whole page, not a header above empty space.
  if (!searched) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-6 py-10">
        <Wordmark size="lg" />
        <p className="max-w-md text-center text-muted">
          Search for an event and we'll email you the moment tickets go on sale.
        </p>
        <div className="w-full max-w-xl">{bar}</div>
      </div>
    );
  }

  return (
    <div className="py-6">
      <div className="border-b border-line pb-4">{bar}</div>

      {loading && (
        <div className="py-10 text-center">
          <Spinner label="Searching Ticketmaster…" />
        </div>
      )}

      {!loading && err && (
        <div className="mt-6 rounded-lg border border-warn-border bg-warn-soft px-4 py-3 text-sm text-warn">
          {err}
        </div>
      )}

      {!loading && !err && results && results.length === 0 && (
        <div className="py-12 text-center">
          <p className="font-medium text-ink">No events found.</p>
          {upcomingOnly && (
            <p className="mt-1 text-sm text-muted">
              Try unchecking the filter to include events already on sale.
            </p>
          )}
        </div>
      )}

      {!loading && groups.length > 0 && (
        <ul className="mt-4 flex flex-col gap-3">
          {groups.map((g) =>
            g.events.length === 1 ? (
              <li key={g.key}>
                <SearchResultCard
                  ev={g.events[0]}
                  tracked={trackedIds.get(g.events[0].tm_event_id)}
                  onChanged={refresh}
                />
              </li>
            ) : (
              <li key={g.key}>
                <GroupCard group={g} />
              </li>
            ),
          )}
        </ul>
      )}
    </div>
  );
}

function GroupCard({ group: g }: { group: Group }) {
  const { trackedIds, refresh } = useAppData();
  const [open, setOpen] = useState(false);

  return (
    <Card className="animate-fade-up overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center gap-3 p-4 text-left transition-colors hover:bg-surface-2"
      >
        {open ? (
          <ChevronDown aria-hidden className="size-4 shrink-0 text-muted" />
        ) : (
          <ChevronRight aria-hidden className="size-4 shrink-0 text-muted" />
        )}
        <span className="min-w-0 flex-1">
          <span className="block font-semibold text-ink">{g.name}</span>
          <span className="block text-sm text-muted">{g.venue || "—"}</span>
        </span>
        <span className="shrink-0 text-sm text-muted">{g.events.length} dates</span>
      </button>

      {open && (
        <ul className="flex flex-col gap-2 border-t border-line bg-surface-2/40 p-3">
          {g.events.map((ev) => (
            <li key={ev.tm_event_id}>
              <SearchResultCard
                ev={ev}
                tracked={trackedIds.get(ev.tm_event_id)}
                onChanged={refresh}
              />
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
