import { useState, type ReactNode } from "react";
import { Bell, Clock, Search, ShieldCheck } from "lucide-react";
import { Wordmark } from "./Wordmark";
import { useReveal } from "../useReveal";
import eventSearchShot from "../assets/eventSearch.webp";
import watchListShot from "../assets/watchList.webp";
import emailSentShot from "../assets/emailSent.webp";
import { LoginDialog } from "./LoginDialog";
import { Button } from "../ui/Button";
import { ThemeToggle } from "./ThemeToggle";
import { ScrollCue } from "./ScrollCue";

// The logged-out root. Its job is to say what the service does before asking
// anyone to sign up, which the old bare login box did not.

// Each screenshot gets a screen of its own and makes one point. This replaces
// the old thumbnail gallery plus a separate "How it works" list, which told the
// same three-step story twice.
const features = [
  {
    icon: Search,
    eyebrow: "Search",
    title: "Easy to search for your favourite events",
    body: "One search across Ticketmaster — artists, teams, shows. Filter out anything that has already gone on sale, so you only see what is still worth watching.",
    src: eventSearchShot,
    alt: "Search results listing events with their on-sale status, each with Watch and Save buttons",
  },
  {
    icon: Clock,
    eyebrow: "Watch",
    title: "Start watching its status, and that's it",
    body: "No refreshing, no calendar reminders. We check each event around the clock and keep its sale status up to date for you.",
    src: watchListShot,
    alt: "Watch list showing each tracked event and when its sale opens",
  },
  {
    icon: Bell,
    eyebrow: "Get alerted",
    title: "An email the moment it goes on sale",
    body: "Presales included, along with date announcements, cancellations and reschedules. Become an early bird today.",
    src: emailSentShot,
    alt: "An alert email announcing that a presale has opened",
  },
];

// Each landing section owns a full screen, so scrolling moves between whole
// ideas rather than through a continuous column. min-h rather than h: a short
// viewport or long text grows the section instead of clipping it.
const screen = "flex min-h-[calc(100dvh-3.5rem)] flex-col justify-center py-16";

/**
 * Wraps one block so it fades and rises when scrolled to. Staggering siblings
 * by ~60ms is what reads as considered; everything landing at once reads cheap.
 */
function Reveal({
  children,
  delay = 0,
  as: Tag = "div",
  className,
}: {
  children: ReactNode;
  delay?: number;
  as?: "div" | "figure" | "h2";
  className?: string;
}) {
  const { ref, style } = useReveal<HTMLElement>(delay);
  return (
    <Tag ref={ref as never} style={style} className={className}>
      {children}
    </Tag>
  );
}

/** One screen: a single claim, with the screenshot that backs it up. */
function Feature({
  feature: f,
  flip,
}: {
  feature: (typeof features)[number];
  flip: boolean;
}) {
  const text = useReveal<HTMLDivElement>();
  const shot = useReveal<HTMLDivElement>(80);

  return (
    <section aria-label={f.eyebrow} className={`${screen} border-t border-line`}>
      <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
        <div ref={text.ref} style={text.style} className={flip ? "lg:order-2" : undefined}>
          <span className="inline-flex items-center gap-2 text-sm font-medium text-accent">
            <f.icon aria-hidden className="size-4" />
            {f.eyebrow}
          </span>
          <h2 className="mt-3 text-3xl font-semibold tracking-tight text-balance text-ink sm:text-4xl">
            {f.title}
          </h2>
          <p className="mt-4 max-w-md text-lg text-muted">{f.body}</p>
        </div>

        <div ref={shot.ref} style={shot.style} className={flip ? "lg:order-1" : undefined}>
          <img
            src={f.src}
            alt={f.alt}
            width={760}
            height={475}
            loading="lazy"
            decoding="async"
            className="aspect-[16/10] w-full rounded-xl border border-line object-cover shadow-lg"
          />
        </div>
      </div>
    </section>
  );
}

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
        {/* The message and the two buttons are the whole first impression. */}
        <section className={`${screen} text-center`}>
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
          <ScrollCue />
        </section>

        {features.map((f, i) => (
          <Feature key={f.eyebrow} feature={f} flip={i % 2 === 1} />
        ))}

      </main>

      <footer className="border-t border-line">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-center gap-x-5 gap-y-2 px-4 py-6">
          <a href="/privacy" className="text-sm text-muted underline-offset-4 hover:text-ink hover:underline">
            Privacy policy
          </a>
          <a href="mailto:ticketmasterwatcher@toandrew.com" className="text-sm text-muted underline-offset-4 hover:text-ink hover:underline">
            Contact
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
