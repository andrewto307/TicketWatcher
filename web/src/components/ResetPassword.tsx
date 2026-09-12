import { useState, type FormEvent } from "react";
import { api } from "../api";

// Shown when the user opens the /reset-password?token=… link from their email.
// The token is read straight from the URL — no client-side router needed, since
// the Go SPA handler serves index.html for this path.
export function ResetPassword({ token }: { token: string }) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setErr("The two passwords don't match.");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await api.resetPassword(token, password);
      setDone(true);
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  // Send them back to a clean URL so the spent token isn't left in the address bar.
  function toLogin() {
    window.location.href = "/";
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <h1>🎟️ Ticket Availability Watcher</h1>
        {done ? (
          <>
            <p className="tagline">Your password has been changed.</p>
            <button type="button" onClick={toLogin}>
              Go to login
            </button>
          </>
        ) : (
          <>
            <p className="tagline">Choose a new password.</p>
            <form onSubmit={submit} className="auth-form">
              <input
                type="password"
                placeholder="new password (min 8 characters)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                autoFocus
              />
              <input
                type="password"
                placeholder="confirm new password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                required
                minLength={8}
              />
              {err && <div className="error">{err}</div>}
              <button type="submit" disabled={busy}>
                {busy ? "…" : "Set new password"}
              </button>
            </form>
            <button className="link" onClick={toLogin}>
              Back to login
            </button>
          </>
        )}
      </div>
    </div>
  );
}
