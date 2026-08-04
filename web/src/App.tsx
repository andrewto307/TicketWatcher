import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import type { Notification, Watch } from "./types";
import { SearchBar } from "./components/SearchBar";
import { WatchList } from "./components/WatchList";
import { NotificationLog } from "./components/NotificationLog";

export function App() {
  const [watches, setWatches] = useState<Watch[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [w, n] = await Promise.all([api.listWatches(), api.notifications()]);
      setWatches(w);
      setNotifications(n);
      setError(null);
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 20_000); // near-live: re-read the DB every 20s
    return () => clearInterval(t);
  }, [refresh]);

  return (
    <div className="app">
      <header className="app-header">
        <h1>🎟️ Ticket Price Watcher</h1>
        <p className="tagline">Track live events and get alerted the moment the price drops.</p>
      </header>

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
    </div>
  );
}
