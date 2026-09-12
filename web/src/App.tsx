import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import type { Me, Notification, Watch } from "./types";
import { clearToken, getToken } from "./auth";
import { Login } from "./components/Login";
import { ResetPassword } from "./components/ResetPassword";
import { VerifyBanner } from "./components/VerifyBanner";
import { Privacy } from "./components/Privacy";
import { AccountSettings } from "./components/AccountSettings";
import { SearchBar } from "./components/SearchBar";
import { WatchList } from "./components/WatchList";
import { NotificationLog } from "./components/NotificationLog";

// The app has no router: these read the URL the browser landed on. Deep links
// work because the Go SPA handler serves index.html for any non-/api path.
const resetToken =
  window.location.pathname === "/reset-password"
    ? new URLSearchParams(window.location.search).get("token")
    : null;

const isPrivacyPage = window.location.pathname === "/privacy";

// Set by the backend's redirect after it consumes a verification link.
const verifyResult = (() => {
  const q = new URLSearchParams(window.location.search);
  if (q.has("verified")) return "ok" as const;
  if (q.has("verify_error")) return "failed" as const;
  return null;
})();

export function App() {
  const [authed, setAuthed] = useState(!!getToken());
  const [me, setMe] = useState<Me | null>(null);
  const [watches, setWatches] = useState<Watch[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [verifyBanner, setVerifyBanner] = useState(verifyResult);

  const refresh = useCallback(async () => {
    try {
      const [m, w, n] = await Promise.all([api.me(), api.listWatches(), api.notifications()]);
      setMe(m);
      setWatches(w);
      setNotifications(n);
      setError(null);
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    if (!authed) return;
    refresh();
    const t = setInterval(refresh, 20_000); // near-live: re-read the DB every 20s
    return () => clearInterval(t);
  }, [authed, refresh]);

  // Clear the one-shot ?verified= flag from the address bar so a refresh (or a
  // shared URL) doesn't replay the message.
  useEffect(() => {
    if (verifyResult) window.history.replaceState({}, "", "/");
  }, []);

  // Readable without an account — a privacy policy you must log in to read is useless.
  if (isPrivacyPage) {
    return <Privacy />;
  }

  if (resetToken) {
    return <ResetPassword token={resetToken} />;
  }

  if (!authed) {
    return <Login onAuthed={() => setAuthed(true)} />;
  }

  function logout() {
    clearToken();
    setMe(null);
    setWatches([]);
    setNotifications([]);
    setAuthed(false);
  }

  return (
    <div className="app">
      <header className="app-header">
        <div>
          <h1>🎟️ Ticket Availability Watcher</h1>
          <p className="tagline">Track live events and get alerted the moment tickets go on sale.</p>
        </div>
        <button className="logout" onClick={logout}>Log out</button>
      </header>

      {verifyBanner === "ok" && (
        <div className="notice">
          <span>✅ Your email is confirmed — alerts are on.</span>
          <button type="button" onClick={() => setVerifyBanner(null)}>Dismiss</button>
        </div>
      )}
      {verifyBanner === "failed" && (
        <div className="error">
          That verification link is invalid or has expired. Use “Resend email” below to get a new one.
        </div>
      )}

      {me && !me.email_verified && <VerifyBanner email={me.email} />}

      {error && <div className="error">Can't reach the backend: {error}</div>}

      <section>
        <h2>Find an event</h2>
        <SearchBar onWatchCreated={refresh} />
      </section>

      <section>
        <h2>My watches <span className="count">{watches.length}</span></h2>
        <WatchList watches={watches} onChanged={refresh} />
      </section>

      <section>
        <h2>Notifications <span className="count">{notifications.length}</span></h2>
        <NotificationLog notifications={notifications} />
      </section>

      {me && <AccountSettings me={me} onChanged={refresh} />}

      <footer className="app-footer">
        <a href="/privacy">Privacy policy</a>
      </footer>
    </div>
  );
}
