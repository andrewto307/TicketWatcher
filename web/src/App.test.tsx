import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { setToken, getToken } from "./auth";
import type { Me, Notification, Watch } from "./types";

vi.mock("./api", () => ({
  api: {
    me: vi.fn(),
    listWatches: vi.fn(),
    notifications: vi.fn(),
    resubscribe: vi.fn(),
    deleteAccount: vi.fn(),
    search: vi.fn(),
    createWatch: vi.fn(),
    history: vi.fn(),
    updateWatch: vi.fn(),
    deleteWatch: vi.fn(),
    resendVerification: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    forgotPassword: vi.fn(),
    resetPassword: vi.fn(),
  },
}));

// App mounts its own <BrowserRouter>, which reads window.location — so each
// test sets the URL before rendering, exactly as a real page load would.
async function renderAt(path: string, search = "") {
  window.history.replaceState({}, "", path + search);
  vi.resetModules();
  const { App } = await import("./App");
  return render(<App />);
}

const me = (over: Partial<Me> = {}): Me => ({
  id: 1,
  email: "user@example.test",
  email_verified: true,
  unsubscribed: false,
  ...over,
});

beforeEach(() => {
  localStorage.clear();
  vi.mocked(api.me).mockResolvedValue(me());
  vi.mocked(api.listWatches).mockResolvedValue([] as Watch[]);
  vi.mocked(api.notifications).mockResolvedValue([] as Notification[]);
  vi.mocked(api.search).mockResolvedValue([]);
});

afterEach(() => {
  window.history.replaceState({}, "", "/");
});

describe("App — routes available without an account", () => {
  it("shows the privacy policy without requiring one", async () => {
    await renderAt("/privacy");

    expect(await screen.findByRole("heading", { name: /privacy policy/i })).toBeInTheDocument();
    // A policy you must log in to read is useless.
    expect(api.me).not.toHaveBeenCalled();
  });

  it("shows the reset form when the URL carries a reset token", async () => {
    await renderAt("/reset-password", "?token=abc123");

    expect(await screen.findByPlaceholderText(/^new password/i)).toBeInTheDocument();
  });

  it("sends /reset-password with no token back to the landing page", async () => {
    await renderAt("/reset-password");

    expect(await screen.findByRole("button", { name: /get started/i })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/^new password/i)).not.toBeInTheDocument();
  });
});

describe("App — the logged-out landing page", () => {
  it("explains the product before asking anyone to sign up", async () => {
    await renderAt("/");

    expect(await screen.findByRole("heading", { name: /go on sale\?/i })).toBeInTheDocument();
    // One screen per claim, each backed by a screenshot.
    expect(screen.getByRole("heading", { name: /easy to search/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /start watching/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /the moment it goes on sale/i })).toBeInTheDocument();
    // No account needed to read any of this, so nothing is fetched.
    expect(api.me).not.toHaveBeenCalled();
  });

  it("keeps the login form behind a button rather than showing it immediately", async () => {
    await renderAt("/");

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const header = screen.getByRole("banner");
    await userEvent.click(within(header).getByRole("button", { name: "Log in" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(screen.getByPlaceholderText("email")).toBeInTheDocument();
  });

  it("closes the login modal on Escape", async () => {
    await renderAt("/");

    const header = screen.getByRole("banner");
    await userEvent.click(within(header).getByRole("button", { name: "Log in" }));
    await screen.findByRole("dialog");
    await userEvent.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("redirects the app routes to the landing page when signed out", async () => {
    for (const path of ["/watches", "/saved", "/settings"]) {
      const { unmount } = await renderAt(path);
      // These are real bookmarkable URLs, so a signed-out visitor must land
      // somewhere sensible rather than on an empty shell.
      expect(await screen.findByRole("button", { name: /get started/i })).toBeInTheDocument();
      expect(api.listWatches).not.toHaveBeenCalled();
      unmount();
    }
  });
});

describe("App — the logged-in shell", () => {
  beforeEach(() => setToken("jwt"));

  it("opens on the search page", async () => {
    await renderAt("/");

    expect(await screen.findByPlaceholderText(/search artists/i)).toBeInTheDocument();
    await waitFor(() => expect(api.listWatches).toHaveBeenCalled());
  });

  it("routes to the watches page", async () => {
    await renderAt("/watches");

    expect(await screen.findByRole("heading", { name: /my watches/i })).toBeInTheDocument();
  });

  it("routes to the saved page", async () => {
    await renderAt("/saved");

    expect(await screen.findByRole("heading", { name: /^saved$/i })).toBeInTheDocument();
  });

  it("routes to settings and shows the signed-in address", async () => {
    await renderAt("/settings");

    const main = screen.getByRole("main");
    expect(await within(main).findByText("user@example.test")).toBeInTheDocument();
    expect(within(main).getByRole("button", { name: /delete account/i })).toBeInTheDocument();
  });

  it("sends an unknown path back to search", async () => {
    await renderAt("/nope");

    expect(await screen.findByPlaceholderText(/search artists/i)).toBeInTheDocument();
  });

  it("offers the wordmark as the way home from any page", async () => {
    await renderAt("/watches");

    const home = await screen.findByRole("link", { name: /ticketwatcher home/i });
    expect(home).toHaveAttribute("href", "/");
  });

  it("reaches every destination from the profile menu", async () => {
    await renderAt("/");
    await waitFor(() => expect(api.me).toHaveBeenCalled());

    await userEvent.click(screen.getByRole("button", { name: /user@example.test/i }));

    const menu = await screen.findByRole("menu");
    for (const name of [/my watches/i, /saved/i, /settings/i, /log out/i]) {
      expect(within(menu).getByText(name)).toBeInTheDocument();
    }
  });
});

describe("App — account state", () => {
  beforeEach(() => setToken("jwt"));

  it("nags an unverified user on every page, since alerts are being withheld", async () => {
    vi.mocked(api.me).mockResolvedValue(me({ email_verified: false }));

    for (const path of ["/", "/watches", "/settings"]) {
      const { unmount } = await renderAt(path);
      expect(await screen.findByText(/confirm your email/i)).toBeInTheDocument();
      unmount();
    }
  });

  it("says nothing about verification once verified", async () => {
    await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    expect(screen.queryByText(/confirm your email/i)).not.toBeInTheDocument();
  });

  it("confirms a successful verification and can be dismissed", async () => {
    const { container } = await renderAt("/", "?verified=1");

    expect(await screen.findByText(/email is confirmed/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /dismiss/i }));
    await waitFor(() => expect(container.textContent).not.toMatch(/email is confirmed/i));
  });

  it("explains a bad verification link and points at the resend control", async () => {
    vi.mocked(api.me).mockResolvedValue(me({ email_verified: false }));
    await renderAt("/", "?verify_error=1");

    expect(await screen.findByText(/invalid or has expired/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /resend email/i })).toBeInTheDocument();
  });

  it("scrubs the one-shot flag from the address bar so a refresh doesn't replay it", async () => {
    await renderAt("/", "?verified=1");

    await waitFor(() => expect(window.location.search).toBe(""));
  });

  it("surfaces a backend failure instead of rendering an empty shell", async () => {
    vi.mocked(api.listWatches).mockRejectedValue(new Error("boom"));
    await renderAt("/");

    expect(await screen.findByText(/can't reach the backend/i)).toBeInTheDocument();
  });

  it("clears the token on logout and returns to the landing page", async () => {
    await renderAt("/");
    await waitFor(() => expect(api.me).toHaveBeenCalled());

    await userEvent.click(screen.getByRole("button", { name: /user@example.test/i }));
    await userEvent.click(await screen.findByRole("menuitem", { name: /log out/i }));

    await waitFor(() => expect(getToken()).toBeNull());
    expect(await screen.findByRole("button", { name: /get started/i })).toBeInTheDocument();
  });

  it("links the privacy policy from the footer", async () => {
    await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    expect(screen.getByRole("link", { name: /privacy policy/i })).toHaveAttribute("href", "/privacy");
  });

  it("does not describe the app as a price tracker anywhere", async () => {
    const { container } = await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    // Price watching was removed when Ticketmaster dropped priceRanges (D13);
    // leftover copy would promise something the app cannot do.
    expect(container.textContent?.toLowerCase()).not.toMatch(/price drop|price below|threshold/);
  });
});
