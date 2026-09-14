import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WatchCard } from "./WatchCard";
import type { Watch } from "../types";
import { api } from "../api";

vi.mock("../api", () => ({
  api: {
    history: vi.fn(),
    updateWatch: vi.fn(),
    deleteWatch: vi.fn(),
  },
}));

function watch(over: Partial<Watch> = {}): Watch {
  return {
    id: 1,
    tm_event_id: "TM1",
    event_name: "Test Show",
    venue: "Test Arena",
    event_url: "https://ticketmaster.test/e/TM1",
    event_date: "2026-12-01T20:00:00Z",
    condition_type: "becomes_available",
    status: "active",
    availability: "offsale",
    poll_interval_s: 300,
    created_at: "2026-09-01T00:00:00Z",
    last_polled_at: "2026-09-13T12:00:00Z",
    public_onsale_start: "2026-09-17T15:00:00Z",
    public_onsale_end: "2026-11-30T00:00:00Z",
    earliest_presale: null,
    earliest_presale_name: null,
    presale_count: 0,
    onsale_tbd: false,
    sale_state: "onsale_scheduled",
    ...over,
  };
}

beforeEach(() => {
  vi.mocked(api.history).mockResolvedValue([]);
  vi.mocked(api.updateWatch).mockResolvedValue(watch());
  vi.mocked(api.deleteWatch).mockResolvedValue(undefined);
});

describe("WatchCard — the sale state a user actually reads", () => {
  it("shows the scheduled onsale time, not a bare status", () => {
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    expect(screen.getByText(/the official sale opens/i)).toBeInTheDocument();
    expect(screen.getByText("Opens later")).toBeInTheDocument();
  });

  it("links to Ticketmaster with a call to action matching the state", () => {
    render(<WatchCard watch={watch({ sale_state: "sale_closed" })} onChanged={vi.fn()} />);

    const link = screen.getByRole("link", { name: /check for resale/i });
    expect(link).toHaveAttribute("href", "https://ticketmaster.test/e/TM1");
    // Opening Ticketmaster must not navigate away from the dashboard.
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", expect.stringContaining("noopener"));
  });

  it("tells a user with a closed sale to go check resale", () => {
    render(<WatchCard watch={watch({ sale_state: "sale_closed" })} onChanged={vi.fn()} />);

    expect(screen.getByText(/official ticketmaster sale has closed/i)).toBeInTheDocument();
    expect(screen.getByText(/resale tickets may still be available/i)).toBeInTheDocument();
  });

  it("says the date is unannounced rather than showing a placeholder date", () => {
    render(
      <WatchCard
        watch={watch({ sale_state: "onsale_tbd", public_onsale_start: null, onsale_tbd: true })}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByText(/hasn't announced/i)).toBeInTheDocument();
    // The 9999 sentinel must never reach the screen.
    expect(screen.queryByText(/9999/)).not.toBeInTheDocument();
  });

  it("shows 'checking' before the first poll instead of an empty state", () => {
    render(<WatchCard watch={watch({ sale_state: "checking", last_polled_at: null })} onChanged={vi.fn()} />);

    expect(screen.getByText("Checking…")).toBeInTheDocument();
    expect(screen.getByText(/haven't checked this event yet/i)).toBeInTheDocument();
  });

  it("surfaces presale information, the closest thing we have to a resale signal", () => {
    render(
      <WatchCard
        watch={watch({
          presale_count: 3,
          earliest_presale: "2026-09-15T15:00:00Z",
          earliest_presale_name: "Artist Presale",
        })}
        onChanged={vi.fn()}
      />,
    );

    // "· earliest" only appears on the detail line, so this is unambiguous.
    expect(screen.getByText(/3 presales · earliest/)).toBeInTheDocument();
    expect(screen.getByText(/Artist Presale/)).toBeInTheDocument();
  });

  it("uses singular grammar for a single presale", () => {
    render(
      <WatchCard watch={watch({ presale_count: 1, earliest_presale: "2026-09-15T15:00:00Z" })} onChanged={vi.fn()} />,
    );

    expect(screen.getByText(/1 presale · earliest/)).toBeInTheDocument();
  });

  it("hides the presale line entirely when there are none", () => {
    render(<WatchCard watch={watch({ presale_count: 0 })} onChanged={vi.fn()} />);

    expect(screen.queryByText(/presale/i)).not.toBeInTheDocument();
  });
});

describe("WatchCard — actions", () => {
  it("pauses an active watch and refreshes", async () => {
    const onChanged = vi.fn();
    render(<WatchCard watch={watch()} onChanged={onChanged} />);

    await userEvent.click(screen.getByRole("button", { name: "Pause" }));

    expect(api.updateWatch).toHaveBeenCalledWith(1, { status: "paused" });
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("resumes a paused watch", async () => {
    render(<WatchCard watch={watch({ status: "paused" })} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Resume" }));

    expect(api.updateWatch).toHaveBeenCalledWith(1, { status: "active" });
  });

  it("asks for confirmation before deleting, and does nothing if declined", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Delete" }));

    expect(window.confirm).toHaveBeenCalled();
    expect(api.deleteWatch).not.toHaveBeenCalled();
  });

  it("deletes when confirmed", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const onChanged = vi.fn();
    render(<WatchCard watch={watch()} onChanged={onChanged} />);

    await userEvent.click(screen.getByRole("button", { name: "Delete" }));

    expect(api.deleteWatch).toHaveBeenCalledWith(1);
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe("WatchCard — status history", () => {
  it("loads history lazily, only when expanded", async () => {
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    expect(api.history).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Status history" }));

    await waitFor(() => expect(api.history).toHaveBeenCalledWith(1));
  });

  it("does not refetch history when toggled twice", async () => {
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Status history" }));
    await waitFor(() => expect(api.history).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "Hide history" }));
    await userEvent.click(screen.getByRole("button", { name: "Status history" }));

    expect(api.history).toHaveBeenCalledTimes(1);
  });

  it("shows an explanation rather than a blank panel when there is no history", async () => {
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Status history" }));

    expect(await screen.findByText(/no changes recorded yet/i)).toBeInTheDocument();
  });

  it("renders newest-first with the current state marked", async () => {
    vi.mocked(api.history).mockResolvedValue([
      { availability: "offsale", checked_at: "2026-09-10T10:00:00Z" },
      { availability: "onsale", checked_at: "2026-09-12T10:00:00Z" },
    ]);
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Status history" }));

    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(2);
    // Newest first: the onsale entry, labelled as current.
    expect(items[0]).toHaveTextContent("onsale");
    expect(items[0]).toHaveTextContent("current");
    expect(items[1]).toHaveTextContent("offsale");
  });

  it("survives a failing history request without breaking the card", async () => {
    vi.mocked(api.history).mockRejectedValue(new Error("network"));
    render(<WatchCard watch={watch()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Status history" }));

    expect(await screen.findByText(/no changes recorded yet/i)).toBeInTheDocument();
    expect(screen.getByText("Test Show")).toBeInTheDocument();
  });
});
