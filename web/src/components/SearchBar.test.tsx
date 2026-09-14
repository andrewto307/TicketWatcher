import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SearchBar } from "./SearchBar";
import type { EventResult } from "../types";
import { api } from "../api";

vi.mock("../api", () => ({
  api: { search: vi.fn(), createWatch: vi.fn() },
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

async function search(term = "fest") {
  await userEvent.type(screen.getByPlaceholderText(/search artists/i), term);
  await userEvent.click(screen.getByRole("button", { name: "Search" }));
}

beforeEach(() => {
  vi.mocked(api.search).mockResolvedValue([result()]);
  vi.mocked(api.createWatch).mockResolvedValue({} as never);
});

describe("SearchBar — the upcoming-onsales filter", () => {
  it("is on by default, because ~85% of unfiltered results are already on sale", () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);

    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("explains what the filter is for, not just what it does", () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);

    expect(screen.getByText(/haven't gone on sale yet/i)).toBeInTheDocument();
    expect(screen.getByText(/worth watching/i)).toBeInTheDocument();
  });

  it("passes the filter through to the API", async () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(api.search).toHaveBeenCalledWith("fest", true);
  });

  it("drops the filter when unchecked", async () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await userEvent.click(screen.getByRole("checkbox"));
    await search();

    expect(api.search).toHaveBeenCalledWith("fest", false);
  });

  it("tells the user how to widen an empty filtered search", async () => {
    vi.mocked(api.search).mockResolvedValue([]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/no events found/i)).toBeInTheDocument();
    expect(screen.getByText(/unchecking the filter/i)).toBeInTheDocument();
  });

  it("does not suggest unchecking a filter that is already off", async () => {
    vi.mocked(api.search).mockResolvedValue([]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await userEvent.click(screen.getByRole("checkbox"));
    await search();

    expect(await screen.findByText(/no events found/i)).toBeInTheDocument();
    expect(screen.queryByText(/unchecking the filter/i)).not.toBeInTheDocument();
  });
});

describe("SearchBar — what each result tells you before you commit", () => {
  it("shows when the official sale opens", async () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/official sale opens/i)).toBeInTheDocument();
  });

  it("warns that an already-onsale event will stay quiet", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ availability: "onsale", public_onsale_start: "2026-06-01T15:00:00Z" }),
    ]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    const note = await screen.findByText(/already on sale/i);
    expect(note).toBeInTheDocument();
    // Must describe the CURRENT behaviour: watching is silent until something changes.
    expect(note.textContent).toMatch(/only hear from us if this changes/i);
  });

  it("says the onsale date is unannounced rather than showing nothing", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ public_onsale_start: null, onsale_tbd: true }),
    ]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/not announced yet/i)).toBeInTheDocument();
    expect(screen.queryByText(/9999/)).not.toBeInTheDocument();
  });

  it("surfaces presale count and earliest window", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ presale_count: 4, earliest_presale_start: "2026-09-15T15:00:00Z" }),
    ]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/4 presales/)).toBeInTheDocument();
  });

  it("falls back gracefully when an event has no date or venue", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ event_date: null, venue: "", public_onsale_start: null, onsale_tbd: false }),
    ]);
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/date TBA/i)).toBeInTheDocument();
    expect(screen.getByText(/not currently on sale/i)).toBeInTheDocument();
    expect(screen.queryByText(/Invalid Date/)).not.toBeInTheDocument();
  });
});

describe("SearchBar — creating a watch", () => {
  it("creates an availability watch and notifies the parent", async () => {
    const onCreated = vi.fn();
    render(<SearchBar onWatchCreated={onCreated} />);
    await search();

    await userEvent.click(await screen.findByRole("button", { name: /Watch/ }));

    expect(api.createWatch).toHaveBeenCalledWith({
      tm_event_id: "TM1",
      condition_type: "becomes_available",
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalled());
  });

  it("confirms visually and prevents adding the same watch twice", async () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    const btn = await screen.findByRole("button", { name: /Watch/ });
    await userEvent.click(btn);

    const done = await screen.findByRole("button", { name: /Watching/ });
    expect(done).toBeDisabled();

    await userEvent.click(done);
    expect(api.createWatch).toHaveBeenCalledTimes(1);
  });

  it("surfaces a creation failure instead of silently doing nothing", async () => {
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    vi.mocked(api.createWatch).mockRejectedValue(new Error("watch limit reached"));
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    await userEvent.click(await screen.findByRole("button", { name: /Watch/ }));

    await waitFor(() => expect(alertSpy).toHaveBeenCalled());
    expect(String(alertSpy.mock.calls[0][0])).toMatch(/watch limit reached/i);
    // The button must return to a usable state so the user can retry.
    expect(await screen.findByRole("button", { name: /Watch/ })).not.toBeDisabled();
  });
});

describe("SearchBar — input handling", () => {
  it("ignores an empty or whitespace-only query", async () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(api.search).not.toHaveBeenCalled();

    await userEvent.type(screen.getByPlaceholderText(/search artists/i), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(api.search).not.toHaveBeenCalled();
  });

  it("shows the error when the search request fails", async () => {
    vi.mocked(api.search).mockRejectedValue(new Error("upstream search failed"));
    render(<SearchBar onWatchCreated={vi.fn()} />);
    await search();

    expect(await screen.findByText(/upstream search failed/i)).toBeInTheDocument();
  });

  it("shows nothing about results before the first search", () => {
    render(<SearchBar onWatchCreated={vi.fn()} />);

    expect(screen.queryByText(/no events found/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  });
});
