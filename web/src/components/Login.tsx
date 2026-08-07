import { useState, type FormEvent } from "react";
import { api } from "../api";
import { setToken } from "../auth";

export function Login({ onAuthed }: { onAuthed: () => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      const { token } = mode === "login" ? await api.login(email, password) : await api.register(email, password);
      setToken(token);
      onAuthed();
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <h1>🎟️ Ticket Price Watcher</h1>
        <p className="tagline">{mode === "login" ? "Sign in to your watches." : "Create an account."}</p>
        <form onSubmit={submit} className="auth-form">
          <input
            type="email"
            placeholder="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <input
            type="password"
            placeholder="password (min 8 characters)"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
          {err && <div className="error">{err}</div>}
          <button type="submit" disabled={busy}>
            {busy ? "…" : mode === "login" ? "Log in" : "Register"}
          </button>
        </form>
        <button
          className="link"
          onClick={() => {
            setMode(mode === "login" ? "register" : "login");
            setErr(null);
          }}
        >
          {mode === "login" ? "Need an account? Register" : "Have an account? Log in"}
        </button>
      </div>
    </div>
  );
}
