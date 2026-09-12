import { useState } from "react";
import { api } from "../api";
import { clearToken } from "../auth";
import type { Me } from "../types";

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
    <section>
      <h2>Account</h2>
      <p className="muted">
        Signed in as <code>{me.email}</code>
      </p>

      {me.unsubscribed && (
        <div className="notice">
          <span>
            <strong>Alerts are off.</strong> You unsubscribed, so your watches keep tracking these
            events but nothing is emailed.
          </span>
          <button type="button" onClick={resubscribe} disabled={busy}>
            {busy ? "…" : "Turn alerts back on"}
          </button>
        </div>
      )}

      {err && <div className="error">{err}</div>}

      {/* Deletion is irreversible, so it takes a deliberate act — not a stray
          click on a button sitting next to everyday controls. */}
      {confirming ? (
        <div className="error">
          <p style={{ marginTop: 0 }}>
            <strong>This permanently deletes your account</strong> — every watch, its status history,
            and your notification log. It cannot be undone.
          </p>
          <p>
            Type <code>DELETE</code> to confirm:
          </p>
          <input
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            placeholder="DELETE"
            aria-label="Type DELETE to confirm"
          />
          <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
            <button className="danger" onClick={deleteAccount} disabled={busy || confirmText !== "DELETE"}>
              {busy ? "…" : "Permanently delete my account"}
            </button>
            <button
              onClick={() => {
                setConfirming(false);
                setConfirmText("");
              }}
              disabled={busy}
            >
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <button className="danger" onClick={() => setConfirming(true)}>
          Delete account
        </button>
      )}
    </section>
  );
}
