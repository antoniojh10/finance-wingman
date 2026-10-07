package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

type workspaceHandlers struct {
	svc    *workspace.Service
	auth   *auth.Service
	logger *slog.Logger
}

type workspaceNameBody struct {
	Name string `json:"name" minLength:"1" maxLength:"100"`
}

type workspaceIDInput struct {
	ID string `path:"id" format:"uuid"`
}

type memberInput struct {
	ID     string `path:"id" format:"uuid"`
	UserID string `path:"user_id" format:"uuid"`
}

type invitationIDInput struct {
	ID           string `path:"id" format:"uuid"`
	InvitationID string `path:"invitation_id" format:"uuid"`
}

type invitationTokenBody struct {
	Token string `json:"token" minLength:"1" doc:"Token from the invitation link"`
}

func registerWorkspaces(api huma.API, svc *workspace.Service, authSvc *auth.Service, logger *slog.Logger) {
	h := &workspaceHandlers{svc: svc, auth: authSvc, logger: logger}
	h.registerSession(api)
	h.registerWorkspaces(api)
	h.registerMembers(api)
	h.registerInvitations(api)
}

func (h *workspaceHandlers) fail(ctx context.Context, err error) error {
	return toHTTPError(ctx, h.logger, err)
}

func (h *workspaceHandlers) registerSession(api huma.API) {
	tags := []string{"Auth"}

	huma.Register(api, huma.Operation{
		OperationID: "get-session",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/auth/session",
		Summary:     "Get the current session",
		Description: "Returns the signed-in user and the workspace the session acts on.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*bodyOutput[auth.Session], error) {
		session, _ := sessionFrom(ctx)
		return &bodyOutput[auth.Session]{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "switch-workspace",
		Method:      http.MethodPut,
		Path:        apiPrefix + "/auth/session/workspace",
		Summary:     "Switch the session to another workspace",
		Tags:        tags,
	}, func(ctx context.Context, in *struct {
		Body struct {
			WorkspaceID uuid.UUID `json:"workspace_id"`
		}
	}) (*bodyOutput[auth.Session], error) {
		session, _ := sessionFrom(ctx)
		switched, err := h.auth.SwitchWorkspace(ctx, session, in.Body.WorkspaceID)
		if errors.Is(err, auth.ErrNotMember) {
			return nil, huma.Error404NotFound("workspace not found")
		}
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[auth.Session]{Body: switched}, nil
	})
}

func (h *workspaceHandlers) registerWorkspaces(api huma.API) {
	tags := []string{"Workspaces"}

	huma.Register(api, huma.Operation{
		OperationID: "list-workspaces",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/workspaces",
		Summary:     "List the workspaces the user belongs to",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*listOutput[workspace.Membership], error) {
		session, _ := sessionFrom(ctx)
		items, err := h.svc.List(ctx, session.User.ID)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-workspace",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/workspaces",
		Summary:       "Create a workspace",
		Description:   "The user becomes its owner. The session keeps its current workspace; switch to the new one with PUT /auth/session/workspace.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *struct{ Body workspaceNameBody }) (*bodyOutput[workspace.Membership], error) {
		session, _ := sessionFrom(ctx)
		w, err := h.svc.Create(ctx, session.User.ID, in.Body.Name)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.Membership]{Body: workspace.Membership{Workspace: w, Role: workspace.RoleOwner}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "rename-workspace",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/workspaces/{id}",
		Summary:     "Rename a workspace",
		Description: "Owners only.",
		Tags:        tags,
	}, func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body workspaceNameBody
	}) (*bodyOutput[workspace.Workspace], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		w, err := h.svc.Rename(ctx, session.User.ID, id, in.Body.Name)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.Workspace]{Body: w}, nil
	})
}

func (h *workspaceHandlers) registerMembers(api huma.API) {
	tags := []string{"Workspaces"}

	huma.Register(api, huma.Operation{
		OperationID: "list-workspace-members",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/workspaces/{id}/members",
		Summary:     "List the members of a workspace",
		Tags:        tags,
	}, func(ctx context.Context, in *workspaceIDInput) (*listOutput[workspace.Member], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		items, err := h.svc.Members(ctx, session.User.ID, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "update-workspace-member",
		Method:        http.MethodPatch,
		Path:          apiPrefix + "/workspaces/{id}/members/{user_id}",
		Summary:       "Change a member's role",
		Description:   "Owners only. The last owner can't be made a member.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *struct {
		ID     string `path:"id" format:"uuid"`
		UserID string `path:"user_id" format:"uuid"`
		Body   struct {
			Role string `json:"role" enum:"owner,member"`
		}
	}) (*struct{}, error) {
		workspaceID, userID, err := parseMember(in.ID, in.UserID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		if err := h.svc.SetRole(ctx, session.User.ID, workspaceID, userID, in.Body.Role); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "remove-workspace-member",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/workspaces/{id}/members/{user_id}",
		Summary:       "Remove a member, or leave a workspace",
		Description:   "Owners can remove anyone; members can only remove themselves. The last owner can't leave.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *memberInput) (*struct{}, error) {
		workspaceID, userID, err := parseMember(in.ID, in.UserID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		if err := h.svc.RemoveMember(ctx, session.User.ID, workspaceID, userID); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})
}

func (h *workspaceHandlers) registerInvitations(api huma.API) {
	tags := []string{"Workspaces"}
	public := []map[string][]string{}

	huma.Register(api, huma.Operation{
		OperationID: "list-workspace-invitations",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/workspaces/{id}/invitations",
		Summary:     "List open invitations",
		Description: "Owners only.",
		Tags:        tags,
	}, func(ctx context.Context, in *workspaceIDInput) (*listOutput[workspace.Invitation], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		items, err := h.svc.Invitations(ctx, session.User.ID, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-workspace-invitation",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/workspaces/{id}/invitations",
		Summary:       "Invite someone by email",
		Description:   "Owners only. Emails a link to join the workspace; inviting the same address again replaces its open invitation.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body workspace.InviteInput
	}) (*bodyOutput[workspace.Invitation], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		inv, err := h.svc.Invite(ctx, session.User.ID, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.Invitation]{Body: inv}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revoke-workspace-invitation",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/workspaces/{id}/invitations/{invitation_id}",
		Summary:       "Revoke an open invitation",
		Description:   "Owners only.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *invitationIDInput) (*struct{}, error) {
		workspaceID, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		invitationID, err := parseID(in.InvitationID)
		if err != nil {
			return nil, err
		}
		session, _ := sessionFrom(ctx)
		if err := h.svc.RevokeInvitation(ctx, session.User.ID, workspaceID, invitationID); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "preview-invitation",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/invitations/preview",
		Summary:     "Describe an invitation from its token",
		Description: "The token travels in the body so it doesn't end up in access logs.",
		Tags:        tags,
		Security:    public,
		Metadata:    publicMetadata,
	}, func(ctx context.Context, in *struct{ Body invitationTokenBody }) (*bodyOutput[workspace.InvitationPreview], error) {
		preview, err := h.svc.PreviewInvitation(ctx, in.Body.Token)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.InvitationPreview]{Body: preview}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "accept-invitation",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/invitations/accept",
		Summary:     "Accept an invitation",
		Description: "Adds the invitee to the workspace (creating their user if needed) and returns a session acting on it. The token proves ownership of the invited address, like a magic link.",
		Tags:        tags,
		Security:    public,
		Metadata:    publicMetadata,
	}, func(ctx context.Context, in *struct{ Body invitationTokenBody }) (*bodyOutput[auth.Session], error) {
		accepted, err := h.svc.AcceptInvitation(ctx, in.Body.Token)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		session, err := h.auth.CreateWorkspaceSession(ctx, accepted.UserID, &accepted.WorkspaceID, auth.ClientWeb, 0)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[auth.Session]{Body: session}, nil
	})
}

func parseMember(rawWorkspace, rawUser string) (uuid.UUID, uuid.UUID, error) {
	workspaceID, err := parseID(rawWorkspace)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	userID, err := parseID(rawUser)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return workspaceID, userID, nil
}
