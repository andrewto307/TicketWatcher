import { useState } from "react";
import { Bookmark, Check, ExternalLink, Plus } from "lucide-react";
import { api } from "../api";
import type { EventResult } from "../types";
import { formatSaleTime } from "../saleState";
import { Button } from "../ui/Button";
import { Card } from "../ui/Card";
import { useToast } from "../ui/Toast";

export type Tracked = "watching" | "saved" | undefined;

export function SearchResultCard({
  ev,
  tracked,
  onChanged,
}: {
  ev: EventResult;
  tracked: Tracked;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const { toast } = useToast();

  const alreadyOnSale = ev.availability === "onsale";
  const onsaleAt = formatSaleTime(ev.public_onsale_start);

  // What we can tell the user about this event before they commit to watching it.
  let note: string;
  if (alreadyOnSale) {
    // Milestones already true when you subscribe are suppressed, so watching
    // this sends nothing until something actually changes.
    note =
      "Already on sale — you'll only hear from us if this changes (goes off sale, is rescheduled, or the sale closes).";
  } else if (onsaleAt) {
    note = `Official sale opens ${onsaleAt}.`;
  } else if (ev.onsale_tbd) {
    note = "Onsale date not announced yet — we'll alert you when it is.";
  } else {
    note = "Not currently on sale.";
  }

  // Save is Watch plus an immediate pause: the create endpoint always makes an
  // active watch, and status is set through PATCH.
  async function add(as: "watching" | "saved") {
    setBusy(true);
    try {
      const w = await api.createWatch({
        tm_event_id: ev.tm_event_id,
        condition_type: "becomes_available",
      });
      if (as === "saved") await api.updateWatch(w.id, { status: "paused" });
      onChanged();
      toast(
        as === "saved" ? `Saved "${ev.name}".` : `Watching "${ev.name}".`,
        { label: as === "saved" ? "View saved" : "View watches", onClick: () => {
          window.location.href = as === "saved" ? "/saved" : "/watches";
        } },
      );
    } catch (e) {
      toast("Could not add: " + String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="animate-fade-up flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold text-ink">{ev.name}</h3>
        <p className="mt-0.5 text-sm text-muted">
          {ev.venue || "—"} ·{" "}
          {ev.event_date ? new Date(ev.event_date).toLocaleDateString() : "date TBA"}
        </p>
        <p className={`mt-1 text-sm ${alreadyOnSale ? "text-warn" : "text-muted"}`}>
          {alreadyOnSale && "⚠ "}
          {note}
        </p>
        {ev.presale_count > 0 && (
          <p className="mt-1 text-xs text-muted">
            {ev.presale_count} presale{ev.presale_count === 1 ? "" : "s"}
            {ev.earliest_presale_start && <> · earliest {formatSaleTime(ev.earliest_presale_start)}</>}
          </p>
        )}
      </div>

      <div className="flex shrink-0 flex-wrap items-center gap-2">
        {ev.url && (
          <a
            href={ev.url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Open ${ev.name} on Ticketmaster`}
            className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-line bg-surface
                       px-3 text-sm font-medium text-ink transition-colors hover:bg-surface-2"
          >
            <ExternalLink aria-hidden className="size-4 text-muted" />
            Ticketmaster
          </a>
        )}

        {/* Already tracked: say so rather than offering to add it twice. */}
        {tracked ? (
          <span className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-good-border
                           bg-good-soft px-3 text-sm font-medium text-good">
            <Check aria-hidden className="size-4" />
            {tracked === "saved" ? "Saved" : "Watching"}
          </span>
        ) : (
          <>
            <Button variant="primary" onClick={() => add("watching")} disabled={busy}>
              <Plus aria-hidden className="size-4" />
              Watch
            </Button>
            <Button onClick={() => add("saved")} disabled={busy}>
              <Bookmark aria-hidden className="size-4 text-muted" />
              Save
            </Button>
          </>
        )}
      </div>
    </Card>
  );
}
