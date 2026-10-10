import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { SearchPage } from "./SearchPage";
import { AppDataProvider } from "../AppData";
import { ToastProvider } from "../ui/Toast";
import type { EventResult } from "../types";
import { api } from "../api";

vi.mock("../api", () => ({
  api: {
    search: vi.fn(),
    createWatch: vi.fn(),
    updateWatch: vi.fn(),
    me: vi.fn(),
    listWatches: vi.fn(),
    notifications: vi.fn(),
  },
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

function renderPage() {
  return render(
    <ToastProvider>
      <MemoryRouter>
        <AppDataProvider>
          <SearchPage />
        </AppDataProvider>
      </MemoryRouter>
    </ToastProvider>,
  );
}

async function search(term = "fest") {
  await userEvent.type(screen.getByPlaceholderText(/search artists/i), term);
  await userEvent.click(screen.getByRole("button", { name: "Search" }));
}

beforeEach(() => {
  vi.mocked(api.search).mockResolvedValue([result()]);
  vi.mocked(api.createWatch).mockResolvedValue({ id: 7 } as never);
  vi.mocked(api.updateWatch).mockResolvedValue({} as never);
  vi.mocked(api.me).mockResolvedValue({
    id: 1,
    email: "u@example.test",
    email_verified: true,
    unsubscribed: false,
  });
  vi.mocked(api.listWatches).mockResolvedValue([]);
  vi.mocked(api.notifications).mockResolvedValue([]);
});

describe("SearchPage — the landing state", () => {
  it("shows only the search bar before anything is searched", async () => {
    renderPage();

    expect(screen.getByPlaceholderText(/search artists/i)).toBeInTheDocument();
    // No results region, and nothing pretending to be one.
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
    expect(screen.queryByText(/no events found/i)).not.toBeInTheDocument();
  });

  it("keeps the bar centred while the user is still typing", async () => {
    renderPage();

    await userEvent.type(screen.getByPlaceholderText(/search artists/i), "fes");

    // The move is driven by submitting, not by typing — otherwise the layout
    // jumps under the user mid-query.
    expect(api.search).not.toHaveBeenCalled();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});

describe("SearchPage — the upcoming-onsales filter", () => {
  it("is on by default, because most unfiltered results are already on sale", () => {
    renderPage();

    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("passes the filter through to the API", async () => {
    renderPage();

    await search();

    await waitFor(() => expect(api.search).toHaveBeenCalledWith("fest", true));
  });

  it("drops the filter when unchecked", async () => {
    renderPage();

    await userEvent.click(screen.getByRole("checkbox"));
    await search();

    await waitFor(() => expect(api.search).toHaveBeenCalledWith("fest", false));
  });

  it("tells the user how to widen an empty filtered search", async () => {
    vi.mocked(api.search).mockResolvedValue([]);
    renderPage();

    await search();

    expect(await screen.findByText(/no events found/i)).toBeInTheDocument();
    expect(screen.getByText(/unchecking the filter/i)).toBeInTheDocument();
  });

  it("does not suggest unchecking a filter that is already off", async () => {
    vi.mocked(api.search).mockResolvedValue([]);
    renderPage();

    await userEvent.click(screen.getByRole("checkbox"));
    await search();

    expect(await screen.findByText(/no events found/i)).toBeInTheDocument();
    expect(screen.queryByText(/unchecking the filter/i)).not.toBeInTheDocument();
  });
});

describe("SearchPage — an empty result still moves the bar", () => {
  it("shows the compact layout and a message, not a centred bar", async () => {
    vi.mocked(api.search).mockResolvedValue([]);
    renderPage();

    await search();

    // If the transition keyed off having results, a zero-result search would
    // leave the bar centred and the Search button would look broken.
    expect(await screen.findByText(/no events found/i)).toBeInTheDocument();
    expect(screen.getByRole("search")).toBeInTheDocument();
  });
});

describe("SearchPage — grouping repeated dates", () => {
  it("collapses one attraction's many dates into a single row", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ tm_event_id: "A1", name: "Residency Show", event_date: "2027-04-10T00:00:00Z" }),
      result({ tm_event_id: "A2", name: "Residency Show", event_date: "2027-04-11T00:00:00Z" }),
      result({ tm_event_id: "A3", name: "Residency Show", event_date: "2027-04-12T00:00:00Z" }),
    ]);
    renderPage();

    await search();

    // Fourteen separate rows for one residency reads as broken, even though
    // the data is correct.
    expect(await screen.findByText("3 dates")).toBeInTheDocument();
    expect(screen.getAllByText("Residency Show")).toHaveLength(1);
    expect(screen.queryByRole("button", { name: /^Watch$/ })).not.toBeInTheDocument();
  });

  it("expands the group to reveal each date", async () => {
    vi.mocked(api.search).mockResolvedValue([
      result({ tm_event_id: "A1", name: "Residency Show" }),
      result({ tm_event_id: "A2", name: "Residency Show" }),
    ]);
    renderPage();

    await search();
    await userEvent.click(await screen.findByRole("button", { name: /Residency Show/ }));

    expect(screen.getAllByRole("button", { name: /^Watch$/ })).toHaveLength(2);
  });

  it("leaves a single-date event ungrouped", async () => {
    renderPage();

    await search();

    expect(await screen.findByText("Future Fest")).toBeInTheDocument();
    expect(screen.queryByText(/\d+ dates/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Watch$/ })).toBeInTheDocument();
  });
});

describe("SearchPage — results the user already tracks", () => {
  it("says Watching instead of offering to add it again", async () => {
    vi.mocked(api.listWatches).mockResolvedValue([
      { id: 1, tm_event_id: "TM1", status: "active" } as never,
    ]);
    renderPage();

    await search();

    expect(await screen.findByText("Watching")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Watch$/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Save$/ })).not.toBeInTheDocument();
  });

  it("distinguishes a saved event from a watched one", async () => {
    vi.mocked(api.listWatches).mockResolvedValue([
      { id: 1, tm_event_id: "TM1", status: "paused" } as never,
    ]);
    renderPage();

    await search();

    expect(await screen.findByText("Saved")).toBeInTheDocument();
  });
});
