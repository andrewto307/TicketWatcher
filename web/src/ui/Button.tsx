import type { ButtonHTMLAttributes } from "react";
import { cn } from "./cn";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md" | "lg";

const base =
  "inline-flex items-center justify-center gap-1.5 rounded-lg font-medium whitespace-nowrap " +
  "transition-colors duration-150 disabled:opacity-50 disabled:pointer-events-none";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-accent-fg hover:bg-accent-hover",
  secondary: "bg-surface text-ink border border-line hover:bg-surface-2 hover:border-line-strong",
  ghost: "text-muted hover:text-ink hover:bg-surface-2",
  danger: "text-danger border border-line hover:bg-danger-soft hover:border-danger",
};

const sizes: Record<Size, string> = {
  sm: "h-8 px-2.5 text-sm",
  md: "h-9 px-3.5 text-sm",
  lg: "h-11 px-5 text-base",
};

export function Button({
  variant = "secondary",
  size = "md",
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: Size }) {
  return <button className={cn(base, variants[variant], sizes[size], className)} {...rest} />;
}
