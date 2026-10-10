// Theme preference: the OS setting by default, with an explicit override the
// user can set. The resolved value ("light" | "dark") is written to
// <html data-theme>, which is the single hook the stylesheet keys off — so the
// CSS carries one dark block rather than duplicating it for a media query.

export type Theme = "light" | "dark" | "system";
export type Resolved = "light" | "dark";

const KEY = "tw_theme";

function systemPrefersDark(): boolean {
  return typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches;
}

export function getTheme(): Theme {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    // Private browsing, or storage blocked — fall back to following the OS.
  }
  return "system";
}

export function resolveTheme(t: Theme): Resolved {
  return t === "system" ? (systemPrefersDark() ? "dark" : "light") : t;
}

export function applyTheme(t: Theme): void {
  document.documentElement.dataset.theme = resolveTheme(t);
}

export function setTheme(t: Theme): void {
  try {
    if (t === "system") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, t);
  } catch {
    // Preference just won't survive a reload; the page still changes.
  }
  applyTheme(t);
}

/**
 * Keeps "system" live: if the user flips their OS appearance while the page is
 * open, follow it — but only while no explicit override is set.
 */
export function watchSystemTheme(): () => void {
  if (typeof window === "undefined" || !window.matchMedia) return () => {};
  const mq = window.matchMedia("(prefers-color-scheme: dark)");
  const onChange = () => {
    if (getTheme() === "system") applyTheme("system");
  };
  mq.addEventListener("change", onChange);
  return () => mq.removeEventListener("change", onChange);
}
