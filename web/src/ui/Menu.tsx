import { useEffect, useRef, useState, type ReactNode } from "react";

/** A dropdown anchored to its trigger. Escape and outside clicks close it. */
export function Menu({ label, children }: { label: ReactNode; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={wrap} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-line bg-surface px-3
                   text-sm font-medium text-ink transition-colors hover:bg-surface-2"
      >
        {label}
      </button>
      {open && (
        <div
          role="menu"
          onClick={() => setOpen(false)}
          className="animate-fade-up absolute right-0 z-40 mt-1.5 w-48 overflow-hidden rounded-lg
                     border border-line bg-surface p-1 shadow-lg"
        >
          {children}
        </div>
      )}
    </div>
  );
}
