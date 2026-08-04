import type { Watch } from "../types";
import { WatchCard } from "./WatchCard";

export function WatchList({ watches, onChanged }: { watches: Watch[]; onChanged: () => void }) {
  if (watches.length === 0) {
    return <p className="muted">No watches yet — search above and add one.</p>;
  }
  return (
    <div className="watchlist">
      {watches.map((w) => (
        <WatchCard key={w.id} watch={w} onChanged={onChanged} />
      ))}
    </div>
  );
}
