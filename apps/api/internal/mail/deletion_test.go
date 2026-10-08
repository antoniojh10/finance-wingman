package mail

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	day := time.Date(2026, time.October, 15, 10, 0, 0, 0, time.UTC)
	if got := formatDate(day, "en"); got != "October 15, 2026" {
		t.Fatalf("en: %q", got)
	}
	if got := formatDate(day, "es"); got != "15 de octubre de 2026" {
		t.Fatalf("es: %q", got)
	}
}

func TestQuoteList(t *testing.T) {
	cases := map[string][]string{
		"":                        nil,
		"“Home”":                  {"Home"},
		"“Home”, “Trip” and “Ok”": {"Home", "Trip", "Ok"},
	}
	for want, names := range cases {
		if got := quoteList(names, "en"); got != want {
			t.Fatalf("%v: got %q, want %q", names, got, want)
		}
	}
	if got := quoteList([]string{"A", "B"}, "es"); got != "“A” y “B”" {
		t.Fatalf("es: %q", got)
	}
}

func TestRenderWorkspaceDeletionScheduled(t *testing.T) {
	msg, err := RenderWorkspaceDeletionScheduled(WorkspaceDeletionEmail{
		To: "bob@example.com", Name: "Bob", Locale: "es", WorkspaceName: "<Casa>", RequestedBy: "Ana",
		Date: time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC), Link: "http://web.test/settings/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.To != "bob@example.com" || msg.Subject != "El espacio <Casa> se eliminará el 15 de octubre de 2026" {
		t.Fatalf("unexpected email: %+v", msg)
	}
	for _, want := range []string{"Hola Bob", "Ana programó", "copias de seguridad", "http://web.test/settings/workspace"} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("text without %q:\n%s", want, msg.Text)
		}
	}
	if strings.Contains(msg.HTML, "<Casa>") || !strings.Contains(msg.HTML, "&lt;Casa&gt;") {
		t.Fatalf("the workspace name should be escaped in HTML:\n%s", msg.HTML)
	}
}

func TestRenderWorkspaceDeleted(t *testing.T) {
	msg, err := RenderWorkspaceDeleted(WorkspaceDeletionEmail{To: "ana@example.com", Locale: "fr", WorkspaceName: "Home"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "The workspace Home was deleted" || !strings.Contains(msg.Text, "Hi,") || !strings.Contains(msg.Text, "backups") {
		t.Fatalf("unknown locales fall back to English: %+v", msg)
	}
}
