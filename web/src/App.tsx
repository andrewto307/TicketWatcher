import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes, useSearchParams } from "react-router-dom";
import { clearToken, getToken } from "./auth";
import { watchSystemTheme } from "./theme";
import { AppDataProvider } from "./AppData";
import { ToastProvider } from "./ui/Toast";
import { AppLayout } from "./components/AppLayout";
import { Landing } from "./components/Landing";
import { Privacy } from "./components/Privacy";
import { ResetPassword } from "./components/ResetPassword";
import { SearchPage } from "./pages/SearchPage";
import { WatchesPage } from "./pages/WatchesPage";
import { SavedPage } from "./pages/SavedPage";
import { SettingsPage } from "./pages/SettingsPage";

// The router lives inside App rather than around it in main.tsx, so tests can
// set window.location and render <App /> directly.

export function App() {
  const [authed, setAuthed] = useState(!!getToken());

  // While the preference is "system", follow the OS if it changes mid-session.
  useEffect(watchSystemTheme, []);

  function logout() {
    clearToken();
    setAuthed(false);
  }

  return (
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          {/* Readable without an account — a privacy policy you must log in to
              read is useless. */}
          <Route path="/privacy" element={<Privacy />} />
          {/* Arrives from an email, so it has to be a real page, not a modal. */}
          <Route path="/reset-password" element={<ResetPasswordRoute />} />

          {authed ? (
            <Route element={<AuthedShell onLogout={logout} />}>
              <Route path="/" element={<SearchPage />} />
              <Route path="/watches" element={<WatchesPage />} />
              <Route path="/saved" element={<SavedPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
          ) : (
            <>
              <Route path="/" element={<Landing onAuthed={() => setAuthed(true)} />} />
              {/* Signed-out visitors have no business on the app routes. */}
              <Route path="*" element={<Navigate to="/" replace />} />
            </>
          )}
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  );
}

function AuthedShell({ onLogout }: { onLogout: () => void }) {
  return (
    <AppDataProvider>
      <VerifyResultNotice />
      <AppLayout onLogout={onLogout} />
    </AppDataProvider>
  );
}

/**
 * The backend redirects to /?verified=1 or /?verify_error=1 after consuming a
 * verification link. Shown once, then scrubbed from the address bar so a
 * refresh or a shared URL doesn't replay it.
 */
function VerifyResultNotice() {
  const [params, setParams] = useSearchParams();
  const result = params.has("verified") ? "ok" : params.has("verify_error") ? "failed" : null;
  const [visible, setVisible] = useState(result);

  useEffect(() => {
    if (!result) return;
    const next = new URLSearchParams(params);
    next.delete("verified");
    next.delete("verify_error");
    setParams(next, { replace: true });
    // Only on the first render that carried the flag.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!visible) return null;

  return (
    <div className="mx-auto w-full max-w-5xl px-4 pt-4">
      {visible === "ok" ? (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-good-border
                        bg-good-soft px-4 py-3 text-sm text-good">
          <span>✅ Your email is confirmed — alerts are on.</span>
          <button
            type="button"
            onClick={() => setVisible(null)}
            className="shrink-0 font-medium underline-offset-4 hover:underline"
          >
            Dismiss
          </button>
        </div>
      ) : (
        <div className="rounded-lg border border-warn-border bg-warn-soft px-4 py-3 text-sm text-warn">
          That verification link is invalid or has expired. Use “Resend email” below to get a new one.
        </div>
      )}
    </div>
  );
}

function ResetPasswordRoute() {
  const [params] = useSearchParams();
  const token = params.get("token");
  if (!token) return <Navigate to="/" replace />;
  return <ResetPassword token={token} />;
}
