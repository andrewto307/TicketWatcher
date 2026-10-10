import { useState } from "react";
import { BellOff } from "lucide-react";
import { api } from "../api";
import { clearToken } from "../auth";
import type { Me } from "../types";
import { Button } from "../ui/Button";
import { Card } from "../ui/Card";

const field =
  "h-10 w-full rounded-lg border border-line bg-surface px-3 text-sm text-ink " +
  "placeholder:text-muted focus:border-accent focus:outline-none";

// Alert opt-out and account deletion — the two controls Tier 3 requires a user
// to have over their own data.
export function AccountSettings({ me, onChanged }: { me: Me; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [confirmText, setConfirmText] = useState("");

  async function resubscribe() {
    setBusy(true);
    setErr(null);
    try {
      await api.resubscribe();
      onChanged();
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
    } finally {
      setBusy(false);
    }
  }

  async function deleteAccount() {
    setBusy(true);
    setErr(null);
    try {
      await api.deleteAccount();
      // The account is gone; the stored token now refers to nothing.
      clearToken();
      window.location.href = "/";
    } catch (e) {
      setErr(String(e).replace(/^Error:\s*/, ""));
      setBusy(false);
    }
  }

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <Card className="p-4">
        <h2 className="text-sm font-semibold text-ink">Account</h2>
        <p className="mt-1 text-sm text-muted">
          Signed in as <code className="text-ink">{me.email}</code>
        </p>
      </Card>

      <Card className="p-4">
        <h2 className="text-sm font-semibold text-ink">Email alerts</h2>
        {me.unsubscribed ? (
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <BellOff aria-hidden className="size-4 text-warn" />
            <span className="min-w-0 flex-1 text-sm text-ink">
              <strong className="font-semibold">Alerts are off.</strong> You unsubscribed, so your
              watches keep tracking these events but nothing is emailed.
            </span>
            <Button type="button" onClick={resubscribe} disabled={busy}>
              {busy ? "…" : "Turn alerts back on"}
            </Button>
          </div>
        ) : (
          <p className="mt-1 text-sm text-muted">
            Alerts are on. Every alert email also carries an unsubscribe link.
          </p>
        )}
      </Card>

      {err && (
        <div className="rounded-lg border border-warn-border bg-warn-soft px-4 py-3 text-sm text-warn">
          {err}
        </div>
      )}

      {/* Deletion is irreversible, so it takes a deliberate act — not a stray
          click on a button sitting next to everyday controls. */}
      <Card className="border-danger/30 p-4">
        <h2 className="text-sm font-semibold text-ink">Danger zone</h2>
        {confirming ? (
          <div className="mt-2">
            <p className="text-sm text-ink">
              <strong className="font-semibold">This permanently deletes your account</strong> —
              every watch, its status history, and your notification log. It cannot be undone.
            </p>
            <p className="mt-2 text-sm text-muted">
              Type <code className="text-ink">DELETE</code> to confirm:
            </p>
            <input
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder="DELETE"
              aria-label="Type DELETE to confirm"
              className={`${field} mt-2 max-w-xs`}
            />
            <div className="mt-3 flex gap-2">
              <Button
                variant="danger"
                onClick={deleteAccount}
                disabled={busy || confirmText !== "DELETE"}
              >
                {busy ? "…" : "Permanently delete my account"}
              </Button>
              <Button
                onClick={() => {
                  setConfirming(false);
                  setConfirmText("");
                }}
                disabled={busy}
              >
                Cancel
              </Button>
            </div>
          </div>
        ) : (
          <>
            <p className="mt-1 text-sm text-muted">
              Deleting removes your watches, status history, and notification log immediately.
            </p>
            <Button variant="danger" className="mt-3" onClick={() => setConfirming(true)}>
              Delete account
            </Button>
          </>
        )}
      </Card>
    </div>
  );
}
