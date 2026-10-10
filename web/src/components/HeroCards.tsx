import { Bell, Ticket } from "lucide-react";
import { Badge } from "../ui/Badge";

/**
 * The hero's right-hand side. Built from the real components rather than a
 * screenshot: it stays crisp at any pixel density, it is readable at this size
 * in a way a shrunken screenshot is not, and it shows the product's actual
 * output instead of a picture of it.
 */
const cards = [
  {
    tone: "good" as const,
    label: "On sale",
    name: "Aurora Nights Festival",
    line: "Tickets are on sale to the general public right now.",
    offset: "lg:-translate-x-6",
    tilt: "lg:rotate-[-1.5deg]",
  },
  {
    tone: "good" as const,
    label: "Presale open",
    name: "The Midnight Orchestra",
    line: "The Artist Presale is open now.",
    offset: "lg:translate-x-4",
    tilt: "lg:rotate-[1deg]",
  },
  {
    tone: "muted" as const,
    label: "Opens later",
    name: "Riverside Classics",
    line: "The official sale opens Fri, 10:00 AM.",
    offset: "lg:-translate-x-2",
    tilt: "lg:rotate-[-0.5deg]",
  },
];

export function HeroCards() {
  return (
    <div aria-hidden className="flex flex-col gap-3">
      {cards.map((c) => (
        <div
          key={c.name}
          className={`rounded-xl border border-line bg-surface p-3.5 shadow-lg
                      transition-transform duration-200 ${c.offset} ${c.tilt}`}
        >
          <div className="flex items-start justify-between gap-3">
            <span className="flex min-w-0 items-center gap-2">
              <span className="grid size-7 shrink-0 place-items-center rounded-lg bg-accent-soft text-accent">
                <Ticket className="size-3.5" />
              </span>
              <span className="truncate text-sm font-semibold text-ink">{c.name}</span>
            </span>
            <Badge tone={c.tone}>{c.label}</Badge>
          </div>
          <p className="mt-2 flex items-start gap-1.5 text-xs text-muted">
            <Bell className="mt-0.5 size-3 shrink-0 text-good" />
            {c.line}
          </p>
        </div>
      ))}
    </div>
  );
}
