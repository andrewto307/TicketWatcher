import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeToggle } from "./ThemeToggle";

// matchMedia does not exist in jsdom, so each test declares what the OS says.
function osPrefersDark(dark: boolean) {
  const listeners: Array<() => void> = [];
  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockImplementation((query: string) => ({
      matches: dark && query.includes("dark"),
      media: query,
      addEventListener: (_: string, fn: () => void) => listeners.push(fn),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  );
  return listeners;
}

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
  osPrefersDark(false);
});

describe("ThemeToggle", () => {
  it("offers all three choices", () => {
    render(<ThemeToggle />);

    expect(screen.getByRole("radio", { name: "Light" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Dark" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "System" })).toBeInTheDocument();
  });

  it("starts on System, so a new visitor gets their OS setting", () => {
    render(<ThemeToggle />);

    expect(screen.getByRole("radio", { name: "System" })).toHaveAttribute("aria-checked", "true");
  });

  it("applies an explicit dark choice and remembers it", async () => {
    render(<ThemeToggle />);

    await userEvent.click(screen.getByRole("radio", { name: "Dark" }));

    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(localStorage.getItem("tw_theme")).toBe("dark");
    expect(screen.getByRole("radio", { name: "Dark" })).toHaveAttribute("aria-checked", "true");
  });

  it("lets a light choice win over a dark OS", async () => {
    osPrefersDark(true);
    render(<ThemeToggle />);

    await userEvent.click(screen.getByRole("radio", { name: "Light" }));

    // The whole point of the control: the OS is dark, the user said light.
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("returning to System drops the override and follows the OS again", async () => {
    osPrefersDark(true);
    render(<ThemeToggle />);

    await userEvent.click(screen.getByRole("radio", { name: "Light" }));
    expect(document.documentElement.dataset.theme).toBe("light");

    await userEvent.click(screen.getByRole("radio", { name: "System" }));

    expect(document.documentElement.dataset.theme).toBe("dark");
    // Nothing stored means nothing to go stale if the OS changes later.
    expect(localStorage.getItem("tw_theme")).toBeNull();
  });

  it("reads a stored preference on mount", () => {
    localStorage.setItem("tw_theme", "dark");

    render(<ThemeToggle />);

    expect(screen.getByRole("radio", { name: "Dark" })).toHaveAttribute("aria-checked", "true");
  });
});
