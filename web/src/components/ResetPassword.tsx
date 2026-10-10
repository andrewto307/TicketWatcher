import { useState, type FormEvent } from "react";
import { api } from "../api";
import { Wordmark } from "./Wordmark";
import { Button } from "../ui/Button";

const field =
  "h-10 w-full rounded-lg border border-line bg-surface px-3 text-sm text-ink " +
  "placeholder:text-muted transition-colors hover:border-line-strong focus:border-accent focus:outline-none";

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
    <div className="grid min-h-dvh place-items-center bg-canvas px-4 py-10">
      <div className="w-full max-w-sm rounded-xl border border-line bg-surface p-6 shadow-sm">
        <div className="mb-5 flex justify-center">
          <Wordmark />
        </div>
        {done ? (
          <>
            <p className="mb-4 text-center text-sm text-muted">Your password has been changed.</p>
            <Button type="button" variant="primary" onClick={toLogin} className="w-full">
              Go to login
            </Button>
          </>
        ) : (
          <>
            <p className="mb-4 text-center text-sm text-muted">Choose a new password.</p>
            <form onSubmit={submit} className="flex flex-col gap-3">
              <input
                type="password"
                placeholder="new password (min 8 characters)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                autoFocus
                className={field}
              />
              <input
                type="password"
                placeholder="confirm new password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                required
                minLength={8}
                className={field}
              />
              {err && (
                <div className="rounded-lg border border-warn-border bg-warn-soft px-3 py-2 text-sm text-warn">
                  {err}
                </div>
              )}
              <Button type="submit" variant="primary" disabled={busy} className="w-full">
                {busy ? "…" : "Set new password"}
              </Button>
            </form>
            <button
              type="button"
              onClick={toLogin}
              className="mt-4 text-sm font-medium text-accent underline-offset-4 hover:underline"
            >
              Back to login
            </button>
          </>
        )}
      </div>
    </div>
  );
}
