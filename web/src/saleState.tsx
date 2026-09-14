import type { SaleState, Watch } from "./types";

// All user-facing copy for sale states lives here, so the wording stays
// consistent between the search list, the watch card, and any future surface.
//
// Every state answers two questions: what's true now, and what should I do?
// The resale caveat appears wherever a user might otherwise conclude "there are
// no tickets" — Ticketmaster's public API doesn't expose resale listings, so we
// genuinely cannot know (see plan/08-sale-milestone-alerts.md).

export interface SaleCopy {
  /** Short label for a badge. */
  label: string;
  /** One line of plain fact. */
  headline: string;
  /** What the user can do about it, if anything. */
  guidance?: string;
  /** Text for the Ticketmaster link, when one helps. */
  cta?: string;
  /** Visual weight: good news, needs attention, or neutral. */
  tone: "good" | "warn" | "muted";
}

export function formatSaleTime(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  // Rendered in the reader's own timezone — unlike email, the browser knows it.
  return d.toLocaleString(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function saleCopy(w: {
  sale_state: SaleState;
  public_onsale_start: string | null;
  earliest_presale: string | null;
  earliest_presale_name: string | null;
  presale_count: number;
}): SaleCopy {
  const publicAt = formatSaleTime(w.public_onsale_start);
  const presaleAt = formatSaleTime(w.earliest_presale);
  const presaleName = w.earliest_presale_name;

  switch (w.sale_state) {
    case "checking":
      return {
        label: "Checking…",
        headline: "We haven't checked this event yet.",
        guidance: "This usually takes under a minute.",
        tone: "muted",
      };

    case "presale_open":
      return {
        label: "Presale open",
        headline: presaleName ? `The ${presaleName} presale is open now.` : "A presale is open now.",
        guidance:
          "Presales often need a code, fan-club membership, or a specific card. " +
          (publicAt ? `The general public sale opens ${publicAt}.` : "The public sale date isn't announced yet."),
        cta: "Open the presale",
        tone: "good",
      };

    case "on_sale":
      return {
        label: "On sale",
        headline: "Tickets are on sale to the general public right now.",
        guidance: "Popular events can sell out quickly.",
        cta: "Buy tickets",
        tone: "good",
      };

    case "onsale_scheduled":
      return {
        label: "Opens later",
        // Defensive: the backend only reports onsale_scheduled when it has a date,
        // but rendering "The official sale opens ." if that ever slips would look
        // broken to the user for no good reason.
        headline: publicAt
          ? `The official sale opens ${publicAt}.`
          : "The official sale hasn't opened yet.",
        guidance:
          w.presale_count > 0
            ? `We'll email you when it opens. There ${w.presale_count === 1 ? "is" : "are"} also ${w.presale_count} presale${w.presale_count === 1 ? "" : "s"}${presaleAt ? `, starting ${presaleAt}` : ""} — we'll alert you for those too.`
            : "We'll email you the moment it opens, so you don't need to keep checking.",
        cta: "See the event page",
        tone: "muted",
      };

    case "onsale_tbd":
      return {
        label: "Date not announced",
        headline: "Ticketmaster hasn't announced when this goes on sale.",
        guidance: "We'll email you as soon as a date is published, and again when the sale opens.",
        cta: "See the event page",
        tone: "muted",
      };

    case "sale_closed":
      return {
        label: "Sale closed",
        headline: "The official Ticketmaster sale has closed.",
        guidance:
          "Resale tickets may still be available from other fans — we can't see resale listings, " +
          "so this is worth checking on Ticketmaster directly.",
        cta: "Check for resale tickets",
        tone: "warn",
      };

    case "cancelled":
      return {
        label: "Cancelled",
        headline: "This event has been cancelled by the organiser.",
        guidance: "If you bought tickets, refunds are handled by Ticketmaster. We've stopped watching it.",
        cta: "See the event page",
        tone: "warn",
      };

    case "rescheduled":
      return {
        label: "Date changed",
        headline: "This event has been postponed or rescheduled.",
        guidance:
          "Existing tickets are usually still valid — check the event page for the new date. " +
          "Your watch stays active.",
        cta: "See the event page",
        tone: "warn",
      };

    default:
      return {
        label: "Unknown",
        headline: "We couldn't determine this event's sale status.",
        guidance: "Check the event page on Ticketmaster.",
        cta: "See the event page",
        tone: "muted",
      };
  }
}

/** True when a watch on this event would fire immediately and tell the user nothing new. */
export function firesImmediately(state: SaleState): boolean {
  return state === "on_sale" || state === "presale_open";
}

export type { Watch };
