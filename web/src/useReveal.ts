import { useEffect, useRef, useState } from "react";

/**
 * Fades and lifts an element the first time it scrolls into view.
 *
 * The hidden state is applied from here, never from the stylesheet. If it lived
 * in CSS and this hook failed to run — a JS error, an old browser, a blocked
 * bundle — the page would be permanently blank. Starting visible means the
 * worst case is simply no animation.
 *
 * Fires once: re-animating on the way back up draws attention to the mechanism
 * rather than the content.
 */
export function useReveal<T extends HTMLElement = HTMLDivElement>(delayMs = 0) {
  const ref = useRef<T>(null);
  const [shown, setShown] = useState(true);
  const [armed, setArmed] = useState(false);

  useEffect(() => {
    const el = ref.current;
    // No observer (jsdom, very old browsers) means no animation, not a blank page.
    if (!el || typeof IntersectionObserver === "undefined") return;

    // Anything already on screen at first paint should not animate in late.
    const onScreen = el.getBoundingClientRect().top < window.innerHeight;
    if (onScreen) return;

    setArmed(true);
    setShown(false);

    const io = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) return;
        io.disconnect();
        setShown(true);
      },
      // A little early, so the content has settled by the time it is read.
      { rootMargin: "0px 0px -10% 0px", threshold: 0.05 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  const style = armed
    ? ({
        opacity: shown ? 1 : 0,
        transform: shown ? "none" : "translateY(20px)",
        transition: `opacity 500ms cubic-bezier(0.22, 1, 0.36, 1) ${delayMs}ms,
                     transform 500ms cubic-bezier(0.22, 1, 0.36, 1) ${delayMs}ms`,
      } as const)
    : undefined;

  return { ref, style };
}
