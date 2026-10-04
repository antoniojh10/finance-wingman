// Package mail sends transactional emails through a pluggable provider.
package mail

import (
	"context"
	"log/slog"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers email messages.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// LogSender writes messages to the log instead of delivering them. Useful in
// development when no SMTP server is available.
type LogSender struct {
	Logger *slog.Logger
}

func (s LogSender) Send(_ context.Context, msg Message) error {
	s.Logger.Info("email (not delivered)", "to", msg.To, "subject", msg.Subject, "text", msg.Text)
	return nil
}
