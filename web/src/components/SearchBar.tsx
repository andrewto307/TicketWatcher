import { useState, type FormEvent } from "react";
import { Search } from "lucide-react";
import { Button } from "../ui/Button";

/**
 * The search input, in one of two positions. The move is driven by whether a
 * search has been *submitted*, not by whether it returned anything — otherwise
 * an empty result set leaves the bar centred and the button looks broken.
 */
export function SearchBar({
  value,
  onChange,
  upcomingOnly,
  onUpcomingChange,
  onSubmit,
  loading,
  compact,
}: {
  value: string;
  onChange: (v: string) => void;
  upcomingOnly: boolean;
  onUpcomingChange: (v: boolean) => void;
  onSubmit: (e: FormEvent) => void;
  loading: boolean;
  compact: boolean;
}) {
  return (
    <div className={compact ? "flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4" : "w-full"}>
      <form
        onSubmit={onSubmit}
        role="search"
        className={compact ? "flex w-full gap-2 sm:max-w-md" : "flex w-full gap-2"}
      >
        <div className="relative flex-1">
          <Search
            aria-hidden
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted"
          />
          <input
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder="Search artists, teams, shows…"
            aria-label="Search events"
            className={`w-full rounded-lg border border-line bg-surface pr-3 pl-9 text-ink
                        placeholder:text-muted transition-colors hover:border-line-strong
                        focus:border-accent focus:outline-none ${compact ? "h-9 text-sm" : "h-12 text-base"}`}
          />
        </div>
        <Button type="submit" variant="primary" size={compact ? "md" : "lg"} disabled={loading}>
          {loading ? "Searching…" : "Search"}
        </Button>
      </form>

      <label className={`flex items-center gap-2 text-sm text-muted ${compact ? "" : "mt-3"}`}>
        <input
          type="checkbox"
          checked={upcomingOnly}
          onChange={(e) => onUpcomingChange(e.target.checked)}
          className="size-4 accent-[var(--c-accent)]"
        />
        <span className="text-ink">Only events that haven't gone on sale yet</span>
      </label>
    </div>
  );
}
