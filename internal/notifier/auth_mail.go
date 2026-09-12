package notifier

import "fmt"

// Transactional account emails. They live here so every message the app sends —
// alerts and account mail alike — is rendered in one place.

// VerificationMessage asks a new user to confirm they own the address.
func VerificationMessage(to, link string) Message {
	text := fmt.Sprintf(
		"Confirm your email to start receiving ticket alerts:\n\n%s\n\n"+
			"This link expires in 24 hours. If you didn't create this account, ignore this email.",
		link)
	html := fmt.Sprintf(
		`<h2>Confirm your email</h2>`+
			`<p>Confirm your address to start receiving ticket alerts.</p>`+
			`<p><a href="%s">Verify my email →</a></p>`+
			`<p style="color:#666;font-size:13px">This link expires in 24 hours. `+
			`If you didn't create this account, you can ignore this email.</p>`,
		link)
	return Message{To: to, Subject: "Confirm your email — Ticket Availability Watcher", HTMLBody: html, TextBody: text}
}

// PasswordResetMessage carries a one-time link to choose a new password.
func PasswordResetMessage(to, link string) Message {
	text := fmt.Sprintf(
		"Reset your Ticket Availability Watcher password:\n\n%s\n\n"+
			"This link expires in 1 hour and can be used once. "+
			"If you didn't request a reset, ignore this email — your password is unchanged.",
		link)
	html := fmt.Sprintf(
		`<h2>Reset your password</h2>`+
			`<p><a href="%s">Choose a new password →</a></p>`+
			`<p style="color:#666;font-size:13px">This link expires in 1 hour and can be used once. `+
			`If you didn't request a reset, ignore this email — your password is unchanged.</p>`,
		link)
	return Message{To: to, Subject: "Reset your password — Ticket Availability Watcher", HTMLBody: html, TextBody: text}
}
