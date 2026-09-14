import { describe, expect, it } from "vitest";
import { formatSaleTime, saleCopy, firesImmediately } from "./saleState";
import type { SaleState } from "./types";

// This module holds every user-facing sentence about an event's sale state, and
// it has more branching than anything else in the frontend. It is also where the
// honesty requirements live: never claim tickets exist (we can't see resale), and
// never leave a state without telling the user what to do about it.

type CopyInput = Parameters<typeof saleCopy>[0];

function watch(over: Partial<CopyInput> = {}): CopyInput {
  return {
    sale_state: "onsale_scheduled",
    public_onsale_start: null,
    earliest_presale: null,
    earliest_presale_name: null,
    presale_count: 0,
    ...over,
  };
}

const allStates: SaleState[] = [
  "checking",
  "cancelled",
  "rescheduled",
  "presale_open",
  "on_sale",
  "sale_closed",
  "onsale_scheduled",
  "onsale_tbd",
  "unknown",
];

describe("saleCopy — contract every state must satisfy", () => {
  it.each(allStates)("%s produces a usable label, headline and tone", (state) => {
    const c = saleCopy(watch({ sale_state: state, public_onsale_start: "2026-09-17T15:00:00Z" }));

    expect(c.label).toBeTruthy();
    expect(c.headline.length).toBeGreaterThan(10);
    expect(["good", "warn", "muted"]).toContain(c.tone);
    // A headline should read as a sentence, not a fragment.
    expect(c.headline.trim()).toMatch(/[.!]$/);
  });

  it.each(allStates)("%s never claims tickets are in stock", (state) => {
    const c = saleCopy(watch({ sale_state: state, public_onsale_start: "2026-09-17T15:00:00Z" }));
    const text = `${c.label} ${c.headline} ${c.guidance ?? ""}`.toLowerCase();

    // Ticketmaster's status says the sale window is open, not that seats exist —
    // a sold-out show still reports onsale. Claiming otherwise would be a lie.
    for (const forbidden of ["tickets are available", "seats available", "in stock", "guaranteed"]) {
      expect(text).not.toContain(forbidden);
    }
  });

  it.each(allStates)("%s leaves no unresolved placeholder in the text", (state) => {
    const c = saleCopy(watch({ sale_state: state }));
    const text = `${c.headline} ${c.guidance ?? ""}`;

    // With no dates supplied, formatSaleTime returns "" — copy must not end up
    // saying "opens ." or "opens undefined".
    expect(text).not.toContain("undefined");
    expect(text).not.toContain("null");
    expect(text).not.toMatch(/\s\./); // a space immediately before a full stop
  });
});

describe("saleCopy — states that must guide the user somewhere", () => {
  it("sale_closed points at resale, because we cannot see resale ourselves", () => {
    const c = saleCopy(watch({ sale_state: "sale_closed" }));

    expect(c.tone).toBe("warn");
    expect(c.guidance?.toLowerCase()).toContain("resale");
    expect(c.cta?.toLowerCase()).toContain("resale");
  });

  it("presale_open explains that access may be restricted", () => {
    const c = saleCopy(
      watch({
        sale_state: "presale_open",
        earliest_presale_name: "Artist Presale",
        public_onsale_start: "2026-09-20T15:00:00Z",
      }),
    );

    expect(c.headline).toContain("Artist Presale");
    // Without this, users click through, hit a code prompt, and assume it's broken.
    expect(c.guidance?.toLowerCase()).toContain("code");
    expect(c.guidance).toMatch(/public sale opens/i);
  });

  it("presale_open says the public date is unknown rather than leaving a gap", () => {
    const c = saleCopy(
      watch({ sale_state: "presale_open", earliest_presale_name: null, public_onsale_start: null }),
    );

    expect(c.headline).toMatch(/a presale is open/i);
    expect(c.guidance?.toLowerCase()).toContain("isn't announced");
  });

  it("cancelled explains refunds and that watching has stopped", () => {
    const c = saleCopy(watch({ sale_state: "cancelled" }));

    expect(c.tone).toBe("warn");
    expect(c.guidance?.toLowerCase()).toContain("refund");
    expect(c.guidance?.toLowerCase()).toMatch(/stopped watching/);
  });

  it("rescheduled reassures that existing tickets usually stand and the watch continues", () => {
    const c = saleCopy(watch({ sale_state: "rescheduled" }));

    expect(c.tone).toBe("warn");
    expect(c.guidance?.toLowerCase()).toContain("still valid");
    expect(c.guidance?.toLowerCase()).toContain("stays active");
  });

  it("onsale_tbd promises an alert when the date appears", () => {
    const c = saleCopy(watch({ sale_state: "onsale_tbd" }));

    expect(c.headline.toLowerCase()).toContain("hasn't announced");
    expect(c.guidance?.toLowerCase()).toContain("as soon as a date");
  });

  it("checking reassures rather than looking broken", () => {
    const c = saleCopy(watch({ sale_state: "checking" }));

    expect(c.tone).toBe("muted");
    expect(c.guidance?.toLowerCase()).toContain("under a minute");
  });
});

describe("saleCopy — onsale_scheduled reflects the presale situation", () => {
  const onsale = "2026-09-17T15:00:00Z";

  it("with no presales, promises a single alert", () => {
    const c = saleCopy(watch({ sale_state: "onsale_scheduled", public_onsale_start: onsale, presale_count: 0 }));

    expect(c.headline).toMatch(/official sale opens/i);
    expect(c.guidance).toMatch(/email you the moment it opens/i);
    expect(c.guidance).not.toMatch(/presale/i);
  });

  it("with one presale, uses singular grammar", () => {
    const c = saleCopy(
      watch({
        sale_state: "onsale_scheduled",
        public_onsale_start: onsale,
        earliest_presale: "2026-09-15T15:00:00Z",
        presale_count: 1,
      }),
    );

    expect(c.guidance).toContain("1 presale");
    expect(c.guidance).not.toContain("1 presales");
    expect(c.guidance).toMatch(/there is/i);
  });

  it("with several presales, uses plural grammar", () => {
    const c = saleCopy(
      watch({
        sale_state: "onsale_scheduled",
        public_onsale_start: onsale,
        earliest_presale: "2026-09-15T15:00:00Z",
        presale_count: 8,
      }),
    );

    expect(c.guidance).toContain("8 presales");
    expect(c.guidance).toMatch(/there are/i);
  });
});

describe("formatSaleTime", () => {
  it("returns an empty string for a missing date rather than 'Invalid Date'", () => {
    expect(formatSaleTime(null)).toBe("");
  });

  it("formats a real timestamp into something readable", () => {
    const out = formatSaleTime("2026-09-17T15:00:00Z");

    expect(out).not.toBe("");
    expect(out).not.toContain("Invalid");
    // Rendered in the runner's local zone, so assert on shape rather than exact text.
    expect(out).toMatch(/\d/);
    expect(out).toMatch(/Sep/i);
  });

  it("does not crash on a malformed date", () => {
    // The API should never send this, but a render crash would blank the whole app.
    expect(() => formatSaleTime("not-a-date")).not.toThrow();
  });
});

describe("firesImmediately", () => {
  it("flags the states where a new watch would alert straight away", () => {
    expect(firesImmediately("on_sale")).toBe(true);
    expect(firesImmediately("presale_open")).toBe(true);
  });

  it("is false for states where a watch is genuinely useful", () => {
    for (const s of ["onsale_scheduled", "onsale_tbd", "checking", "cancelled", "rescheduled", "sale_closed"] as SaleState[]) {
      expect(firesImmediately(s)).toBe(false);
    }
  });
});
