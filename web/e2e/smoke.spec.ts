import { expect, test } from "@playwright/test";
import { collectConsoleErrors, mockApi, search, signIn, stubEvent } from "./helpers";

// One question per page: is everything that should be here actually here?
// These run in a real browser against the real binary, which is what jsdom
// cannot do — it applies no layout, never exercises the router against the
// server's index.html fallback, and cannot tell a missing component from one
// that CSS has pushed off-screen.

test.describe("logged out", () => {
  test("the landing page sells the product and hides the form", async ({ page }) => {
    const errors = collectConsoleErrors(page);
    await page.goto("/");

    await expect(page.getByRole("heading", { name: /go on sale/i })).toBeVisible();
    await expect(page.getByRole("banner").getByRole("button", { name: "Log in" })).toBeVisible();
    await expect(page.getByRole("button", { name: /get started/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: /how it works/i })).toBeVisible();
    await expect(page.getByRole("region", { name: /screenshots/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /privacy policy/i })).toBeVisible();

    // No dashboard chrome leaking onto a page for people without an account.
    await expect(page.getByPlaceholder(/search artists/i)).toHaveCount(0);
    await expect(page.getByRole("dialog")).toHaveCount(0);

    expect(errors).toEqual([]);
  });

  test("the login modal opens with every control it needs", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("banner").getByRole("button", { name: "Log in" }).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByPlaceholder("email")).toBeVisible();
    await expect(dialog.getByPlaceholder(/password/i)).toBeVisible();
    await expect(dialog.getByRole("button", { name: /need an account\? register/i })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /forgot password/i })).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("app routes redirect to the landing page", async ({ page }) => {
    for (const path of ["/watches", "/saved", "/settings"]) {
      await page.goto(path);
      await expect(page.getByRole("button", { name: /get started/i })).toBeVisible();
      await expect(page).toHaveURL("/");
    }
  });

  test("the privacy policy is readable without an account", async ({ page }) => {
    const errors = collectConsoleErrors(page);
    await page.goto("/privacy");

    await expect(page.getByRole("heading", { name: /privacy policy/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /back to app/i })).toBeVisible();
    expect(errors).toEqual([]);
  });

  test("a reset link renders a real page, not a modal", async ({ page }) => {
    // Arrives from an email, so it has to survive a cold page load.
    await page.goto("/reset-password?token=not-a-real-token");

    await expect(page.getByPlaceholder(/^new password/i)).toBeVisible();
    await expect(page.getByPlaceholder(/confirm new password/i)).toBeVisible();
  });
});

test.describe("logged in", () => {
  test("search starts centred and moves up once submitted", async ({ page }) => {
    const errors = collectConsoleErrors(page);
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);

    // Before searching the bar is the whole page: no results region at all.
    const barBefore = await page.getByPlaceholder(/search artists/i).boundingBox();
    await expect(page.getByRole("listitem")).toHaveCount(0);

    await search(page);

    await expect(page.getByText("E2E Future Fest")).toBeVisible();
    const barAfter = await page.getByPlaceholder(/search artists/i).boundingBox();
    expect(barAfter!.y).toBeLessThan(barBefore!.y);

    expect(errors).toEqual([]);
  });

  test("an empty result still moves the bar up", async ({ page }) => {
    await mockApi(page, { events: [] });
    await signIn(page);
    const barBefore = await page.getByPlaceholder(/search artists/i).boundingBox();

    await search(page);

    // Keyed off having searched, not off having results — otherwise the
    // Search button looks broken when nothing matches.
    await expect(page.getByText(/no events found/i)).toBeVisible();
    const barAfter = await page.getByPlaceholder(/search artists/i).boundingBox();
    expect(barAfter!.y).toBeLessThan(barBefore!.y);
  });

  test("a result card carries a Ticketmaster link, Watch and Save", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);
    await search(page);

    await expect(page.getByRole("link", { name: /open e2e future fest/i })).toBeVisible();
    await expect(page.getByRole("button", { name: "Watch", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeVisible();
  });

  test("repeated dates for one attraction collapse into a group", async ({ page }) => {
    await mockApi(page, {
      events: [
        stubEvent({ tm_event_id: "G1", name: "Residency Run" }),
        stubEvent({ tm_event_id: "G2", name: "Residency Run" }),
        stubEvent({ tm_event_id: "G3", name: "Residency Run" }),
      ],
    });
    await signIn(page);
    await search(page);

    await expect(page.getByText("3 dates")).toBeVisible();
    await page.getByRole("button", { name: /Residency Run/ }).click();
    await expect(page.getByRole("button", { name: "Watch", exact: true })).toHaveCount(3);
  });
});

test.describe("watching and saving", () => {
  test("Watch confirms, lands in /watches, and the result says so next time", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);
    await search(page);

    await page.getByRole("button", { name: "Watch", exact: true }).click();

    // The list is a different page now, so without this the click looks inert.
    await expect(page.getByRole("status")).toContainText(/watching "e2e future fest"/i);

    await page.goto("/watches");
    await expect(page.getByRole("heading", { name: /my watches/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: "E2E Future Fest" })).toBeVisible();

    // Searching again must not offer to add it a second time.
    await page.goto("/");
    await search(page);
    await expect(page.getByText("Watching")).toBeVisible();
    await expect(page.getByRole("button", { name: "Watch", exact: true })).toHaveCount(0);
  });

  test("Save puts the event straight into /saved, not /watches", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);
    await search(page);

    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText(/saved "e2e future fest"/i);

    await page.goto("/saved");
    await expect(page.getByRole("heading", { name: "E2E Future Fest" })).toBeVisible();

    await page.goto("/watches");
    await expect(page.getByText(/don't have any active watches/i)).toBeVisible();
  });

  test("Pause moves a card from watches to saved, and Watch brings it back", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);
    await search(page);
    await page.getByRole("button", { name: "Watch", exact: true }).click();

    await page.goto("/watches");
    await page.getByRole("button", { name: "Pause" }).click();
    await expect(page.getByRole("status")).toContainText(/moved .* to saved/i);
    await expect(page.getByText(/don't have any active watches/i)).toBeVisible();

    await page.goto("/saved");
    await expect(page.getByRole("heading", { name: "E2E Future Fest" })).toBeVisible();
    await page.getByRole("button", { name: "Watch", exact: true }).click();
    await expect(page.getByRole("status")).toContainText(/watching .* again/i);

    await page.goto("/watches");
    await expect(page.getByRole("heading", { name: "E2E Future Fest" })).toBeVisible();
  });

  test("a watch card shows its alert state and status history", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await signIn(page);
    await search(page);
    await page.getByRole("button", { name: "Watch", exact: true }).click();
    await page.goto("/watches");

    // Alert history sits on the card it belongs to rather than a separate page.
    await expect(page.getByText(/no alerts sent yet/i)).toBeVisible();

    await page.getByRole("button", { name: /status history/i }).click();
    await expect(page.getByText(/no changes recorded yet|onsale|offsale|unknown/i).first()).toBeVisible();
  });
});

test.describe("the signed-in shell", () => {
  test("every destination is reachable from the profile menu", async ({ page }) => {
    await mockApi(page);
    await signIn(page);

    await page.getByRole("button", { name: /account|@example\.test/i }).first().click();
    const menu = page.getByRole("menu");
    // Each entry carries a count, so match the item rather than exact text.
    await expect(menu.getByRole("menuitem", { name: /my watches/i })).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: /saved/i })).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: /settings/i })).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: /log out/i })).toBeVisible();
  });

  test("the wordmark gets you home from any page", async ({ page }) => {
    await mockApi(page);
    await signIn(page);
    await page.goto("/watches");

    await page.getByRole("link", { name: /ticketwatcher home/i }).click();

    await expect(page).toHaveURL("/");
    await expect(page.getByPlaceholder(/search artists/i)).toBeVisible();
  });

  test("settings offers only the controls that exist", async ({ page }) => {
    const errors = collectConsoleErrors(page);
    await mockApi(page);
    await signIn(page);
    await page.goto("/settings");

    await expect(page.getByRole("heading", { name: /settings/i })).toBeVisible();
    // The address also appears in the verify banner, so scope to the card.
    await expect(page.getByText(/signed in as/i)).toContainText("e2e@example.test");
    await expect(page.getByText(/email alerts/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /delete account/i })).toBeVisible();
    expect(errors).toEqual([]);
  });

  test("the verify banner follows the user across every page", async ({ page }) => {
    await mockApi(page, { emailVerified: false });
    await signIn(page);

    // A fresh account is unverified: its watches poll but are never emailed,
    // so this warning must not be hidden behind one tab.
    for (const path of ["/", "/watches", "/saved", "/settings"]) {
      await page.goto(path);
      await expect(page.getByText(/confirm your email/i)).toBeVisible();
      await expect(page.getByRole("button", { name: /resend email/i })).toBeVisible();
    }
  });

  test("logging out returns to the landing page", async ({ page }) => {
    await mockApi(page);
    await signIn(page);

    await page.getByRole("button", { name: /account|@example\.test/i }).first().click();
    await page.getByRole("menuitem", { name: /log out/i }).click();

    await expect(page.getByRole("button", { name: /get started/i })).toBeVisible();
  });
});

test.describe("deep links survive a cold load", () => {
  // This is the only thing that exercises the Go spaHandler's index.html
  // fallback; no unit test touches it.
  test("every route loads directly and on refresh", async ({ page }) => {
    await mockApi(page);
    await signIn(page);

    for (const path of ["/watches", "/saved", "/settings", "/privacy"]) {
      const res = await page.goto(path);
      expect(res?.status(), `${path} should not 404`).toBe(200);
      await page.reload();
      expect(page.url()).toContain(path);
    }
  });
});

test.describe("responsive", () => {
  test("no horizontal scroll at phone width", async ({ page }) => {
    await mockApi(page, { events: [stubEvent()] });
    await page.setViewportSize({ width: 390, height: 844 });
    await signIn(page);

    for (const path of ["/", "/watches", "/saved", "/settings", "/privacy"]) {
      await page.goto(path);
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
      );
      expect(overflow, `${path} overflows horizontally`).toBeLessThanOrEqual(1);
    }
  });

  test("the profile menu is reachable on a phone", async ({ page }) => {
    await mockApi(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await signIn(page);

    await page.getByRole("button", { name: /account/i }).first().click();
    await expect(page.getByRole("menu")).toBeVisible();
  });
});

test.describe("theme", () => {
  test("defaults to the OS setting", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto("/");

    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

    await page.emulateMedia({ colorScheme: "light" });
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  });

  test("an explicit choice overrides the OS and survives a reload", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto("/");

    await page.getByRole("radio", { name: "Light" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

    // The pre-paint script in index.html has to agree with the stored value,
    // or a dark-mode user gets a white flash on every load.
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  });

  test("System returns control to the OS", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto("/");

    await page.getByRole("radio", { name: "Light" }).click();
    await page.getByRole("radio", { name: "System" }).click();

    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  });

  test("signed-in users can change it from the profile menu", async ({ page }) => {
    await mockApi(page);
    await signIn(page);

    await page.getByRole("button", { name: /account|@example\.test/i }).first().click();
    await page.getByRole("radio", { name: "Dark" }).click();

    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    // Choosing a theme must not dismiss the menu mid-decision.
    await expect(page.getByRole("menu")).toBeVisible();
  });
});
