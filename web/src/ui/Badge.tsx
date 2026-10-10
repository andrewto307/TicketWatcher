import type { ReactNode } from "react";
import { cn } from "./cn";

export type Tone = "good" | "warn" | "muted";

const tones: Record<Tone, string> = {
  good: "bg-good-soft text-good border-good-border",
  warn: "bg-warn-soft text-warn border-warn-border",
  muted: "bg-surface-2 text-muted border-line",
};

export function Badge({ tone = "muted", children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded-full border px-2 py-0.5",
        "text-[11px] font-semibold tracking-wide uppercase",
        tones[tone],
      )}
    >
      {children}
    </span>
  );
}
