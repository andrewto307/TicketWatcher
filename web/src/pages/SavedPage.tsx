import { Link } from "react-router-dom";
import { Bookmark } from "lucide-react";
import { useAppData } from "../AppData";
import { WatchCard } from "../components/WatchCard";
import { Spinner } from "../ui/Spinner";

export function SavedPage() {
  const { saved, alertsByWatch, loading, refresh } = useAppData();

  return (
    <div className="py-6">
      <header className="mb-1 flex items-baseline gap-2">
        <h1 className="text-xl font-semibold text-ink">Saved</h1>
        <span className="text-sm text-muted">{saved.length}</span>
      </header>
      <p className="mb-4 text-sm text-muted">
        Events you're holding onto. These aren't being tracked, so no alerts are sent for them.
      </p>

      {loading && saved.length === 0 ? (
        <Spinner label="Loading saved events…" />
      ) : saved.length === 0 ? (
        <div className="rounded-xl border border-dashed border-line px-6 py-12 text-center">
          <Bookmark aria-hidden className="mx-auto size-6 text-muted" />
          <p className="mt-3 font-medium text-ink">You haven't saved any events yet.</p>
          <p className="mt-1 text-sm text-muted">
            Save an event from search, or pause one you're watching.
          </p>
          <Link
            to="/"
            className="mt-4 inline-flex h-9 items-center rounded-lg bg-accent px-4 text-sm font-medium
                       text-accent-fg transition-colors hover:bg-accent-hover"
          >
            Search events
          </Link>
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {saved.map((w) => (
            <WatchCard key={w.id} watch={w} alerts={alertsByWatch.get(w.id)} onChanged={refresh} />
          ))}
        </div>
      )}
    </div>
  );
}
