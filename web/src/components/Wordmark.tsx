import { Ticket } from "lucide-react";

/** The mark, used in the header and on the landing hero. */
export function Wordmark({
  size = "md",
  /** Drops the text below `sm`, leaving the mark — for headers that are tight
      on a phone. The link still has an accessible name from its aria-label. */
  collapsible = false,
}: {
  size?: "md" | "lg";
  collapsible?: boolean;
}) {
  const lg = size === "lg";
  return (
    <span className="inline-flex items-center gap-2">
      <span
        aria-hidden
        className={`grid place-items-center rounded-lg bg-accent text-accent-fg ${lg ? "size-9" : "size-7"}`}
      >
        <Ticket className={lg ? "size-5" : "size-4"} />
      </span>
      <span
        className={`font-semibold tracking-tight text-ink ${lg ? "text-2xl" : "text-base"} ${
          collapsible ? "hidden sm:inline" : ""
        }`}
      >
        TicketWatcher
      </span>
    </span>
  );
}
