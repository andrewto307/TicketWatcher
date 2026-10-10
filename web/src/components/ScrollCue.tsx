import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";

/**
 * The first screen ends exactly at the fold, so nothing signals that the page
 * continues — without this a visitor can read the hero and leave, never seeing
 * the product. Disappears once they have scrolled, since it has done its job.
 *
 * The bob is a plain CSS animation, so the global prefers-reduced-motion rule
 * in styles.css stops it along with everything else.
 */
export function ScrollCue() {
  const [past, setPast] = useState(false);

  useEffect(() => {
    const onScroll = () => setPast(window.scrollY > 80);
    onScroll(); // a reload can restore a scroll position
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <div
      aria-hidden
      className={`pointer-events-none mt-8 flex justify-center transition-opacity duration-300 ${
        past ? "opacity-0" : "opacity-100"
      }`}
    >
      <ChevronDown className="animate-bob size-6 text-muted" />
    </div>
  );
}
