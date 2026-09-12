import { useState } from "react";
import { api } from "../api";

// Alerts are withheld from unconfirmed addresses (the backend skips them), so an
// unverified user needs to know why their watches would stay silent.
export function VerifyBanner({ email }: { email: string }) {
  const [sent, setSent] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function resend() {
    setBusy(true);
    setErr(null);
    try {
      await api.resendVerification();
      setSent(true);
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="notice">
      <span>
        <strong>Confirm your email.</strong> We sent a link to <code>{email}</code>. Until you
        confirm it, your watches keep tracking these events but alerts won't be emailed.
      </span>
      {sent ? (
        <span className="muted">Sent — check your inbox.</span>
      ) : (
        <button type="button" onClick={resend} disabled={busy}>
          {busy ? "…" : "Resend email"}
        </button>
      )}
      {err && <span className="muted">{err}</span>}
    </div>
  );
}
