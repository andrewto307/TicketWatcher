import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SearchResultCard } from "./SearchResultCard";
import { ToastProvider } from "../ui/Toast";
import type { EventResult } from "../types";
import { api } from "../api";

vi.mock("../api", () => ({
  api: { createWatch: vi.fn(), updateWatch: vi.fn() },
}));

function result(over: Partial<EventResult> = {}): EventResult {
  return {
    tm_event_id: "TM1",
    name: "Future Fest",
    venue: "Test Arena",
    url: "https://ticketmaster.test/e/TM1",
    event_date: "2026-12-01T20:00:00Z",
    availability: "offsale",
    public_onsale_start: "2026-09-17T15:00:00Z",
    public_onsale_end: "2026-11-30T00:00:00Z",
    earliest_presale_start: null,
    earliest_presale_name: "",
    presale_count: 0,
    onsale_tbd: false,
    ...over,
  };
}

const show = (ev: EventResult, tracked?: "watching" | "saved", onChanged = vi.fn()) =>
  render(
    <ToastProvider>
      <SearchResultCard ev={ev} tracked={tracked} onChanged={onChanged} />
    </ToastProvider>,
  );

beforeEach(() => {
  vi.mocked(api.createWatch).mockResolvedValue({ id: 7 } as never);
  vi.mocked(api.updateWatch).mockResolvedValue({} as never);
});

describe("SearchResultCard — what it tells you before you commit", () => {
  it("shows when the official sale opens", () => {
    show(result());

    expect(screen.getByText(/official sale opens/i)).toBeInTheDocument();
  });

  it("warns that an already-onsale event will stay quiet", () => {
    show(result({ availability: "onsale" }));

    // Watching this sends nothing until something actually changes, so the
    // card has to say so rather than implying an alert is coming.
    expect(screen.getByText(/already on sale/i)).toBeInTheDocument();
    expect(screen.getByText(/only hear from us if this changes/i)).toBeInTheDocument();
  });

  it("says the onsale date is unannounced rather than showing nothing", () => {
    show(result({ public_onsale_start: null, onsale_tbd: true }));

    expect(screen.getByText(/not announced yet/i)).toBeInTheDocument();
  });

  it("surfaces presale count and earliest window", () => {
    show(result({ presale_count: 3, earliest_presale_start: "2026-09-10T15:00:00Z" }));

    expect(screen.getByText(/3 presales/i)).toBeInTheDocument();
    expect(screen.getByText(/earliest/i)).toBeInTheDocument();
  });

  it("falls back gracefully when an event has no date or venue", () => {
    show(result({ venue: "", event_date: null }));

    expect(screen.getByText(/date TBA/i)).toBeInTheDocument();
  });

  it("links out to Ticketmaster", () => {
    show(result());

    expect(screen.getByRole("link", { name: /ticketmaster/i })).toHaveAttribute(
      "href",
      "https://ticketmaster.test/e/TM1",
    );
  });
});

describe("SearchResultCard — adding", () => {
  it("Watch creates an active availability watch", async () => {
    const onChanged = vi.fn();
    show(result(), undefined, onChanged);

    await userEvent.click(screen.getByRole("button", { name: /^Watch$/ }));

    await waitFor(() =>
      expect(api.createWatch).toHaveBeenCalledWith({
        tm_event_id: "TM1",
        condition_type: "becomes_available",
      }),
    );
    // Watch must not pause: that is what Save is for.
    expect(api.updateWatch).not.toHaveBeenCalled();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("Save creates the watch and immediately pauses it", async () => {
    show(result());

    await userEvent.click(screen.getByRole("button", { name: /^Save$/ }));

    // POST always creates an active watch, so Saved is reached with a PATCH.
    await waitFor(() => expect(api.createWatch).toHaveBeenCalled());
    await waitFor(() => expect(api.updateWatch).toHaveBeenCalledWith(7, { status: "paused" }));
  });

  it("confirms the action, because the card gives no other feedback", async () => {
    show(result());

    await userEvent.click(screen.getByRole("button", { name: /^Watch$/ }));

    // The watch list is a different page now, so without this the click
    // appears to do nothing at all.
    expect(await screen.findByRole("status")).toHaveTextContent(/watching "future fest"/i);
  });

  it("surfaces a failure instead of silently doing nothing", async () => {
    vi.mocked(api.createWatch).mockRejectedValue(new Error("watch limit reached"));
    show(result());

    await userEvent.click(screen.getByRole("button", { name: /^Watch$/ }));

    expect(await screen.findByRole("status")).toHaveTextContent(/watch limit reached/i);
  });

  it("offers no add buttons for an event already tracked", () => {
    show(result(), "watching");

    expect(screen.getByText("Watching")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Watch$/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Save$/ })).not.toBeInTheDocument();
  });
});
