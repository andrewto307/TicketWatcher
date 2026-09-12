import type { EventResult, Me, Notification, Snapshot, Watch } from "./types";
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

// For endpoints that answer 204 with no body but can still fail with a message.
async function jEmpty(res: Response): Promise<void> {
  if (res.ok) return;
  let msg = `error ${res.status}`;
  try {
    const b = (await res.json()) as { error?: string };
    if (b.error) msg = b.error;
  } catch {
    /* ignore */
  }
  throw new Error(msg);
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

  me: () => fetch(`${base}/me`, { headers: authHeaders() }).then(jProtected<Me>),

  resendVerification: () =>
    fetch(`${base}/auth/verify/resend`, { method: "POST", headers: authHeaders() }).then(jEmpty),

  forgotPassword: (email: string) =>
    fetch(`${base}/auth/forgot`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ email }),
    }).then(jEmpty),

  resetPassword: (token: string, password: string) =>
    fetch(`${base}/auth/reset`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ token, password }),
    }).then(jEmpty),

  search: (q: string) =>
    fetch(`${base}/search?q=${encodeURIComponent(q)}`, { headers: authHeaders() }).then(jProtected<EventResult[]>),

  listWatches: () => fetch(`${base}/watches`, { headers: authHeaders() }).then(jProtected<Watch[]>),

  createWatch: (body: { tm_event_id: string; condition_type: string; poll_interval_s?: number }) =>
    fetch(`${base}/watches`, {
      method: "POST",
      headers: authHeaders({ "content-type": "application/json" }),
      body: JSON.stringify(body),
    }).then(jProtected<Watch>),

  history: (id: number) => fetch(`${base}/watches/${id}/history`, { headers: authHeaders() }).then(jProtected<Snapshot[]>),

  updateWatch: (id: number, body: { status?: string }) =>
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

  resubscribe: () =>
    fetch(`${base}/account/resubscribe`, { method: "POST", headers: authHeaders() }).then(jEmpty),

  deleteAccount: () =>
    fetch(`${base}/account`, { method: "DELETE", headers: authHeaders() }).then(jEmpty),
};
