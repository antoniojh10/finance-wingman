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

// freezeClock fixes the workspace service's clock and returns a function
// that moves it forward.
func (a *testAPI) freezeClock() func(time.Duration) {
	now := time.Now().UTC().Truncate(time.Second)
	a.workspaces.SetClock(func() time.Time { return now })
	return func(d time.Duration) { now = now.Add(d) }
}

// addMember adds a new user to the owner's workspace as a member and
// returns a client signed in as them, acting on it.
func (a *testAPI) addMember(email string) *testAPI {
	a.t.Helper()
	ctx := context.Background()
	user, err := a.auth.AddUser(ctx, email, "")
	if err != nil {
		a.t.Fatal(err)
	}
	if _, err := a.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, a.workspace.ID, user.ID); err != nil {
		a.t.Fatal(err)
	}
	session, err := a.auth.CreateWorkspaceSession(ctx, user.ID, &a.workspace.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		a.t.Fatal(err)
	}
	return a.as(session.Token)
}

func TestWorkspaceDeletionFlow(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	advance := api.freezeClock()
	path := "/api/v1/workspaces/" + api.workspace.ID.String() + "/deletion"
	member := api.addMember("bob@example.com")
	api.createAccount("Checking", "EUR", 1000)

	api.do(http.MethodPost, path, map[string]any{"name": "home"}).expectError(http.StatusUnprocessableEntity)
	member.do(http.MethodPost, path, map[string]any{"name": "Home"}).expectError(http.StatusForbidden)
	api.newUser("eve@example.com", "Eve's").do(http.MethodPost, path, map[string]any{"name": "Home"}).expectError(http.StatusNotFound)
	api.do(http.MethodPost, "/api/v1/workspaces/nope/deletion", map[string]any{"name": "Home"}).expectError(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodPost, path, map[string]any{"name": "Home"}).expectError(http.StatusUnauthorized)
	api.do(http.MethodDelete, path, nil).expectError(http.StatusNotFound)

	var w workspace.Workspace
	api.do(http.MethodPost, path, map[string]any{"name": "Home"}).expect(http.StatusOK).decode(&w)
	if w.DeletionScheduledFor == nil {
		t.Fatalf("expected a scheduled deletion: %+v", w)
	}
	if n := len(api.mail.Messages()); n != 2 {
		t.Fatalf("expected an email to each member, got %d", n)
	}
	api.do(http.MethodPost, path, map[string]any{"name": "Home"}).expectError(http.StatusConflict)

	var list listBody[workspace.Membership]
	member.do(http.MethodGet, "/api/v1/workspaces", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].DeletionScheduledFor == nil {
		t.Fatalf("members should see the scheduled deletion: %+v", list.Items)
	}

	member.do(http.MethodDelete, path, nil).expectError(http.StatusForbidden)
	api.do(http.MethodDelete, path, nil).expect(http.StatusNoContent)
	list = listBody[workspace.Membership]{}
	api.do(http.MethodGet, "/api/v1/workspaces", nil).expect(http.StatusOK).decode(&list)
	if list.Items[0].DeletionScheduledFor != nil {
		t.Fatalf("the deletion should be cancelled: %+v", list.Items)
	}

	api.do(http.MethodPost, path, map[string]any{"name": "Home"}).expect(http.StatusOK)
	advance(workspace.DefaultDeletionGracePeriod + time.Minute)
	if err := api.workspaces.ExecuteScheduledDeletions(context.Background()); err != nil {
		t.Fatal(err)
	}

	var session auth.Session
	api.do(http.MethodGet, "/api/v1/auth/session", nil).expect(http.StatusOK).decode(&session)
	if session.Workspace != nil {
		t.Fatalf("the session should no longer act on the deleted workspace: %+v", session.Workspace)
	}
	list = listBody[workspace.Membership]{}
	api.do(http.MethodGet, "/api/v1/workspaces", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 0 {
		t.Fatalf("the workspace should be gone: %+v", list.Items)
	}
	member.do(http.MethodGet, "/api/v1/accounts", nil).expectError(http.StatusConflict)
	last := api.mail.Messages()[len(api.mail.Messages())-1]
	if !strings.Contains(last.Subject, "was deleted") {
		t.Fatalf("expected a deletion email, got %q", last.Subject)
	}
}

func TestWorkspaceDeletionRequiresBrowserSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	app := api.as(client.exchange(client.authorize(t, "owner@example.com")).AccessToken)
	path := "/api/v1/workspaces/" + api.workspace.ID.String() + "/deletion"

	app.do(http.MethodPost, path, map[string]any{"name": "Home"}).expectError(http.StatusForbidden)
	app.do(http.MethodDelete, path, nil).expectError(http.StatusForbidden)
	// The sign-in code and the "app connected" notice; no deletion email.
	if n := len(api.mail.Messages()); n != 2 {
		t.Fatalf("only the sign-in and app connected emails should have been sent, got %d", n)
	}
}
