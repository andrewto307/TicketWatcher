import { Loader2 } from "lucide-react";
import { cn } from "./cn";

export function Spinner({ className, label }: { className?: string; label?: string }) {
  return (
    <span role="status" aria-live="polite" className="inline-flex items-center gap-2 text-muted">
      <Loader2 aria-hidden className={cn("size-4 animate-spin", className)} />
      {label && <span className="text-sm">{label}</span>}
    </span>
  );
}
