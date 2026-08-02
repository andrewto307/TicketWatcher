package notifier

import (
	"context"
	"log"
)

// LogSender "sends" by logging. It's the default when no email provider is
// configured, so the whole pipeline is demoable without external accounts.
type LogSender struct{}

func NewLogSender() *LogSender { return &LogSender{} }

func (*LogSender) Channel() string { return "email" }

func (*LogSender) Send(_ context.Context, msg Message) error {
	log.Printf("[notify:email] to=%s subject=%q", msg.To, msg.Subject)
	return nil
}
