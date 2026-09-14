package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendSender sends email via the Resend API (https://resend.com). Used when
// RESEND_API_KEY is set. The Sender interface makes it a drop-in for LogSender.
type ResendSender struct {
	apiKey   string
	from     string
	endpoint string // overridden in tests; production always uses resendEndpoint
	http     *http.Client
}

const resendEndpoint = "https://api.resend.com/emails"

func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey:   apiKey,
		from:     from,
		endpoint: resendEndpoint,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (*ResendSender) Channel() string { return "email" }

func (r *ResendSender) Send(ctx context.Context, msg Message) error {
	payload := map[string]any{
		"from":    r.from,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"html":    msg.HTMLBody,
		"text":    msg.TextBody,
	}
	// List-Unsubscribe et al. Omitted when empty: Resend rejects an empty
	// headers object on some API versions, and transactional mail has none.
	if len(msg.Headers) > 0 {
		payload["headers"] = msg.Headers
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, b)
	}
	return nil
}
