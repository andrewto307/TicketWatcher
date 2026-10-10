import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

// Watch / Save / Pause all move a card out of the list the user is looking at.
// Without an acknowledgement those actions read as "the thing vanished", so
// every one of them raises a toast.

interface Toast {
  id: number;
  message: string;
  action?: { label: string; onClick: () => void };
}

interface ToastApi {
  toast: (message: string, action?: Toast["action"]) => void;
}

const Ctx = createContext<ToastApi>({ toast: () => {} });

export function useToast(): ToastApi {
  return useContext(Ctx);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);

  const toast = useCallback((message: string, action?: Toast["action"]) => {
    const id = Date.now() + Math.random();
    setItems((cur) => [...cur, { id, message, action }]);
    setTimeout(() => setItems((cur) => cur.filter((t) => t.id !== id)), 4000);
  }, []);

  const api = useMemo(() => ({ toast }), [toast]);

  return (
    <Ctx.Provider value={api}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4"
      >
        {items.map((t) => (
          <div
            key={t.id}
            role="status"
            className="animate-fade-up pointer-events-auto flex max-w-[min(26rem,calc(100vw-2rem))]
                       items-center gap-3 rounded-lg border border-line bg-surface px-4 py-2.5
                       text-sm shadow-lg"
          >
            <span className="text-ink">{t.message}</span>
            {t.action && (
              <button
                type="button"
                onClick={() => {
                  t.action!.onClick();
                  setItems((cur) => cur.filter((x) => x.id !== t.id));
                }}
                className="shrink-0 font-medium text-accent underline-offset-2 hover:underline"
              >
                {t.action.label}
              </button>
            )}
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}
