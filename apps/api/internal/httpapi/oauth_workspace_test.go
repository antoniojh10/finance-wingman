package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var workspaceOptionPattern = regexp.MustCompile(`name="workspace_id" value="([0-9a-f-]{36})"`)

// TestOAuthChoosesWorkspace connects an MCP client for a user with two
// workspaces: the authorize page asks which one, and the issued tokens (and
// their refreshes) act on it.
func TestOAuthChoosesWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Home checking", "MXN", 0)
	trip, err := api.workspaces.Create(context.Background(), api.owner.ID, "Trip")
	if err != nil {
		t.Fatal(err)
	}
	stranger := api.newUser("other@example.com", "Other")

	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)
	client.submit(requestID, map[string]string{"action": "send_code", "email": "owner@example.com"})
	rec := client.submit(requestID, map[string]string{"action": "verify", "code": api.mail.LastCode(t)})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected the workspace step, got %d: %s", rec.Code, rec.Body)
	}
	options := workspaceOptionPattern.FindAllStringSubmatch(rec.Body.String(), -1)
	if len(options) != 2 || !strings.Contains(rec.Body.String(), "Trip") || !strings.Contains(rec.Body.String(), "Home") {
		t.Fatalf("expected both workspaces as options:\n%s", rec.Body)
	}

	// Workspaces the user doesn't belong to are refused.
	rec = client.submit(requestID, map[string]string{"action": "choose_workspace", "workspace_id": stranger.workspace.ID.String()})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign workspace: expected 422, got %d", rec.Code)
	}

	rec = client.submit(requestID, map[string]string{"action": "choose_workspace", "workspace_id": trip.ID.String()})
	if rec.Code != http.StatusFound {
		t.Fatalf("choose workspace: %d %s", rec.Code, rec.Body)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	tokens := client.exchange(loc.Query().Get("code"))
	if tokens.Status != http.StatusOK {
		t.Fatalf("exchange: %+v", tokens)
	}

	assertWorkspace := func(token string) {
		t.Helper()
		session, err := api.auth.Authenticate(context.Background(), token)
		if err != nil || session.Workspace == nil || session.Workspace.ID != trip.ID {
			t.Fatalf("expected the token to act on Trip: %+v %v", session.Workspace, err)
		}
	}
	assertWorkspace(tokens.AccessToken)
	refreshed := client.refresh(tokens.RefreshToken)
	if refreshed.Status != http.StatusOK {
		t.Fatalf("refresh: %+v", refreshed)
	}
	assertWorkspace(refreshed.AccessToken)

	// The request can't be reused once approved.
	if rec := client.submit(requestID, map[string]string{"action": "choose_workspace", "workspace_id": trip.ID.String()}); rec.Code != http.StatusBadRequest {
		t.Fatalf("reused request: expected 400, got %d", rec.Code)
	}
}

func TestOAuthWorkspaceStepRequiresVerifiedCode(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)

	// Skipping the code step falls back to the email step.
	rec := client.submit(requestID, map[string]string{"action": "choose_workspace", "workspace_id": api.workspace.ID.String()})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `value="send_code"`) {
		t.Fatalf("expected the email step, got %d:\n%s", rec.Code, rec.Body)
	}
}

func TestOAuthWithoutWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	if _, err := api.auth.AddUser(context.Background(), "nomad@example.com", ""); err != nil {
		t.Fatal(err)
	}
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)
	client.submit(requestID, map[string]string{"action": "send_code", "email": "nomad@example.com"})
	rec := client.submit(requestID, map[string]string{"action": "verify", "code": api.mail.LastCode(t)})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "doesn&#39;t belong to any workspace") {
		t.Fatalf("expected the no-workspace page, got %d:\n%s", rec.Code, rec.Body)
	}
}
