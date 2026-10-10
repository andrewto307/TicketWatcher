import type { Page, Route } from "@playwright/test";
import { expect } from "@playwright/test";

/**
 * These are frontend smoke tests. Pages, navigation and the SPA fallback are
 * served by the real Go binary — `page.goto` always hits it, so deep links and
 * refreshes still exercise `spaHandler`. Only the JSON API is stubbed, with a
 * small in-memory backend below.
 *
 * Stubbing the API rather than running the real one is deliberate: creating a
 * watch makes the server fetch the event from Ticketmaster, so a real write
 * path would depend on a live third party, spend real API budget, and flake
 * whenever Ticketmaster changed an event. None of that is what this suite is
 * checking.
 */

export interface StubEvent {
  tm_event_id: string;
  name: string;
  venue: string;
  url: string;
  event_date: string | null;
  availability: string;
  public_onsale_start: string | null;
  public_onsale_end: string | null;
  earliest_presale_start: string | null;
  earliest_presale_name: string;
  presale_count: number;
  onsale_tbd: boolean;
}

export function stubEvent(over: Partial<StubEvent> = {}): StubEvent {
  return {
    tm_event_id: "E2E-1",
    name: "E2E Future Fest",
    venue: "Playwright Arena",
    url: "https://ticketmaster.test/e/E2E-1",
    event_date: "2030-12-01T20:00:00Z",
    availability: "offsale",
    public_onsale_start: "2030-06-01T15:00:00Z",
    public_onsale_end: "2030-11-30T00:00:00Z",
    earliest_presale_start: null,
    earliest_presale_name: "",
    presale_count: 0,
    onsale_tbd: false,
    ...over,
  };
}

interface Options {
  /** What a search returns. */
  events?: StubEvent[];
  /** Start out verified; a fresh account is unverified in reality. */
  emailVerified?: boolean;
  email?: string;
}

interface StoredWatch extends StubEvent {
  id: number;
  status: "active" | "paused";
}

/** Installs the fake API and returns a handle to its state. */
export async function mockApi(page: Page, opts: Options = {}) {
  const events = opts.events ?? [stubEvent()];
  const state = {
    email: opts.email ?? "e2e@example.test",
    emailVerified: opts.emailVerified ?? false,
    watches: [] as StoredWatch[],
    nextId: 1,
  };

  const watchView = (w: StoredWatch) => ({
    id: w.id,
    tm_event_id: w.tm_event_id,
    event_name: w.name,
    venue: w.venue,
    event_url: w.url,
    event_date: w.event_date,
    condition_type: "becomes_available",
    status: w.status,
    availability: w.availability,
    poll_interval_s: 300,
    created_at: new Date().toISOString(),
    last_polled_at: new Date().toISOString(),
    public_onsale_start: w.public_onsale_start,
    public_onsale_end: w.public_onsale_end,
    earliest_presale: w.earliest_presale_start,
    earliest_presale_name: w.earliest_presale_name || null,
    presale_count: w.presale_count,
    onsale_tbd: w.onsale_tbd,
    sale_state: "onsale_scheduled",
  });

  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

  await page.route("**/api/**", async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname;
    const method = req.method();

    if (path === "/api/auth/register" || path === "/api/auth/login") {
      const body = req.postDataJSON() as { email?: string };
      if (body?.email) state.email = body.email;
      return json(route, { token: "e2e-token" });
    }
    if (path === "/api/auth/verify/resend") return route.fulfill({ status: 204, body: "" });
    if (path === "/api/auth/forgot" || path === "/api/auth/reset")
      return route.fulfill({ status: 204, body: "" });

    if (path === "/api/me") {
      return json(route, {
        id: 1,
        email: state.email,
        email_verified: state.emailVerified,
        unsubscribed: false,
      });
    }

    if (path === "/api/search") return json(route, events);
    if (path === "/api/notifications") return json(route, []);

    if (path === "/api/watches" && method === "GET")
      return json(route, state.watches.map(watchView));

    if (path === "/api/watches" && method === "POST") {
      const body = req.postDataJSON() as { tm_event_id: string };
      const ev = events.find((e) => e.tm_event_id === body.tm_event_id) ?? stubEvent(body);
      const w: StoredWatch = { ...ev, id: state.nextId++, status: "active" };
      state.watches.push(w);
      return json(route, watchView(w));
    }

    const match = /^\/api\/watches\/(\d+)(\/history)?$/.exec(path);
    if (match) {
      const id = Number(match[1]);
      if (match[2]) return json(route, []);
      const w = state.watches.find((x) => x.id === id);
      if (!w) return json(route, { error: "not found" }, 404);
      if (method === "PATCH") {
        const body = req.postDataJSON() as { status?: "active" | "paused" };
        if (body.status) w.status = body.status;
        return json(route, watchView(w));
      }
      if (method === "DELETE") {
        state.watches = state.watches.filter((x) => x.id !== id);
        return route.fulfill({ status: 204, body: "" });
      }
    }

    if (path === "/api/account" && method === "DELETE")
      return route.fulfill({ status: 204, body: "" });
    if (path === "/api/account/resubscribe") return route.fulfill({ status: 204, body: "" });

    return json(route, { error: `unhandled ${method} ${path}` }, 500);
  });

  return state;
}

/** Signs in through the UI and lands on the search page. */
export async function signIn(page: Page): Promise<void> {
  await page.goto("/");
  await page.getByRole("banner").getByRole("button", { name: "Log in" }).click();
  await page.getByPlaceholder("email").fill("e2e@example.test");
  await page.getByPlaceholder(/password/i).fill("e2e-password-123");
  await page.getByRole("dialog").getByRole("button", { name: "Log in" }).click();
  await expect(page.getByPlaceholder(/search artists/i)).toBeVisible();
}

export async function search(page: Page, term = "fest"): Promise<void> {
  await page.getByPlaceholder(/search artists/i).fill(term);
  await page.getByRole("button", { name: "Search" }).click();
}

/** Console errors seen on the page — asserted empty on every route. */
export function collectConsoleErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  return errors;
}
