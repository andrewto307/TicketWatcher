import { useEffect, useState } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { getTheme, setTheme, watchSystemTheme, type Theme } from "../theme";
import { cn } from "../ui/cn";

const options: Array<{ value: Theme; label: string; Icon: typeof Sun }> = [
  { value: "light", label: "Light", Icon: Sun },
  { value: "dark", label: "Dark", Icon: Moon },
  { value: "system", label: "System", Icon: Monitor },
];

/**
 * Three explicit choices rather than a cycling button: with a cycle you cannot
 * tell what the next press will do, or whether you are currently following the
 * OS. "System" is the default and stays live if the OS setting changes.
 */
export function ThemeToggle() {
  const [theme, setLocal] = useState<Theme>("system");

  // Read on mount, not during render: the value comes from localStorage, which
  // does not exist during SSR or the first server-rendered pass.
  useEffect(() => {
    setLocal(getTheme());
    return watchSystemTheme();
  }, []);

  function choose(t: Theme) {
    setTheme(t);
    setLocal(t);
  }

  return (
    <div role="radiogroup" aria-label="Colour theme" className="flex gap-0.5 rounded-lg bg-surface-2 p-0.5">
      {options.map(({ value, label, Icon }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={theme === value}
          aria-label={label}
          title={label}
          onClick={() => choose(value)}
          className={cn(
            "grid h-7 flex-1 place-items-center rounded-md transition-colors",
            theme === value
              ? "bg-surface text-ink shadow-[0_1px_2px_rgb(0_0_0/0.06)]"
              : "text-muted hover:text-ink",
          )}
        >
          <Icon aria-hidden className="size-4" />
        </button>
      ))}
    </div>
  );
}
