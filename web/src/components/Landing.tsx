import { useState } from "react";
import { Bell, Clock, Search, ShieldCheck } from "lucide-react";
import { Wordmark } from "./Wordmark";
import { LoginDialog } from "./LoginDialog";
import { Button } from "../ui/Button";
import { ThemeToggle } from "./ThemeToggle";

// The logged-out root. Its job is to say what the service does before asking
// anyone to sign up, which the old bare login box did not.

const steps = [
  { icon: Search, title: "Find the event", body: "Search Ticketmaster for the artist, team, or show you're after." },
  { icon: Clock, title: "We watch it for you", body: "We check the on-sale status continuously so you don't have to keep refreshing." },
  { icon: Bell, title: "You get an email", body: "Presale opening, public sale, a date announcement, a cancellation — the moment it happens." },
];

// Placeholders until real screenshots are exported; the layout is the point.
const shots = [
  { label: "Searching for an event", hint: "search results with on-sale status" },
  { label: "Your watch list", hint: "cards showing each event's sale state" },
  { label: "The alert email", hint: "what lands in your inbox" },
];

export function Landing({ onAuthed }: { onAuthed: () => void }) {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<"login" | "register">("login");

  function openAs(m: "login" | "register") {
    setMode(m);
    setOpen(true);
  }

  return (
    <div className="min-h-dvh bg-canvas">
      <header className="border-b border-line">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
          <Wordmark />
          <div className="flex items-center gap-3">
            <div className="w-28">
              <ThemeToggle />
            </div>
            <Button variant="primary" onClick={() => openAs("login")}>
              Log in
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4">
        <section className="py-16 text-center sm:py-24">
          <h1 className="mx-auto max-w-3xl text-4xl font-semibold tracking-tight text-balance text-ink sm:text-5xl">
            Want to get notified when your favourite events go on sale?
          </h1>
          <p className="mx-auto mt-5 max-w-xl text-lg text-muted">
            TicketWatcher follows the events you care about and emails you the moment tickets become
            available — presales included.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Button variant="primary" size="lg" onClick={() => openAs("register")}>
              Get started
            </Button>
            <Button size="lg" onClick={() => openAs("login")}>
              Log in
            </Button>
          </div>
          <p className="mt-4 flex items-center justify-center gap-1.5 text-sm text-muted">
            <ShieldCheck aria-hidden className="size-4" />
            No tracking, no ads. Just an email address.
          </p>
        </section>

        <section aria-label="Screenshots" className="pb-16">
          <div className="grid gap-4 sm:grid-cols-3">
            {shots.map((s) => (
              <figure key={s.label} className="flex flex-col gap-2">
                <div
                  className="grid aspect-[4/3] place-items-center rounded-xl border border-line
                             bg-surface-2 px-4 text-center text-xs text-muted"
                >
                  {s.hint}
                </div>
                <figcaption className="text-sm text-muted">{s.label}</figcaption>
              </figure>
            ))}
          </div>
        </section>

        <section className="border-t border-line py-16">
          <h2 className="text-center text-2xl font-semibold tracking-tight text-ink">How it works</h2>
          <div className="mt-10 grid gap-8 sm:grid-cols-3">
            {steps.map((s) => (
              <div key={s.title}>
                <span aria-hidden className="grid size-9 place-items-center rounded-lg bg-accent-soft text-accent">
                  <s.icon className="size-5" />
                </span>
                <h3 className="mt-3 font-semibold text-ink">{s.title}</h3>
                <p className="mt-1 text-sm text-muted">{s.body}</p>
              </div>
            ))}
          </div>
        </section>
      </main>

      <footer className="border-t border-line">
        <div className="mx-auto flex max-w-5xl justify-center px-4 py-6">
          <a href="/privacy" className="text-sm text-muted underline-offset-4 hover:text-ink hover:underline">
            Privacy policy
          </a>
        </div>
      </footer>

      <LoginDialog
        key={mode}
        open={open}
        initialMode={mode}
        onClose={() => setOpen(false)}
        onAuthed={onAuthed}
      />
    </div>
  );
}
