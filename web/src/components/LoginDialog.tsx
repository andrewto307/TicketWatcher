import { useState, type FormEvent } from "react";
import { api } from "../api";
import { setToken } from "../auth";
import { Dialog } from "../ui/Dialog";
import { Button } from "../ui/Button";

type Mode = "login" | "register" | "forgot";

const field =
  "h-10 w-full rounded-lg border border-line bg-surface px-3 text-sm text-ink " +
  "placeholder:text-muted transition-colors hover:border-line-strong focus:border-accent focus:outline-none";

export function LoginDialog({
  open,
  onClose,
  onAuthed,
  initialMode = "login",
}: {
  open: boolean;
  onClose: () => void;
  onAuthed: () => void;
  initialMode?: Mode;
}) {
  const [mode, setMode] = useState<Mode>(initialMode);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

  function switchTo(next: Mode) {
    setMode(next);
    setErr(null);
    setSent(false);
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      if (mode === "forgot") {
        await api.forgotPassword(email);
        setSent(true);
      } else {
        const { token } =
          mode === "login" ? await api.login(email, password) : await api.register(email, password);
        setToken(token);
        onAuthed();
      }
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  const title = mode === "login" ? "Log in" : mode === "register" ? "Create an account" : "Reset your password";

  return (
    <Dialog open={open} onClose={onClose} title={title}>
      {/* Deliberately the same confirmation whether or not the address is
          registered — the server answers identically so this screen can't be
          used to discover who has an account. */}
      {mode === "forgot" && sent ? (
        <>
          <div className="rounded-lg border border-line bg-surface-2 p-3 text-sm text-ink">
            If an account exists for <code>{email}</code>, a reset link is on its way. The link
            expires in an hour.
          </div>
          <button
            type="button"
            onClick={() => switchTo("login")}
            className="mt-3 text-sm font-medium text-accent underline-offset-4 hover:underline"
          >
            Back to login
          </button>
        </>
      ) : (
        <>
          <p className="-mt-2 mb-4 text-sm text-muted">
            {mode === "login"
              ? "Sign in to your watches."
              : mode === "register"
                ? "Takes a few seconds. We only need an email."
                : "We'll email you a link to reset your password."}
          </p>

          <form onSubmit={submit} className="flex flex-col gap-3">
            <input
              type="email"
              placeholder="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className={field}
            />
            {mode !== "forgot" && (
              <input
                type="password"
                placeholder="password (min 8 characters)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
                className={field}
              />
            )}
            {err && (
              <div className="rounded-lg border border-warn-border bg-warn-soft px-3 py-2 text-sm text-warn">
                {err}
              </div>
            )}
            <Button type="submit" variant="primary" disabled={busy} className="w-full">
              {busy ? "…" : mode === "login" ? "Log in" : mode === "register" ? "Register" : "Send reset link"}
            </Button>
          </form>

          <div className="mt-4 flex flex-col items-start gap-2 border-t border-line pt-4">
            <button
              type="button"
              onClick={() => switchTo(mode === "login" ? "register" : "login")}
              className="text-sm font-medium text-accent underline-offset-4 hover:underline"
            >
              {mode === "login" ? "Need an account? Register" : "Have an account? Log in"}
            </button>
            {mode === "login" && (
              <button
                type="button"
                onClick={() => switchTo("forgot")}
                className="text-sm font-medium text-accent underline-offset-4 hover:underline"
              >
                Forgot password?
              </button>
            )}
            {/* Visible before signing up, which is when it actually matters. */}
            <a href="/privacy" className="text-xs text-muted underline-offset-4 hover:underline">
              Privacy policy
            </a>
          </div>
        </>
      )}
    </Dialog>
  );
}
