import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { getToken, setToken } from "./auth";

// The API layer decides three things that are easy to get subtly wrong: what gets
// sent, how a dead session is handled, and whether the server's error message
// reaches the user or is replaced by something generic.

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

let reloadSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  localStorage.clear();
  // jsdom's location.reload is not implemented; replace it so the 401 path can
  // be observed rather than throwing.
  reloadSpy = vi.fn();
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { ...window.location, reload: reloadSpy },
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubFetch(make: () => Response) {
  // A fresh Response per call: bodies are single-read, so reusing one object
  // fails the second time with "Body has already been read".
  const spy = vi.fn().mockImplementation(async () => make());
  vi.stubGlobal("fetch", spy);
  return spy;
}

// The Fetch spec forbids a body on 204, so it cannot be built with `new Response("")`.
const noContent = () => new Response(null, { status: 204 });

describe("api — authentication header", () => {
  it("attaches the bearer token to protected calls", async () => {
    setToken("jwt-abc");
    const spy = stubFetch(() => json([]));

    await api.listWatches();

    const [, init] = spy.mock.calls[0];
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer jwt-abc");
  });

  it("omits the header entirely when there is no token", async () => {
    const spy = stubFetch(() => json([]));

    await api.listWatches();

    const [, init] = spy.mock.calls[0];
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
  });

  it("does not send a token on login or register", async () => {
    setToken("jwt-abc");
    const spy = stubFetch(() => json({ token: "new" }));

    await api.login("u@example.test", "pw");

    const [, init] = spy.mock.calls[0];
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
  });
});

describe("api — a dead session", () => {
  it("clears the stored token and reloads on 401", async () => {
    setToken("stale-jwt");
    stubFetch(() => new Response(null, { status: 401 }));

    await expect(api.listWatches()).rejects.toThrow(/session expired/i);

    // Without clearing, the app would loop on a token the server rejects.
    expect(getToken()).toBeNull();
    expect(reloadSpy).toHaveBeenCalled();
  });

  it("treats a 401 from /api/me the same way — it means the account is gone", async () => {
    setToken("stale-jwt");
    stubFetch(() => new Response(null, { status: 401 }));

    await expect(api.me()).rejects.toThrow(/session expired/i);
    expect(getToken()).toBeNull();
  });
});

describe("api — surfacing server errors", () => {
  it("uses the server's message on auth failures rather than a status code", async () => {
    stubFetch(() => json({ error: "email already registered" }, 409));

    await expect(api.register("u@example.test", "password123")).rejects.toThrow(
      /email already registered/i,
    );
  });

  it("falls back to the status when the body has no message", async () => {
    stubFetch(() => new Response("not json", { status: 500 }));

    await expect(api.login("u@example.test", "pw")).rejects.toThrow(/500/);
  });

  it("surfaces the message from 204-style endpoints too", async () => {
    stubFetch(() => json({ error: "this link is invalid or has expired" }, 400));

    await expect(api.resetPassword("stale", "password123")).rejects.toThrow(/invalid or has expired/i);
  });

  it("resolves quietly when a 204 endpoint succeeds", async () => {
    stubFetch(noContent);

    await expect(api.forgotPassword("u@example.test")).resolves.toBeUndefined();
  });

  it("reports the watch-limit refusal so the user learns the cap", async () => {
    setToken("jwt");
    stubFetch(() => json({ error: "watch limit reached: you can track up to 50 events at once" }, 403));

    await expect(
      api.createWatch({ tm_event_id: "TM1", condition_type: "becomes_available" }),
    ).rejects.toThrow(/up to 50 events/i);
  });
});

describe("api — request shape", () => {
  it("url-encodes the search query", async () => {
    setToken("jwt");
    const spy = stubFetch(() => json([]));

    await api.search("rock & roll", false);

    expect(spy.mock.calls[0][0]).toContain("q=rock%20%26%20roll");
  });

  it("adds the upcoming filter only when asked", async () => {
    setToken("jwt");
    const spy = stubFetch(() => json([]));

    await api.search("fest", true);
    expect(spy.mock.calls[0][0]).toContain("upcoming=1");

    await api.search("fest", false);
    expect(spy.mock.calls[1][0]).not.toContain("upcoming");
  });

  it("defaults the upcoming filter to off when the caller omits it", async () => {
    setToken("jwt");
    const spy = stubFetch(() => json([]));

    await api.search("fest");

    expect(spy.mock.calls[0][0]).not.toContain("upcoming");
  });

  it("sends the watch payload as JSON", async () => {
    setToken("jwt");
    const spy = stubFetch(() => json({}));

    await api.createWatch({ tm_event_id: "TM1", condition_type: "becomes_available" });

    const [url, init] = spy.mock.calls[0];
    expect(url).toContain("/api/watches");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      tm_event_id: "TM1",
      condition_type: "becomes_available",
    });
  });

  it("uses DELETE for account removal", async () => {
    setToken("jwt");
    const spy = stubFetch(noContent);

    await api.deleteAccount();

    const [url, init] = spy.mock.calls[0];
    expect(url).toContain("/api/account");
    expect(init.method).toBe("DELETE");
  });
});
