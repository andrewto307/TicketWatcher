import { render, screen, waitFor } from "@testing-library/react";
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
  },
}));

// App reads window.location at module scope to decide which view to show (there
// is no router), so each test must set the URL *before* importing it.
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
  vi.mocked(api.me).mockResolvedValue(me());
  vi.mocked(api.listWatches).mockResolvedValue([] as Watch[]);
  vi.mocked(api.notifications).mockResolvedValue([] as Notification[]);
  vi.mocked(api.search).mockResolvedValue([]);
});

afterEach(() => {
  window.history.replaceState({}, "", "/");
});

describe("App — URL-driven views", () => {
  it("shows the privacy policy without requiring an account", async () => {
    await renderAt("/privacy");

    expect(await screen.findByRole("heading", { name: /privacy policy/i })).toBeInTheDocument();
    // A policy you must log in to read is useless — no login form here.
    expect(screen.queryByPlaceholderText("email")).not.toBeInTheDocument();
    expect(api.me).not.toHaveBeenCalled();
  });

  it("shows the reset form when the URL carries a reset token", async () => {
    await renderAt("/reset-password", "?token=abc123");

    expect(await screen.findByPlaceholderText(/^new password/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Log in" })).not.toBeInTheDocument();
  });

  it("falls back to login on /reset-password with no token", async () => {
    await renderAt("/reset-password");

    expect(await screen.findByRole("button", { name: "Log in" })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/^new password/i)).not.toBeInTheDocument();
  });

  it("shows login when there is no stored token", async () => {
    await renderAt("/");

    expect(await screen.findByRole("button", { name: "Log in" })).toBeInTheDocument();
    expect(api.listWatches).not.toHaveBeenCalled();
  });

  it("shows the dashboard when a token is present", async () => {
    setToken("jwt");
    await renderAt("/");

    expect(await screen.findByRole("heading", { name: /my watches/i })).toBeInTheDocument();
    await waitFor(() => expect(api.listWatches).toHaveBeenCalled());
  });
});

describe("App — verification feedback from the emailed link", () => {
  it("confirms success and can be dismissed", async () => {
    setToken("jwt");
    const { container } = await renderAt("/", "?verified=1");

    expect(await screen.findByText(/email is confirmed/i)).toBeInTheDocument();

    const dismiss = screen.getByRole("button", { name: /dismiss/i });
    dismiss.click();
    await waitFor(() => expect(container.textContent).not.toMatch(/email is confirmed/i));
  });

  it("explains a bad link and points at the resend control", async () => {
    setToken("jwt");
    vi.mocked(api.me).mockResolvedValue(me({ email_verified: false }));
    await renderAt("/", "?verify_error=1");

    expect(await screen.findByText(/invalid or has expired/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /resend email/i })).toBeInTheDocument();
  });

  it("scrubs the one-shot flag from the address bar so a refresh doesn't replay it", async () => {
    setToken("jwt");
    await renderAt("/", "?verified=1");

    await waitFor(() => expect(window.location.search).toBe(""));
  });
});

describe("App — account state drives what is shown", () => {
  it("nags an unverified user, since their alerts are being withheld", async () => {
    setToken("jwt");
    vi.mocked(api.me).mockResolvedValue(me({ email_verified: false }));
    await renderAt("/");

    expect(await screen.findByText(/confirm your email/i)).toBeInTheDocument();
  });

  it("says nothing about verification once verified", async () => {
    setToken("jwt");
    await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    expect(screen.queryByText(/confirm your email/i)).not.toBeInTheDocument();
  });

  it("shows the account section with the signed-in address", async () => {
    setToken("jwt");
    await renderAt("/");

    expect(await screen.findByText("user@example.test")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /delete account/i })).toBeInTheDocument();
  });

  it("links the privacy policy from the footer", async () => {
    setToken("jwt");
    await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    expect(screen.getByRole("link", { name: /privacy policy/i })).toHaveAttribute("href", "/privacy");
  });
});

describe("App — failure and logout", () => {
  it("surfaces a backend failure instead of rendering an empty dashboard", async () => {
    setToken("jwt");
    vi.mocked(api.listWatches).mockRejectedValue(new Error("boom"));
    await renderAt("/");

    expect(await screen.findByText(/can't reach the backend/i)).toBeInTheDocument();
  });

  it("clears the token on logout and returns to login", async () => {
    setToken("jwt");
    await renderAt("/");

    const out = await screen.findByRole("button", { name: /log out/i });
    out.click();

    await waitFor(() => expect(getToken()).toBeNull());
    expect(await screen.findByRole("button", { name: "Log in" })).toBeInTheDocument();
  });

  it("does not describe the app as a price tracker anywhere", async () => {
    setToken("jwt");
    const { container } = await renderAt("/");

    await waitFor(() => expect(api.me).toHaveBeenCalled());
    // Price watching was removed when Ticketmaster dropped priceRanges (D13);
    // leftover copy would promise something the app cannot do.
    expect(container.textContent?.toLowerCase()).not.toMatch(/price drop|price below|threshold/);
  });
});
