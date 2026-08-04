import type { EventResult, Notification, Snapshot, Watch } from "./types";

const base = "/api";

async function j<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(`${res.status}: ${await res.text()}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  search: (q: string) =>
    fetch(`${base}/search?q=${encodeURIComponent(q)}`).then(j<EventResult[]>),

  listWatches: () => fetch(`${base}/watches`).then(j<Watch[]>),

  createWatch: (body: {
    tm_event_id: string;
    condition_type: string;
    threshold?: number;
    poll_interval_s?: number;
  }) =>
    fetch(`${base}/watches`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }).then(j<Watch>),

  history: (id: number) => fetch(`${base}/watches/${id}/history`).then(j<Snapshot[]>),

  updateWatch: (id: number, body: { threshold?: number; status?: string }) =>
    fetch(`${base}/watches/${id}`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }).then(j<Watch>),

  deleteWatch: (id: number) =>
    fetch(`${base}/watches/${id}`, { method: "DELETE" }).then((res) => {
      if (!res.ok) throw new Error(`${res.status}`);
    }),

  notifications: () => fetch(`${base}/notifications`).then(j<Notification[]>),
};
