import { Link } from "react-router-dom";
import { Wordmark } from "./Wordmark";

// Deliberately plain and specific: a privacy policy that describes what this
// app actually does is more useful, and more honest, than boilerplate copied
// from a generator.
export function Privacy() {
  return (
    <div className="min-h-dvh bg-canvas">
      <header className="border-b border-line">
        <div className="mx-auto flex h-14 max-w-3xl items-center justify-between px-4">
          <Link to="/" aria-label="TicketWatcher home" className="rounded-lg">
            <Wordmark />
          </Link>
          <Link
            to="/"
            className="inline-flex h-9 items-center rounded-lg border border-line bg-surface px-3
                       text-sm font-medium text-ink transition-colors hover:bg-surface-2"
          >
            Back to app
          </Link>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 py-10">
        <h1 className="text-3xl font-semibold tracking-tight text-ink">Privacy Policy</h1>
        <p className="mt-1 text-muted">TicketWatcher</p>
      <section
          className="mt-8 flex flex-col gap-4 text-ink
                     [&_h2]:mt-6 [&_h2]:text-base [&_h2]:font-semibold
                     [&_li]:text-muted [&_li]:leading-relaxed
                     [&_li_strong]:text-ink
                     [&_p]:text-muted [&_p]:leading-relaxed
                     [&_p_strong]:text-ink
                     [&_ul]:flex [&_ul]:list-disc [&_ul]:flex-col [&_ul]:gap-2 [&_ul]:pl-5"
        >
        <h2>What we collect</h2>
        <ul>
          <li>
            <strong>Your email address</strong> — so you can sign in and so we can send the alerts
            you asked for.
          </li>
          <li>
            <strong>Your password</strong> — stored only as a bcrypt hash. We cannot read it or
            recover it.
          </li>
          <li>
            <strong>The events you watch</strong>, plus the on-sale status history we observe for
            those events.
          </li>
        </ul>
        <p>
          That's all. No analytics, no advertising trackers, no third-party scripts, no cookies
          beyond the login token stored in your browser.
        </p>

        <h2>Why we collect it</h2>
        <p>
          Solely to run the service: to authenticate you, to poll the events you chose, and to email
          you when one of them goes on sale. We do not sell or share your data.
        </p>

        <h2>Who else is involved</h2>
        <ul>
          <li>
            <strong>Ticketmaster</strong> — we query their public Discovery API for event and
            on-sale data. We send them event searches, never your identity.
          </li>
          <li>
            <strong>Resend</strong> — delivers our email. They process your address in order to
            deliver messages to it.
          </li>
          <li>
            <strong>Fly.io</strong> — hosts the application and its database.
          </li>
        </ul>

        <h2>How long we keep it</h2>
        <p>
          For as long as your account exists. Delete your account and everything above is removed
          immediately — your watches, status history, and notification records are erased along with
          it. There is no soft-delete and no backup copy retained for marketing.
        </p>

        <h2>Your choices</h2>
        <ul>
          <li>
            <strong>Stop the email</strong> — every alert has an unsubscribe link, or use the toggle
            on your dashboard. Unsubscribing keeps your account and watches intact.
          </li>
          <li>
            <strong>Delete everything</strong> — "Delete account" on your dashboard. It is immediate
            and irreversible.
          </li>
        </ul>

        <h2>A note on accuracy</h2>
        <p>
          On-sale status comes from Ticketmaster and is not guaranteed to be complete or current.
          Note that <strong>“on sale” means the sale window is open, not that seats are in
          stock</strong> — a sold-out event still reports as on sale, because Ticketmaster's public
          API does not expose real inventory. Alerts are best-effort: this service polls within a
          limited API budget and should not be relied on for time-critical purchases.
        </p>

        <h2>Contact</h2>
        <p>
          Questions or feedback:{" "}
          <a
            href="mailto:ticketmasterwatcher@toandrew.com"
            className="text-accent underline-offset-4 hover:underline"
          >
            ticketmasterwatcher@toandrew.com
          </a>
        </p>
        </section>
      </main>
    </div>
  );
}
