package httpapi

// Standalone pages served outside the SPA.
//
// The unsubscribe link is opened from an email client by someone who may not be
// logged in — and may not even remember signing up. Redirecting them into the app
// would show a login screen and leave them unsure whether the opt-out actually
// worked. A self-contained page answers that immediately, with no JS and no
// session required.

const unsubscribedPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Unsubscribed — TicketWatcher</title>
<style>
  body{font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;background:#0f1115;color:#e6e8eb;
       display:grid;place-items:center;min-height:100vh;margin:0;padding:20px}
  .card{max-width:440px;background:#171a21;border:1px solid #262b36;border-radius:14px;padding:32px;text-align:center}
  h1{font-size:1.25rem;margin:0 0 12px}
  p{color:#9aa3ad;margin:0 0 8px}
  a{color:#6ea8fe}
</style></head>
<body><div class="card">
  <h1>🎟️ You've been unsubscribed</h1>
  <p>You won't receive any more ticket alerts from TicketWatcher.</p>
  <p>Your watches are still saved — you can turn alerts back on any time from your dashboard.</p>
  <p style="margin-top:20px"><a href="/">Back to TicketWatcher</a></p>
</div></body></html>`

const unsubscribeFailedPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Unsubscribe failed — TicketWatcher</title>
<style>
  body{font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;background:#0f1115;color:#e6e8eb;
       display:grid;place-items:center;min-height:100vh;margin:0;padding:20px}
  .card{max-width:440px;background:#171a21;border:1px solid #262b36;border-radius:14px;padding:32px;text-align:center}
  h1{font-size:1.25rem;margin:0 0 12px}
  p{color:#9aa3ad;margin:0 0 8px}
  a{color:#6ea8fe}
</style></head>
<body><div class="card">
  <h1>That unsubscribe link didn't work</h1>
  <p>It may have been altered in transit or copied incompletely.</p>
  <p>You can turn alerts off from your dashboard, or reply to the email and we'll handle it.</p>
  <p style="margin-top:20px"><a href="/">Back to TicketWatcher</a></p>
</div></body></html>`
