import { Link, NavLink, Outlet } from "react-router-dom";
import { Bookmark, ChevronDown, Eye, LogOut, Settings } from "lucide-react";
import { useAppData } from "../AppData";
import { Wordmark } from "./Wordmark";
import { VerifyBanner } from "./VerifyBanner";
import { Menu } from "../ui/Menu";
import { ThemeToggle } from "./ThemeToggle";

const item =
  "flex items-center gap-2 rounded-md px-2.5 py-2 text-sm text-ink transition-colors hover:bg-surface-2";

export function AppLayout({ onLogout }: { onLogout: () => void }) {
  const { me, watches, saved, error } = useAppData();

  return (
    <div className="min-h-dvh bg-canvas">
      <header className="sticky top-0 z-30 border-b border-line bg-canvas/85 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between gap-4 px-4">
          {/* The wordmark is how you get back to search from anywhere. */}
          <Link to="/" aria-label="TicketWatcher home" className="rounded-lg">
            <Wordmark />
          </Link>

          <Menu
            label={
              <>
                <span className="hidden max-w-[12rem] truncate sm:inline">{me?.email ?? "Account"}</span>
                <span className="sm:hidden">Account</span>
                <ChevronDown aria-hidden className="size-4 text-muted" />
              </>
            }
          >
            <NavLink to="/watches" className={item} role="menuitem">
              <Eye aria-hidden className="size-4 text-muted" />
              My watches
              <span className="ml-auto text-xs text-muted">{watches.length}</span>
            </NavLink>
            <NavLink to="/saved" className={item} role="menuitem">
              <Bookmark aria-hidden className="size-4 text-muted" />
              Saved
              <span className="ml-auto text-xs text-muted">{saved.length}</span>
            </NavLink>
            <NavLink to="/settings" className={item} role="menuitem">
              <Settings aria-hidden className="size-4 text-muted" />
              Settings
            </NavLink>
            <div className="my-1 h-px bg-line" />
            {/* Clicking inside must not close the menu mid-choice. */}
            <div className="px-2.5 py-2" onClick={(e) => e.stopPropagation()}>
              <p className="mb-1.5 text-xs text-muted">Theme</p>
              <ThemeToggle />
            </div>
            <div className="my-1 h-px bg-line" />
            <button type="button" role="menuitem" onClick={onLogout} className={`${item} w-full`}>
              <LogOut aria-hidden className="size-4 text-muted" />
              Log out
            </button>
          </Menu>
        </div>
      </header>

      <main className="mx-auto w-full max-w-5xl px-4 pb-16">
        {/* Account-level and true on every page: an unverified account is polled
            normally but never emailed, so this must not be hidden behind a tab. */}
        {me && !me.email_verified && (
          <div className="pt-4">
            <VerifyBanner email={me.email} />
          </div>
        )}

        {error && (
          <div className="mt-4 rounded-lg border border-warn-border bg-warn-soft px-4 py-3 text-sm text-warn">
            Can't reach the backend: {error}
          </div>
        )}

        <Outlet />
      </main>

      <footer className="border-t border-line">
        <div className="mx-auto flex max-w-5xl justify-center px-4 py-6">
          <Link to="/privacy" className="text-sm text-muted underline-offset-4 hover:text-ink hover:underline">
            Privacy policy
          </Link>
        </div>
      </footer>
    </div>
  );
}
