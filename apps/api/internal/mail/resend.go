package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

// ResendSender delivers mail through the Resend HTTP API.
type ResendSender struct {
	APIKey string
	From   string
	// Endpoint and Client can be overridden in tests.
	Endpoint string
	Client   *http.Client
}

func (s ResendSender) Send(ctx context.Context, msg Message) error {
	payload, err := json.Marshal(map[string]any{
		"from":    s.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
		"html":    msg.HTML,
	})
	if err != nil {
		return err
	}

	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = resendEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend responded %d: %s", resp.StatusCode, body)
	}
	return nil
}
