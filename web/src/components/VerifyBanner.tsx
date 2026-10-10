import { useState } from "react";
import { MailWarning } from "lucide-react";
import { api } from "../api";
import { Button } from "../ui/Button";

// Alerts are withheld from unconfirmed addresses (the backend skips them), so an
// unverified user needs to know why their watches would stay silent. Shown on
// every logged-in page, because the consequence applies everywhere.
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
    <div className="flex flex-wrap items-center gap-3 rounded-lg border border-warn-border bg-warn-soft px-4 py-3">
      <MailWarning aria-hidden className="size-4 shrink-0 text-warn" />
      <span className="min-w-0 flex-1 text-sm text-ink">
        <strong className="font-semibold">Confirm your email.</strong> We sent a link to{" "}
        <code>{email}</code>. Until you confirm it, your watches keep tracking these events but
        alerts won't be emailed.
      </span>
      {sent ? (
        <span className="text-sm text-muted">Sent — check your inbox.</span>
      ) : (
        <Button type="button" onClick={resend} disabled={busy}>
          {busy ? "…" : "Resend email"}
        </Button>
      )}
      {err && <span className="text-sm text-muted">{err}</span>}
    </div>
  );
}
