import type { EventResult, Notification, Snapshot, Watch } from "./types";
import { clearToken, getToken } from "./auth";

const base = "/api";

function authHeaders(extra?: Record<string, string>): Record<string, string> {
  const t = getToken();
  return { ...(extra ?? {}), ...(t ? { Authorization: `Bearer ${t}` } : {}) };
}

// For authenticated endpoints: a 401 means the session is gone -> back to login.
async function jProtected<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    clearToken();
    window.location.reload();
    throw new Error("session expired");
  }
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

// For login/register: surface the server's error message for display.
async function jAuth<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let msg = `error ${res.status}`;
    try {
      const b = (await res.json()) as { error?: string };
      if (b.error) msg = b.error;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export const api = {
  register: (email: string, password: string) =>
    fetch(`${base}/auth/register`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ email, password }),
    }).then(jAuth<{ token: string }>),

  login: (email: string, password: string) =>
    fetch(`${base}/auth/login`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ email, password }),
    }).then(jAuth<{ token: string }>),

  search: (q: string) =>
    fetch(`${base}/search?q=${encodeURIComponent(q)}`, { headers: authHeaders() }).then(jProtected<EventResult[]>),

  listWatches: () => fetch(`${base}/watches`, { headers: authHeaders() }).then(jProtected<Watch[]>),

  createWatch: (body: { tm_event_id: string; condition_type: string; threshold?: number; poll_interval_s?: number }) =>
    fetch(`${base}/watches`, {
      method: "POST",
      headers: authHeaders({ "content-type": "application/json" }),
      body: JSON.stringify(body),
    }).then(jProtected<Watch>),

  history: (id: number) => fetch(`${base}/watches/${id}/history`, { headers: authHeaders() }).then(jProtected<Snapshot[]>),

  updateWatch: (id: number, body: { threshold?: number; status?: string }) =>
    fetch(`${base}/watches/${id}`, {
      method: "PATCH",
      headers: authHeaders({ "content-type": "application/json" }),
      body: JSON.stringify(body),
    }).then(jProtected<Watch>),

  deleteWatch: (id: number) =>
    fetch(`${base}/watches/${id}`, { method: "DELETE", headers: authHeaders() }).then((res) => {
      if (res.status === 401) {
        clearToken();
        window.location.reload();
        return;
      }
      if (!res.ok) throw new Error(`${res.status}`);
    }),

  notifications: () => fetch(`${base}/notifications`, { headers: authHeaders() }).then(jProtected<Notification[]>),
};
