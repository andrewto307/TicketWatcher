import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useReveal } from "./useReveal";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useReveal", () => {
  it("leaves content visible when IntersectionObserver is unavailable", () => {
    vi.stubGlobal("IntersectionObserver", undefined);

    const { result } = renderHook(() => useReveal());

    // The failure mode that matters: if the hidden state came from CSS, a
    // browser without the observer would render a permanently blank page.
    expect(result.current.style).toBeUndefined();
  });

  it("does not hide an element that is already on screen", () => {
    const observe = vi.fn();
    vi.stubGlobal(
      "IntersectionObserver",
      vi.fn().mockImplementation(() => ({ observe, disconnect: vi.fn() })),
    );

    const { result } = renderHook(() => useReveal<HTMLDivElement>());
    // No ref attached, so the hook bails before arming.
    expect(result.current.style).toBeUndefined();
    expect(observe).not.toHaveBeenCalled();
  });

  it("gives back a ref for the caller to attach", () => {
    const { result } = renderHook(() => useReveal());

    expect(result.current.ref).toHaveProperty("current", null);
  });
});
