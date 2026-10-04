package testutil

import (
	"context"
	"regexp"
	"sync"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
)

// MailRecorder is a mail.Sender that keeps messages in memory.
type MailRecorder struct {
	mu       sync.Mutex
	messages []mail.Message
}

func (r *MailRecorder) Send(_ context.Context, msg mail.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, msg)
	return nil
}

func (r *MailRecorder) Messages() []mail.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]mail.Message(nil), r.messages...)
}

var (
	tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
	codePattern  = regexp.MustCompile(`\b(\d{6})\b`)
)

// LastLogin returns the token and code from the most recent sign-in email.
func (r *MailRecorder) LastLogin(t *testing.T) (token, code string) {
	t.Helper()
	msgs := r.Messages()
	if len(msgs) == 0 {
		t.Fatal("no email was sent")
	}
	text := msgs[len(msgs)-1].Text
	tm := tokenPattern.FindStringSubmatch(text)
	cm := codePattern.FindStringSubmatch(text)
	if tm == nil || cm == nil {
		t.Fatalf("sign-in email without token or code:\n%s", text)
	}
	return tm[1], cm[1]
}
