package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRenderLogin(t *testing.T) {
	base := LoginEmail{To: "ana@example.com", Name: "Ana", Link: "https://app.test/auth/verify?token=abc", Code: "123456", TTLMinutes: 15}

	tests := map[string]string{"en": "Your Finance Wingman sign-in link", "es": "Tu enlace para entrar a Finance Wingman", "fr": "Your Finance Wingman sign-in link"}
	for locale, subject := range tests {
		t.Run(locale, func(t *testing.T) {
			e := base
			e.Locale = locale
			msg, err := RenderLogin(e)
			if err != nil {
				t.Fatal(err)
			}
			if msg.To != e.To || msg.Subject != subject {
				t.Fatalf("unexpected headers: %+v", msg)
			}
			for _, body := range []string{msg.Text, msg.HTML} {
				if !strings.Contains(body, "123456") || !strings.Contains(body, "Ana") || !strings.Contains(body, "15") {
					t.Fatalf("body is missing code, name or ttl:\n%s", body)
				}
			}
			if !strings.Contains(msg.Text, e.Link) || !strings.Contains(msg.HTML, `href="https://app.test/auth/verify?token=abc"`) {
				t.Fatal("body is missing the link")
			}
		})
	}
}

func TestRenderConnectCode(t *testing.T) {
	msg, err := RenderLogin(LoginEmail{To: "a@b.c", Locale: "es", Code: "424242", TTLMinutes: 15, ClientName: "Claude"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Tu código para conectar Claude con Finance Wingman" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	if strings.Contains(msg.HTML, "href=") || strings.Contains(msg.Text, "http") {
		t.Fatal("connect emails must not include a sign-in link")
	}
	if !strings.Contains(msg.Text, "424242") || !strings.Contains(msg.Text, "Claude está solicitando acceso") || !strings.Contains(msg.Text, "El código expira en 15 minutos") {
		t.Fatalf("unexpected body:\n%s", msg.Text)
	}
}

func TestRenderLoginEscapesHTML(t *testing.T) {
	msg, err := RenderLogin(LoginEmail{Name: "<script>", Link: "https://x.test", Code: "1", TTLMinutes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Fatal("name must be HTML-escaped")
	}
}

func TestResendSender(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}))
	defer server.Close()

	sender := ResendSender{APIKey: "re_test", From: "Wingman <no-reply@test.dev>", Endpoint: server.URL}
	if err := sender.Send(context.Background(), Message{To: "ana@example.com", Subject: "Hi", Text: "Body"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test" || got.From != sender.From || got.To[0] != "ana@example.com" || got.Subject != "Hi" || got.Text != "Body" {
		t.Fatalf("unexpected request: auth=%q body=%+v", auth, got)
	}
}

func TestResendSenderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"invalid from"}`)
	}))
	defer server.Close()

	err := ResendSender{APIKey: "k", From: "x", Endpoint: server.URL}.Send(context.Background(), Message{To: "a@b.c"})
	if err == nil || !strings.Contains(err.Error(), "invalid from") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestLogSender(t *testing.T) {
	var buf strings.Builder
	sender := LogSender{Logger: slog.New(slog.NewTextHandler(&buf, nil))}
	if err := sender.Send(context.Background(), Message{To: "a@b.c", Subject: "S", Text: "code 42"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "code 42") {
		t.Fatalf("expected message in log, got %s", buf.String())
	}
}

func TestEnvelopeAddress(t *testing.T) {
	if got := envelopeAddress("Wingman <no-reply@test.dev>"); got != "no-reply@test.dev" {
		t.Fatalf("got %q", got)
	}
	if got := envelopeAddress("plain@test.dev"); got != "plain@test.dev" {
		t.Fatalf("got %q", got)
	}
}

// TestSMTPSenderWithMailpit delivers a real message to the docker-compose
// Mailpit instance and reads it back through Mailpit's API.
func TestSMTPSenderWithMailpit(t *testing.T) {
	apiURL := envOr("TEST_MAILPIT_URL", "http://localhost:8025")
	recipient := fmt.Sprintf("smtp-test-%d@example.com", time.Now().UnixNano())

	sender := SMTPSender{Host: envOr("TEST_SMTP_HOST", "localhost"), Port: 1025, From: "Finance Wingman <no-reply@test.dev>"}
	err := sender.Send(context.Background(), Message{To: recipient, Subject: "Código de acceso", Text: "Tu código: 654321", HTML: "<p>654321</p>"})
	if err != nil {
		t.Fatalf("send (is `make up` running?): %v", err)
	}

	resp, err := http.Get(apiURL + "/api/v1/search?query=" + url.QueryEscape("to:"+recipient))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Messages []struct {
			Subject string `json:"Subject"`
			Snippet string `json:"Snippet"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message in Mailpit, got %d", len(result.Messages))
	}
	if m := result.Messages[0]; m.Subject != "Código de acceso" || !strings.Contains(m.Snippet, "654321") {
		t.Fatalf("unexpected message: %+v", m)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
