import type { ReactNode } from "react";

/**
 * Wraps a screenshot in minimal browser chrome. It costs almost nothing and
 * makes the image read as a running product rather than a PNG dropped on the
 * page. Decorative, so the whole frame is hidden from assistive tech — the
 * screenshot inside carries the alt text.
 */
export function BrowserFrame({ children, label }: { children: ReactNode; label?: string }) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface shadow-lg">
      <div aria-hidden className="flex items-center gap-2 border-b border-line bg-surface-2 px-3 py-2">
        <span className="flex gap-1.5">
          <span className="size-2.5 rounded-full bg-line-strong" />
          <span className="size-2.5 rounded-full bg-line-strong" />
          <span className="size-2.5 rounded-full bg-line-strong" />
        </span>
        <span className="mx-auto max-w-[60%] truncate rounded-md bg-canvas px-3 py-0.5 text-[11px] text-muted">
          {label ?? "ticketwatcher.fly.dev"}
        </span>
      </div>
      {children}
    </div>
  );
}
