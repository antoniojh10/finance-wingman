package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPSender delivers mail through an SMTP server (e.g. Mailpit locally).
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (s SMTPSender) Send(ctx context.Context, msg Message) error {
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, envelopeAddress(s.From), []string{msg.To}, buildMIME(s.From, msg))
	}()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// envelopeAddress extracts the bare address from "Name <addr>".
func envelopeAddress(from string) string {
	if start := strings.LastIndex(from, "<"); start >= 0 {
		if end := strings.LastIndex(from, ">"); end > start {
			return from[start+1 : end]
		}
	}
	return from
}

func buildMIME(from string, msg Message) []byte {
	boundary := randomBoundary()
	var b strings.Builder
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from)
	header("To", msg.To)
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("MIME-Version", "1.0")
	header("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")
	for _, part := range []struct{ contentType, body string }{
		{"text/plain", msg.Text},
		{"text/html", msg.HTML},
	} {
		if part.body == "" {
			continue
		}
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		fmt.Fprintf(&b, "Content-Type: %s; charset=utf-8\r\n", part.contentType)
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(strings.ReplaceAll(part.body, "\n", "\r\n"))
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func randomBoundary() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
