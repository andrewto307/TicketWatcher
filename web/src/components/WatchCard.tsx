import { useState } from "react";
import { Bell, BellOff, ExternalLink, History, Pause, Trash2, Eye } from "lucide-react";
import { api } from "../api";
import type { Notification, Snapshot, Watch } from "../types";
import { StatusHistory } from "./StatusHistory";
import { saleCopy, formatSaleTime } from "../saleState";
import { Button } from "../ui/Button";
import { Badge } from "../ui/Badge";
import { Card } from "../ui/Card";
import { Spinner } from "../ui/Spinner";
import { useToast } from "../ui/Toast";

const boxTone = {
  good: "border-good-border bg-good-soft",
  warn: "border-warn-border bg-warn-soft",
  muted: "border-line bg-surface-2",
} as const;

export function WatchCard({
  watch,
  alerts = [],
  onChanged,
}: {
  watch: Watch;
  alerts?: Notification[];
  onChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [history, setHistory] = useState<Snapshot[] | null>(null);
  const [busy, setBusy] = useState(false);
  const { toast } = useToast();

  const isSaved = watch.status === "paused";

  async function toggleHistory() {
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

  // The card leaves the list the user is looking at, so both directions
  // confirm with a toast and offer the way back.
  async function move() {
    setBusy(true);
    try {
      const to = isSaved ? "active" : "paused";
      await api.updateWatch(watch.id, { status: to });
      onChanged();
      toast(
        isSaved ? `Watching "${watch.event_name}" again.` : `Moved "${watch.event_name}" to Saved.`,
        {
          label: "Undo",
          onClick: async () => {
            await api.updateWatch(watch.id, { status: isSaved ? "paused" : "active" });
            onChanged();
          },
        },
      );
    } catch (e) {
      toast(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm(`Delete watch on "${watch.event_name}"?`)) return;
    setBusy(true);
    try {
      await api.deleteWatch(watch.id);
      onChanged();
      toast(`Deleted "${watch.event_name}".`);
    } finally {
      setBusy(false);
    }
  }

  const date = watch.event_date ? new Date(watch.event_date).toLocaleDateString() : "date TBA";
  const copy = saleCopy(watch);
  const latest = alerts[0];

  return (
    <Card className="animate-fade-up flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="font-semibold text-ink">{watch.event_name}</h3>
          <p className="mt-0.5 text-sm text-muted">
            {watch.venue || "—"} · {date}
          </p>
        </div>
        <Badge tone={copy.tone}>{copy.label}</Badge>
      </div>

      <div className={`rounded-lg border p-3 ${boxTone[copy.tone]}`}>
        <p className="text-sm font-medium text-ink">{copy.headline}</p>
        {copy.guidance && <p className="mt-1 text-sm text-muted">{copy.guidance}</p>}
        {copy.cta && watch.event_url && (
          <a
            href={watch.event_url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-sm font-medium text-accent
                       underline-offset-4 hover:underline"
          >
            {copy.cta}
            <ExternalLink aria-hidden className="size-3.5" />
          </a>
        )}
      </div>

      {/* The presale list is the closest thing we have to a resale signal: if a
          presale has already opened, tickets are likely circulating. */}
      {watch.presale_count > 0 && (
        <p className="text-xs text-muted">
          {watch.presale_count} presale{watch.presale_count === 1 ? "" : "s"}
          {watch.earliest_presale && <> · earliest {formatSaleTime(watch.earliest_presale)}</>}
          {watch.earliest_presale_name && <> ({watch.earliest_presale_name})</>}
        </p>
      )}

      {/* Alert history lives next to the event it belongs to, rather than in a
          separate chronological list the user has to cross-reference. */}
      <p className="flex items-center gap-1.5 text-xs text-muted">
        {latest ? (
          <>
            <Bell aria-hidden className="size-3.5 text-good" />
            <span>
              {alerts.length} alert{alerts.length === 1 ? "" : "s"} sent · latest{" "}
              {String(latest.payload?.subject ?? "alert")} on{" "}
              {new Date(latest.sent_at).toLocaleString()}
            </span>
          </>
        ) : (
          <>
            <BellOff aria-hidden className="size-3.5" />
            <span>No alerts sent yet.</span>
          </>
        )}
      </p>

      <div className="flex flex-wrap items-center gap-2 border-t border-line pt-3">
        {watch.event_url && (
          <a
            href={watch.event_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-line bg-surface
                       px-3 text-sm font-medium text-ink transition-colors hover:bg-surface-2"
          >
            <ExternalLink aria-hidden className="size-4 text-muted" />
            Ticketmaster
          </a>
        )}
        <Button onClick={toggleHistory} aria-expanded={open}>
          <History aria-hidden className="size-4 text-muted" />
          {open ? "Hide history" : "Status history"}
        </Button>
        <Button onClick={move} disabled={busy}>
          {isSaved ? <Eye aria-hidden className="size-4 text-muted" /> : <Pause aria-hidden className="size-4 text-muted" />}
          {isSaved ? "Watch" : "Pause"}
        </Button>
        <Button variant="danger" onClick={remove} disabled={busy}>
          <Trash2 aria-hidden className="size-4" />
          Delete
        </Button>
      </div>

      {open && (
        <div className="animate-fade-up border-t border-line pt-3">
          {history === null ? <Spinner label="Loading…" /> : <StatusHistory snapshots={history} />}
        </div>
      )}
    </Card>
  );
}
