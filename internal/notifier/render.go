package notifier

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// Message copy for each sale milestone. Every message does three things: name the
// state plainly, say what the user can do next, and link to the event.
//
// Emphasis is written as **markdown-style bold** and converted per format. Event
// names come from the Ticketmaster API, so the HTML path escapes them before any
// markup is inserted — an event named `Foo & <Bar>` must not break the email.
//
// The resale disclaimer is not decoration. Ticketmaster's public API exposes no
// resale inventory (see plan/08-sale-milestone-alerts.md), so "the official sale
// hasn't opened" is NOT the same as "there are no tickets". Saying so is the
// difference between an honest product and one that quietly misleads.
const resaleNote = "Ticketmaster's public data doesn't include resale listings, so tickets may be " +
	"available on the event page even when the official sale hasn't opened or has closed."

var boldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)

// plain strips emphasis markers for the text/plain part and for subjects.
func plain(s string) string { return boldRe.ReplaceAllString(s, "$1") }

// htmlLine escapes first, then applies emphasis, so API-supplied text can't
// inject markup.
func htmlLine(s string) string {
	return boldRe.ReplaceAllString(html.EscapeString(s), "<strong>$1</strong>")
}

// saleTime formats a sale timestamp for humans. UTC is explicit rather than
// guessed: we don't reliably know the reader's timezone, and a wrong local time
// on a ticket drop is worse than an unambiguous UTC one.
func saleTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("Mon 2 Jan 2006 at 15:04 UTC")
}

// renderMilestone builds the subject and body lines for one alert kind.
// ok=false means "no copy for this kind" so the caller can skip sending rather
// than mail an empty message.
func renderMilestone(a Alert) (subject string, lines []string, showResaleNote bool, ok bool) {
	name := a.EventName

	switch a.Kind {
	case "onsale_announced":
		subject = fmt.Sprintf("📅 Onsale date announced: %s", name)
		lines = []string{
			fmt.Sprintf("Ticketmaster has announced when **%s** goes on sale.", name),
			fmt.Sprintf("Public sale opens: **%s**", saleTime(a.PublicOnsaleStart)),
			"We'll email you again the moment it opens, so you don't need to keep checking.",
		}
		if a.EarliestPresaleStart != nil {
			lines = append(lines, fmt.Sprintf(
				"There's also a presale starting %s — we'll alert you for that too.",
				saleTime(a.EarliestPresaleStart)))
		}
		return subject, lines, true, true

	case "presale_open":
		subject = fmt.Sprintf("🔑 Presale open now: %s", name)
		// Names come straight from Ticketmaster and are wildly inconsistent:
		// "Artist Presale", "Amex Presale Tickets®", or just "Presale". Appending
		// the word unconditionally produces "The Presale presale".
		what := "A presale"
		if n := a.EarliestPresaleName; n != "" {
			if strings.Contains(strings.ToLower(n), "presale") {
				what = "The " + n
			} else {
				what = fmt.Sprintf("The %s presale", n)
			}
		}
		lines = []string{
			fmt.Sprintf("%s for **%s** is open right now — ahead of the general public.", what, name),
			"Presales often need a code, a fan-club membership, or a specific credit card. " +
				"If you don't have access, you can still wait for the public sale.",
		}
		switch {
		case a.PublicOnsaleStart != nil:
			lines = append(lines, fmt.Sprintf("General public sale opens **%s**.", saleTime(a.PublicOnsaleStart)))
		case a.OnsaleTBD:
			lines = append(lines, "The general public sale date hasn't been announced yet.")
		}
		return subject, lines, true, true

	case "public_open", "status_onsale":
		subject = fmt.Sprintf("🎟️ On sale now: %s", name)
		lines = []string{
			fmt.Sprintf("Tickets for **%s** are on sale to the general public right now.", name),
			"Popular events can sell out quickly, so it's worth going straight to the event page.",
		}
		return subject, lines, false, true

	case "sale_closed":
		subject = fmt.Sprintf("⏰ Official sale closed: %s", name)
		lines = []string{
			fmt.Sprintf("The official Ticketmaster sale for **%s** has closed.", name),
			"**Resale tickets may still be available.** Open the event page below to check for " +
				"Verified Resale listings from other fans — those are sold separately from the " +
				"official sale and often stay available right up to the event.",
			"We can't see resale listings, so this is the one case worth checking manually.",
		}
		// The body already explains resale in full; the generic note would repeat it.
		return subject, lines, false, true

	case "cancelled":
		subject = fmt.Sprintf("❌ Cancelled: %s", name)
		lines = []string{
			fmt.Sprintf("**%s** has been cancelled by the organiser.", name),
			"If you already bought tickets, refunds are handled by Ticketmaster — check the event " +
				"page or your Ticketmaster account for details.",
			"We've stopped watching this event, since nothing further will happen.",
		}
		return subject, lines, false, true

	case "rescheduled":
		subject = fmt.Sprintf("🔄 Date changed: %s", name)
		lines = []string{
			fmt.Sprintf("**%s** has been postponed or rescheduled.", name),
			"Existing tickets are usually still valid for the new date, but check the event page to " +
				"confirm — and to see the new date if it's been announced.",
			"Your watch is still active, so we'll keep watching it for you.",
		}
		return subject, lines, false, true
	}

	return "", nil, false, false
}

// buildBodies assembles the text and HTML bodies from rendered lines.
func buildBodies(a Alert, lines []string, showResaleNote bool) (text, htmlBody string) {
	venue := ""
	if a.Venue != "" {
		venue = " @ " + a.Venue
	}

	var tb, hb strings.Builder
	tb.WriteString(plain(a.EventName) + venue + "\n\n")
	hb.WriteString("<h2>" + htmlLine(a.EventName+venue) + "</h2>")

	for _, l := range lines {
		tb.WriteString(plain(l) + "\n\n")
		hb.WriteString("<p>" + htmlLine(l) + "</p>")
	}

	if showResaleNote {
		tb.WriteString(resaleNote + "\n\n")
		hb.WriteString(`<p style="color:#666;font-size:13px">` + htmlLine(resaleNote) + "</p>")
	}

	if a.EventURL != "" {
		label := ctaLabel(a.Kind)
		tb.WriteString(label + ": " + a.EventURL + "\n")
		hb.WriteString(`<p><a href="` + html.EscapeString(a.EventURL) + `">` + html.EscapeString(label) + ` →</a></p>`)
	}
	return tb.String(), hb.String()
}

// ctaLabel makes the link text match what the user should do, rather than a
// generic "view event" on every message.
func ctaLabel(kind string) string {
	switch kind {
	case "presale_open":
		return "Open the presale on Ticketmaster"
	case "public_open", "status_onsale":
		return "Buy tickets on Ticketmaster"
	case "sale_closed":
		return "Check for resale tickets on Ticketmaster"
	case "cancelled", "rescheduled":
		return "See the event page on Ticketmaster"
	default:
		return "View on Ticketmaster"
	}
}
