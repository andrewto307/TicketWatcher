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
import { BrowserFrame } from "./BrowserFrame";
import { HeroCards } from "./HeroCards";

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
    frameLabel: "ticketwatcher.fly.dev",
    src: eventSearchShot,
    alt: "Search results listing events with their on-sale status, each with Watch and Save buttons",
  },
  {
    icon: Clock,
    eyebrow: "Watch",
    title: "Start watching its status, and that's it",
    body: "No refreshing, no calendar reminders. We check each event around the clock and keep its sale status up to date for you.",
    frameLabel: "ticketwatcher.fly.dev/watches",
    src: watchListShot,
    alt: "Watch list showing each tracked event and when its sale opens",
  },
  {
    icon: Bell,
    eyebrow: "Get alerted",
    title: "An email the moment it goes on sale",
    body: "Presales included, along with date announcements, cancellations and reschedules. Become an early bird today.",
    frameLabel: "mail.google.com",
    src: emailSentShot,
    alt: "An alert email announcing that a presale has opened",
  },
];

// Each landing section owns a full screen, so scrolling moves between whole
// ideas rather than through a continuous column. min-h rather than h: a short
// viewport or long text grows the section instead of clipping it.
const screen = "flex min-h-[calc(100dvh-3.5rem)] flex-col justify-center py-10 sm:py-16";

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
const washes = ["wash-1", "wash-2", "wash-3"] as const;

function Feature({
  feature: f,
  flip,
  index,
}: {
  feature: (typeof features)[number];
  flip: boolean;
  index: number;
}) {
  const text = useReveal<HTMLDivElement>();
  const shot = useReveal<HTMLDivElement>(80, flip ? 6 : -6);

  return (
    <section
      aria-label={f.eyebrow}
      className={`${screen} relative
        before:absolute before:inset-y-0 before:left-1/2 before:-z-10 before:w-screen
        before:-translate-x-1/2 ${flip ? "before:bg-surface-2" : "before:border-t before:border-line"}`}
    >
      {/* The wash sits above the band and below the content, full-bleed so the
          colour runs edge to edge rather than stopping at the container. */}
      <div
        aria-hidden
        className={`pointer-events-none absolute inset-y-0 left-1/2 w-screen
                    -translate-x-1/2 ${washes[index % washes.length]}`}
      />
      <div className="relative grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
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

        {/* perspective on the wrapper, transform on the child: the tilt needs a
            3D context to read as depth rather than a squash. */}
        <div className={`[perspective:1400px] ${flip ? "lg:order-1" : ""}`}>
          <div ref={shot.ref} style={shot.style} className="shot-lit">
          <BrowserFrame label={f.frameLabel}>
            <img
              src={f.src}
              alt={f.alt}
              width={760}
              height={475}
              loading="lazy"
              decoding="async"
              className="aspect-[16/10] w-full object-cover"
            />
          </BrowserFrame>
          </div>
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
    <div className="relative min-h-dvh overflow-x-clip bg-canvas">
      <div aria-hidden className="grain" />
      <header className="relative z-10 border-b border-line">
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
        {/* The message and the two buttons are the whole first impression.
            Text left, product right: centred everything is safe but inert. */}
        <section className={`${screen} relative`}>
          <div aria-hidden className="hero-grid" />
          <div aria-hidden className="hero-glow" />
          <div className="relative grid items-center gap-12 text-center lg:grid-cols-[1.1fr_1fr] lg:gap-16 lg:text-left">
          <div>
          <h1 className="max-w-3xl text-4xl font-semibold tracking-tight text-balance text-ink sm:text-5xl">
            Want to get notified when your favourite events go on sale?
          </h1>
          <p className="mt-6 max-w-xl text-lg text-muted lg:mx-0">
            TicketWatcher follows the events you care about and emails you the moment tickets become
            available — presales included.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3 lg:justify-start">
            <Button variant="primary" size="lg" onClick={() => openAs("register")}>
              Get started
            </Button>
            <Button size="lg" onClick={() => openAs("login")}>
              Log in
            </Button>
          </div>
          <p className="mt-4 flex items-center justify-center gap-1.5 text-sm text-muted lg:justify-start">
            <ShieldCheck aria-hidden className="size-4" />
            No tracking, no ads. Just an email address.
          </p>
          </div>

          <div className="mx-auto hidden w-full max-w-sm lg:block lg:max-w-none">
            <HeroCards />
          </div>
          </div>
          <ScrollCue />
        </section>

        {features.map((f, i) => (
          <Feature key={f.eyebrow} feature={f} flip={i % 2 === 1} index={i} />
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
