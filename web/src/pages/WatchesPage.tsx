import { Link } from "react-router-dom";
import { Eye } from "lucide-react";
import { useAppData } from "../AppData";
import { WatchCard } from "../components/WatchCard";
import { Spinner } from "../ui/Spinner";

export function WatchesPage() {
  const { watches, alertsByWatch, loading, refresh } = useAppData();

  return (
    <div className="py-6">
      <header className="mb-4 flex items-baseline gap-2">
        <h1 className="text-xl font-semibold text-ink">My watches</h1>
        <span className="text-sm text-muted">{watches.length}</span>
      </header>

      {loading && watches.length === 0 ? (
        <Spinner label="Loading your watches…" />
      ) : watches.length === 0 ? (
        <EmptyWatches />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {watches.map((w) => (
            <WatchCard key={w.id} watch={w} alerts={alertsByWatch.get(w.id)} onChanged={refresh} />
          ))}
        </div>
      )}
    </div>
  );
}

function EmptyWatches() {
  return (
    <div className="rounded-xl border border-dashed border-line px-6 py-12 text-center">
      <Eye aria-hidden className="mx-auto size-6 text-muted" />
      <p className="mt-3 font-medium text-ink">You don't have any active watches yet.</p>
      <p className="mt-1 text-sm text-muted">
        Find an event and we'll email you when it goes on sale.
      </p>
      <Link
        to="/"
        className="mt-4 inline-flex h-9 items-center rounded-lg bg-accent px-4 text-sm font-medium
                   text-accent-fg transition-colors hover:bg-accent-hover"
      >
        Search events
      </Link>
    </div>
  );
}
