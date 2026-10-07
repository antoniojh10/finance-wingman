package httpapi

import (
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func TestSessionAndSwitchWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var session auth.Session
	api.do(http.MethodGet, "/api/v1/auth/session", nil).expect(http.StatusOK).decode(&session)
	if session.Token != "" || session.User.ID != api.owner.ID || session.Workspace == nil || session.Workspace.ID != api.workspace.ID || session.Workspace.Role != "owner" {
		t.Fatalf("unexpected session: %+v", session)
	}

	var trip workspace.Membership
	api.do(http.MethodPost, "/api/v1/workspaces", map[string]any{"name": "Trip"}).expect(http.StatusCreated).decode(&trip)
	if trip.Name != "Trip" || trip.Role != workspace.RoleOwner {
		t.Fatalf("unexpected workspace: %+v", trip)
	}
	var list listBody[workspace.Membership]
	api.do(http.MethodGet, "/api/v1/workspaces", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 2 {
		t.Fatalf("expected two workspaces: %+v", list.Items)
	}

	api.createAccount("Home checking", "MXN", 0)
	api.do(http.MethodPut, "/api/v1/auth/session/workspace", map[string]any{"workspace_id": trip.ID}).expect(http.StatusOK).decode(&session)
	if session.Workspace.ID != trip.ID {
		t.Fatalf("expected to act on Trip: %+v", session.Workspace)
	}
	var accounts listBody[map[string]any]
	api.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK).decode(&accounts)
	if len(accounts.Items) != 0 {
		t.Fatalf("Trip should have no accounts: %+v", accounts.Items)
	}

	other := api.newUser("other@example.com", "Other")
	api.do(http.MethodPut, "/api/v1/auth/session/workspace", map[string]any{"workspace_id": other.workspace.ID}).expectError(http.StatusNotFound)
	api.do(http.MethodPost, "/api/v1/workspaces", map[string]any{"name": ""}).expectError(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodGet, "/api/v1/workspaces", nil).expectError(http.StatusUnauthorized)
}

type listBody[T any] struct {
	Items []T `json:"items"`
}

func TestRenameWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	path := "/api/v1/workspaces/" + api.workspace.ID.String()

	var w workspace.Workspace
	api.do(http.MethodPatch, path, map[string]any{"name": "Casa"}).expect(http.StatusOK).decode(&w)
	if w.Name != "Casa" {
		t.Fatalf("unexpected workspace: %+v", w)
	}
	other := api.newUser("other@example.com", "Other")
	other.do(http.MethodPatch, path, map[string]any{"name": "Mine"}).expectError(http.StatusNotFound)
	api.do(http.MethodPatch, "/api/v1/workspaces/nope", map[string]any{"name": "Mine"}).expectError(http.StatusUnprocessableEntity)
}

// TestInvitationFlow invites a new user, who previews and accepts the
// invitation, lands in the workspace, and is then managed by the owner.
func TestInvitationFlow(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	base := "/api/v1/workspaces/" + api.workspace.ID.String()

	var inv workspace.Invitation
	api.do(http.MethodPost, base+"/invitations", map[string]any{"email": "bob@example.com"}).expect(http.StatusCreated).decode(&inv)
	if inv.Email != "bob@example.com" || inv.Role != "member" || inv.InvitedBy != "Owner" {
		t.Fatalf("unexpected invitation: %+v", inv)
	}
	var open listBody[workspace.Invitation]
	api.do(http.MethodGet, base+"/invitations", nil).expect(http.StatusOK).decode(&open)
	if len(open.Items) != 1 {
		t.Fatalf("expected one open invitation: %+v", open.Items)
	}
	token := api.mail.LastInvitationToken(t)

	anon := api.as("")
	var preview workspace.InvitationPreview
	anon.do(http.MethodPost, "/api/v1/invitations/preview", map[string]any{"token": token}).expect(http.StatusOK).decode(&preview)
	if preview.WorkspaceName != "Home" || preview.Email != "bob@example.com" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	var session auth.Session
	anon.do(http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token}).expect(http.StatusOK).decode(&session)
	if session.Token == "" || session.User.Email != "bob@example.com" || session.Workspace == nil || session.Workspace.ID != api.workspace.ID || session.Workspace.Role != "member" {
		t.Fatalf("unexpected session: %+v", session)
	}
	anon.do(http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token}).expectError(http.StatusNotFound)
	anon.do(http.MethodPost, "/api/v1/invitations/preview", map[string]any{"token": "nope"}).expectError(http.StatusNotFound)

	// Bob shares the workspace's data but can't manage it.
	bob := api.as(session.Token)
	api.createAccount("Shared", "MXN", 0)
	bob.getAccountsNamed(t, "Shared")
	bob.do(http.MethodPost, base+"/invitations", map[string]any{"email": "carl@example.com"}).expectError(http.StatusForbidden)
	bob.do(http.MethodGet, base+"/invitations", nil).expectError(http.StatusForbidden)

	var members listBody[workspace.Member]
	bob.do(http.MethodGet, base+"/members", nil).expect(http.StatusOK).decode(&members)
	if len(members.Items) != 2 {
		t.Fatalf("expected two members: %+v", members.Items)
	}
	bobPath := base + "/members/" + session.User.ID.String()
	ownerPath := base + "/members/" + api.owner.ID.String()

	bob.do(http.MethodPatch, bobPath, map[string]any{"role": "owner"}).expectError(http.StatusForbidden)
	api.do(http.MethodPatch, ownerPath, map[string]any{"role": "member"}).expectError(http.StatusConflict)
	api.do(http.MethodPatch, bobPath, map[string]any{"role": "admin"}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, bobPath, map[string]any{"role": "owner"}).expect(http.StatusNoContent)
	api.do(http.MethodPatch, bobPath, map[string]any{"role": "member"}).expect(http.StatusNoContent)

	// Bob leaves; his session no longer acts on the workspace.
	bob.do(http.MethodDelete, ownerPath, nil).expectError(http.StatusForbidden)
	bob.do(http.MethodDelete, bobPath, nil).expect(http.StatusNoContent)
	bob.do(http.MethodGet, "/api/v1/accounts", nil).expectError(http.StatusConflict)
	api.do(http.MethodDelete, ownerPath, nil).expectError(http.StatusConflict)
	api.do(http.MethodDelete, bobPath, nil).expectError(http.StatusNotFound)
}

func TestRevokeInvitation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	base := "/api/v1/workspaces/" + api.workspace.ID.String() + "/invitations"

	var inv workspace.Invitation
	api.do(http.MethodPost, base, map[string]any{"email": "bob@example.com", "role": "owner", "locale": "es"}).expect(http.StatusCreated).decode(&inv)
	token := api.mail.LastInvitationToken(t)
	api.do(http.MethodDelete, base+"/"+inv.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, base+"/"+inv.ID.String(), nil).expectError(http.StatusNotFound)
	api.as("").do(http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": token}).expectError(http.StatusNotFound)

	api.do(http.MethodPost, base, map[string]any{"email": "owner@example.com"}).expectError(http.StatusConflict)
	api.do(http.MethodPost, base, map[string]any{"email": "nope"}).expectError(http.StatusUnprocessableEntity)
}

func (a *testAPI) getAccountsNamed(t *testing.T, name string) {
	t.Helper()
	var accounts listBody[map[string]any]
	a.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK).decode(&accounts)
	for _, acc := range accounts.Items {
		if acc["name"] == name {
			return
		}
	}
	t.Fatalf("account %q not visible: %+v", name, accounts.Items)
}
