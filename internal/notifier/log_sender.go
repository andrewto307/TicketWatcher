package notifier

import (
	"context"
	"log"
	"strings"
)

// LogSender "sends" by logging. It's the default when no email provider is
// configured, so the whole pipeline is demoable without external accounts.
type LogSender struct{}

func NewLogSender() *LogSender { return &LogSender{} }

func (*LogSender) Channel() string { return "email" }

// Send logs the message, body included. The body matters: verification and
// password-reset links are only usable if they're visible somewhere, and with no
// email provider configured the log is the only place they can appear.
func (*LogSender) Send(_ context.Context, msg Message) error {
	log.Printf("[notify:email] to=%s subject=%q\n%s", msg.To, msg.Subject, indent(msg.TextBody))
	return nil
}

// indent offsets the body so it reads as part of the log line above it.
func indent(s string) string {
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}
