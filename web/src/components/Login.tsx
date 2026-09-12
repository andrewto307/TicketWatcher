import { useState, type FormEvent } from "react";
import { api } from "../api";
import { setToken } from "../auth";

type Mode = "login" | "register" | "forgot";

export function Login({ onAuthed }: { onAuthed: () => void }) {
  const [mode, setMode] = useState<Mode>("login");
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

  const tagline =
    mode === "login"
      ? "Sign in to your watches."
      : mode === "register"
        ? "Create an account."
        : "We'll email you a link to reset your password.";

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <h1>🎟️ Ticket Availability Watcher</h1>
        <p className="tagline">{tagline}</p>

        {/* Deliberately the same confirmation whether or not the address is
            registered — the server answers identically so this screen can't be
            used to discover who has an account. */}
        {mode === "forgot" && sent ? (
          <>
            <div className="notice">
              If an account exists for <code>{email}</code>, a reset link is on its way. The link
              expires in an hour.
            </div>
            <button className="link" onClick={() => switchTo("login")}>
              Back to login
            </button>
          </>
        ) : (
          <>
            <form onSubmit={submit} className="auth-form">
              <input
                type="email"
                placeholder="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
              {mode !== "forgot" && (
                <input
                  type="password"
                  placeholder="password (min 8 characters)"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={8}
                />
              )}
              {err && <div className="error">{err}</div>}
              <button type="submit" disabled={busy}>
                {busy ? "…" : mode === "login" ? "Log in" : mode === "register" ? "Register" : "Send reset link"}
              </button>
            </form>

            <div className="auth-links">
              <button className="link" onClick={() => switchTo(mode === "login" ? "register" : "login")}>
                {mode === "login" ? "Need an account? Register" : "Have an account? Log in"}
              </button>
              {mode === "login" && (
                <button className="link" onClick={() => switchTo("forgot")}>
                  Forgot password?
                </button>
              )}
              {/* Visible before signing up, which is when it actually matters. */}
              <a className="muted" href="/privacy" style={{ fontSize: "0.85rem", marginTop: 6 }}>
                Privacy policy
              </a>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
