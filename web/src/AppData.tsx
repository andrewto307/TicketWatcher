import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api } from "./api";
import type { Me, Notification, Watch } from "./types";

// One fetch of the account's state, shared by every page. Pages read from here
// rather than fetching for themselves, so moving between /watches and /saved
// does not re-hit the API.

interface AppDataValue {
  me: Me | null;
  watches: Watch[];
  saved: Watch[];
  notifications: Notification[];
  /** Notifications grouped by the watch they belong to, newest first. */
  alertsByWatch: Map<number, Notification[]>;
  /** Ticketmaster ids the user already tracks, so search can say so. */
  trackedIds: Map<string, "watching" | "saved">;
  error: string | null;
  loading: boolean;
  refresh: () => Promise<void>;
}

const Ctx = createContext<AppDataValue | null>(null);

export function useAppData(): AppDataValue {
  const v = useContext(Ctx);
  if (!v) throw new Error("useAppData must be used inside <AppDataProvider>");
  return v;
}

export function AppDataProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [all, setAll] = useState<Watch[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const [m, w, n] = await Promise.all([api.me(), api.listWatches(), api.notifications()]);
      setMe(m);
      setAll(w);
      setNotifications(n);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 20_000); // near-live: re-read the DB every 20s
    return () => clearInterval(t);
  }, [refresh]);

  const value = useMemo<AppDataValue>(() => {
    const watches = all.filter((w) => w.status !== "paused");
    const saved = all.filter((w) => w.status === "paused");

    const alertsByWatch = new Map<number, Notification[]>();
    for (const n of [...notifications].sort((a, b) => b.sent_at.localeCompare(a.sent_at))) {
      const list = alertsByWatch.get(n.watch_id);
      if (list) list.push(n);
      else alertsByWatch.set(n.watch_id, [n]);
    }

    const trackedIds = new Map<string, "watching" | "saved">();
    for (const w of all) trackedIds.set(w.tm_event_id, w.status === "paused" ? "saved" : "watching");

    return { me, watches, saved, notifications, alertsByWatch, trackedIds, error, loading, refresh };
  }, [me, all, notifications, error, loading, refresh]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
