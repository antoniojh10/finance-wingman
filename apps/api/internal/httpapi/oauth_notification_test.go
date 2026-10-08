package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
)

// appConnectedMails returns the "app connected" emails sent so far.
func appConnectedMails(api *testAPI) []mail.Message {
	var out []mail.Message
	for _, m := range api.mail.Messages() {
		if strings.Contains(m.Subject, "was connected to") || strings.Contains(m.Subject, "se conectó a") {
			out = append(out, m)
		}
	}
	return out
}

func TestOAuthGrantEmailsTheUser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, locale, access, subject string
		contains                      []string
	}{
		{"english read only", "en", "read_only", "Claude was connected to your Finance Wingman account", []string{"Access: Read only.", "Workspace: Home.", "http://web.test/settings/security"}},
		{"english read and write", "en", "read_write", "Claude was connected to your Finance Wingman account", []string{"Access: Read and write."}},
		{"spanish read only", "es", "read_only", "Claude se conectó a tu cuenta de Finance Wingman", []string{"Acceso: Solo lectura.", "Espacio: Home."}},
		{"spanish read and write", "es", "read_write", "Claude se conectó a tu cuenta de Finance Wingman", []string{"Acceso: Lectura y escritura."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			api := newTestAPI(t)
			if _, err := api.pool.Exec(context.Background(), `UPDATE users SET locale = $1 WHERE email = 'owner@example.com'`, tc.locale); err != nil {
				t.Fatal(err)
			}
			client := newOAuthClient(t, api, "none")
			code := client.authorizeAccess(t, "owner@example.com", tc.access)
			if got := appConnectedMails(api); len(got) != 0 {
				t.Fatalf("nothing should be sent before the code is exchanged: %+v", got)
			}

			if tokens := client.exchange(code); tokens.Status != http.StatusOK {
				t.Fatalf("exchange: %+v", tokens)
			}
			got := appConnectedMails(api)
			if len(got) != 1 || got[0].To != "owner@example.com" || got[0].Subject != tc.subject {
				t.Fatalf("expected one email to the user: %+v", got)
			}
			for _, want := range tc.contains {
				if !strings.Contains(got[0].Text, want) {
					t.Fatalf("email without %q:\n%s", want, got[0].Text)
				}
			}
		})
	}
}

func TestOAuthRefreshDoesNotEmailTheUser(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))
	if len(appConnectedMails(api)) != 1 {
		t.Fatalf("expected the grant email: %+v", appConnectedMails(api))
	}

	if refreshed := client.refresh(tokens.RefreshToken); refreshed.Status != http.StatusOK {
		t.Fatalf("refresh: %+v", refreshed)
	}
	// Failed exchanges do not notify either.
	if bad := client.exchange("not-a-code"); bad.Status != http.StatusBadRequest {
		t.Fatalf("bad code: %+v", bad)
	}
	if got := appConnectedMails(api); len(got) != 1 {
		t.Fatalf("only the new grant should be notified: %+v", got)
	}
}

func TestOAuthGrantSucceedsWhenTheEmailFails(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	code := client.authorize(t, "owner@example.com")
	api.mail.Fail = func(m mail.Message) error {
		if strings.Contains(m.Subject, "was connected to") {
			return errors.New("mail server down")
		}
		return nil
	}

	tokens := client.exchange(code)
	if tokens.Status != http.StatusOK || tokens.AccessToken == "" {
		t.Fatalf("a failed notification must not fail the connection: %+v", tokens)
	}
	api.as(tokens.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
	if len(api.listConnections()) != 1 {
		t.Fatal("the app should be connected")
	}
}
