package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

const accountDeletionPath = "/api/v1/auth/me/deletion"

func TestAccountDeletionFlow(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	advance := api.freezeClock()
	member := api.addMember("bob@example.com")
	api.createAccount("Checking", "EUR", 1000)

	var d workspace.AccountDeletion
	api.do(http.MethodGet, accountDeletionPath, nil).expect(http.StatusOK).decode(&d)
	if d.ScheduledFor != nil || len(d.Workspaces) != 1 || d.Workspaces[0].Outcome != workspace.OutcomeBlocked || d.Workspaces[0].Members != 2 {
		t.Fatalf("the only owner of a shared workspace is blocked: %+v", d)
	}
	api.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "someone@example.com"}).expectError(http.StatusUnprocessableEntity)
	body := api.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "owner@example.com"}).expectError(http.StatusConflict)
	if !strings.Contains(body.Detail, `"Home"`) {
		t.Fatalf("the conflict should name the workspace: %+v", body)
	}
	api.as("").do(http.MethodGet, accountDeletionPath, nil).expectError(http.StatusUnauthorized)

	// The member can delete their account: they just leave Home.
	member.do(http.MethodDelete, accountDeletionPath, nil).expectError(http.StatusNotFound)
	d = workspace.AccountDeletion{}
	member.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "bob@example.com"}).expect(http.StatusOK).decode(&d)
	if d.ScheduledFor == nil || len(d.Workspaces) != 1 || d.Workspaces[0].Outcome != workspace.OutcomeLeave {
		t.Fatalf("unexpected deletion: %+v", d)
	}
	member.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "bob@example.com"}).expectError(http.StatusConflict)
	var me auth.User
	member.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK).decode(&me)
	if me.DeletionScheduledFor == nil || !me.DeletionScheduledFor.Equal(*d.ScheduledFor) {
		t.Fatalf("the session should report the scheduled deletion: %+v", me)
	}

	member.do(http.MethodDelete, accountDeletionPath, nil).expect(http.StatusNoContent)
	me = auth.User{}
	member.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK).decode(&me)
	if me.DeletionScheduledFor != nil {
		t.Fatalf("the deletion should be cancelled: %+v", me)
	}

	member.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "bob@example.com"}).expect(http.StatusOK)
	advance(workspace.DefaultDeletionGracePeriod + time.Minute)
	if err := api.workspaces.ExecuteScheduledDeletions(context.Background()); err != nil {
		t.Fatal(err)
	}

	member.do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	var members listBody[workspace.Member]
	api.do(http.MethodGet, "/api/v1/workspaces/"+api.workspace.ID.String()+"/members", nil).expect(http.StatusOK).decode(&members)
	if len(members.Items) != 1 {
		t.Fatalf("bob should have left Home: %+v", members.Items)
	}
	api.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "bob@example.com"}).expect(http.StatusAccepted)
	last := api.mail.Messages()[len(api.mail.Messages())-1]
	if last.To != "bob@example.com" || !strings.Contains(last.Subject, "was deleted") {
		t.Fatalf("a deleted user can't sign in; the last email should be the deletion notice: %q to %s", last.Subject, last.To)
	}

	// With bob gone, the owner is Home's only member: deleting the account
	// deletes the workspace too.
	d = workspace.AccountDeletion{}
	api.do(http.MethodGet, accountDeletionPath, nil).expect(http.StatusOK).decode(&d)
	if len(d.Workspaces) != 1 || d.Workspaces[0].Outcome != workspace.OutcomeDelete {
		t.Fatalf("expected Home to be deleted with the account: %+v", d)
	}
}

func TestAccountDeletionRequiresBrowserSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	app := api.as(client.exchange(client.authorize(t, "owner@example.com")).AccessToken)

	app.do(http.MethodGet, accountDeletionPath, nil).expectError(http.StatusForbidden)
	app.do(http.MethodPost, accountDeletionPath, map[string]any{"email": "owner@example.com"}).expectError(http.StatusForbidden)
	app.do(http.MethodDelete, accountDeletionPath, nil).expectError(http.StatusForbidden)
	var me auth.User
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK).decode(&me)
	if me.DeletionScheduledFor != nil {
		t.Fatal("an app must not schedule the deletion")
	}
}
