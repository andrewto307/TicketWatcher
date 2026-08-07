// Minimal token storage. The JWT lives in localStorage and is attached to every
// authenticated request (see api.ts).
const KEY = "tw_token";

export function getToken(): string | null {
  return localStorage.getItem(KEY);
}

export function setToken(t: string): void {
  localStorage.setItem(KEY, t);
}

export function clearToken(): void {
  localStorage.removeItem(KEY);
}
