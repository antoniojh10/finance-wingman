package mail

import (
	"strings"
	"testing"
	"time"
)

func TestRenderAppConnected(t *testing.T) {
	date := time.Date(2026, time.October, 8, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		in       AppConnectedEmail
		subject  string
		contains []string
	}{
		{
			name:     "english read only",
			in:       AppConnectedEmail{To: "ana@example.com", Name: "Ana", Locale: "en", AppName: "Claude", WorkspaceName: "Home", Date: date, Link: "http://web.test/settings/security"},
			subject:  "Claude was connected to your Finance Wingman account",
			contains: []string{"Hi Ana", "Access: Read only.", "Workspace: Home.", "Date: October 8, 2026.", "Settings → Security", "http://web.test/settings/security"},
		},
		{
			name:     "english read and write",
			in:       AppConnectedEmail{To: "ana@example.com", Locale: "en", AppName: "Claude", CanWrite: true, Date: date},
			subject:  "Claude was connected to your Finance Wingman account",
			contains: []string{"Access: Read and write."},
		},
		{
			name:     "spanish read only",
			in:       AppConnectedEmail{To: "ana@example.com", Locale: "es", AppName: "Claude", WorkspaceName: "Casa", Date: date, Link: "http://web.test/settings/security"},
			subject:  "Claude se conectó a tu cuenta de Finance Wingman",
			contains: []string{"Hola", "Acceso: Solo lectura.", "Espacio: Casa.", "Fecha: 8 de octubre de 2026.", "Configuración → Seguridad"},
		},
		{
			name:     "spanish read and write, unnamed app",
			in:       AppConnectedEmail{To: "ana@example.com", Locale: "es", CanWrite: true, Date: date},
			subject:  "Una aplicación sin nombre se conectó a tu cuenta de Finance Wingman",
			contains: []string{"Acceso: Lectura y escritura."},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := RenderAppConnected(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if msg.To != tc.in.To || msg.Subject != tc.subject {
				t.Fatalf("unexpected message header: %q %q", msg.To, msg.Subject)
			}
			for _, want := range tc.contains {
				if !strings.Contains(msg.Text, want) {
					t.Fatalf("text without %q:\n%s", want, msg.Text)
				}
			}
			if tc.in.Link != "" && !strings.Contains(msg.HTML, tc.in.Link) {
				t.Fatalf("html without the link:\n%s", msg.HTML)
			}
		})
	}
}

func TestRenderMemberJoined(t *testing.T) {
	msg, err := RenderMemberJoined(MemberJoinedEmail{
		To: "ana@example.com", Name: "Ana", Locale: "en",
		Member: "Carl", MemberEmail: "carl@example.com", Role: "member", WorkspaceName: "Home",
		Link: "http://web.test/settings/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "Carl joined Home" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	for _, want := range []string{"Hi Ana", "Carl (carl@example.com)", "“Home”", "Role: Member.", "http://web.test/settings/workspace"} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("text without %q:\n%s", want, msg.Text)
		}
	}

	msg, err = RenderMemberJoined(MemberJoinedEmail{To: "ana@example.com", Locale: "es", Member: "carl@example.com", MemberEmail: "carl@example.com", Role: "owner", WorkspaceName: "Casa"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject != "carl@example.com se unió a Casa" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	for _, want := range []string{"Hola", "«Casa»", "Rol: Propietario."} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("text without %q:\n%s", want, msg.Text)
		}
	}
	if strings.Contains(msg.Text, "(carl@example.com)") {
		t.Fatalf("the address should not repeat when it is the display name:\n%s", msg.Text)
	}
}
