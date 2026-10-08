package mail

import (
	"strings"
	"testing"
	"time"
)

func TestRenderAccountDeletionScheduled(t *testing.T) {
	msg, err := RenderAccountDeletionScheduled(AccountDeletionEmail{
		To: "ana@example.com", Name: "Ana", Locale: "en",
		Date:       time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC),
		Workspaces: []string{"Solo"},
		Link:       "http://web.test/settings/security",
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Your Finance Wingman account will be deleted on October 15, 2026" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	for _, want := range []string{"Hi Ana", "only member of “Solo”", "Export", "cancel", "backups", "http://web.test/settings/security"} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("text without %q:\n%s", want, msg.Text)
		}
	}

	msg, err = RenderAccountDeletionScheduled(AccountDeletionEmail{To: "ana@example.com", Locale: "es", Date: time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Tu cuenta de Finance Wingman se eliminará el 15 de octubre de 2026" || strings.Contains(msg.Text, "único miembro") {
		t.Fatalf("unexpected Spanish email without workspaces to delete: %s\n%s", msg.Subject, msg.Text)
	}
}

func TestRenderAccountDeletedAndBlocked(t *testing.T) {
	msg, err := RenderAccountDeleted(AccountDeletionEmail{To: "ana@example.com", Locale: "es", Workspaces: []string{"Solo", "Viaje"}})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Tu cuenta de Finance Wingman fue eliminada" || !strings.Contains(msg.Text, "“Solo” y “Viaje”") {
		t.Fatalf("unexpected email: %s\n%s", msg.Subject, msg.Text)
	}

	msg, err = RenderAccountDeletionBlocked(AccountDeletionEmail{To: "ana@example.com", Workspaces: []string{"Home"}, Link: "http://web.test/settings/workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Subject, "couldn't delete") || !strings.Contains(msg.Text, "only owner of “Home”") || !strings.Contains(msg.Text, "http://web.test/settings/workspace") {
		t.Fatalf("unexpected email: %s\n%s", msg.Subject, msg.Text)
	}
}
