package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func (h *workspaceHandlers) registerAccountDeletion(api huma.API) {
	tags := []string{"Auth"}
	path := apiPrefix + "/auth/me/deletion"

	huma.Register(api, huma.Operation{
		OperationID: "get-account-deletion",
		Method:      http.MethodGet,
		Path:        path,
		Summary:     "Describe the deletion of the user's account",
		Description: "Whether the account is scheduled for deletion, and what deleting it does to each of the user's workspaces.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*bodyOutput[workspace.AccountDeletion], error) {
		session, err := browserSession(ctx, errDeletionWebSessionOnly)
		if err != nil {
			return nil, err
		}
		d, err := h.svc.AccountDeletion(ctx, session.User.ID)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.AccountDeletion]{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "schedule-account-deletion",
		Method:      http.MethodPost,
		Path:        path,
		Summary:     "Schedule the deletion of the user's account",
		Description: "From a signed-in browser session; the body repeats the user's email to confirm. " +
			"Refused (409) while the user is the only owner of a workspace with other members. " +
			"Once the grace period (7 days) is over, the user leaves every workspace (their accounts become shared), " +
			"workspaces where they are the only member are deleted with all their data, and the user is deleted with their sessions and connected apps. " +
			"Until then they can sign in and cancel. The user is emailed when it is scheduled and when it is carried out.",
		Tags: tags,
	}, func(ctx context.Context, in *struct {
		Body struct {
			Email string `json:"email" maxLength:"254" doc:"The user's email address, to confirm"`
		}
	}) (*bodyOutput[workspace.AccountDeletion], error) {
		session, err := browserSession(ctx, errDeletionWebSessionOnly)
		if err != nil {
			return nil, err
		}
		d, err := h.svc.ScheduleAccountDeletion(ctx, session.User.ID, in.Body.Email)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[workspace.AccountDeletion]{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "cancel-account-deletion",
		Method:        http.MethodDelete,
		Path:          path,
		Summary:       "Cancel the scheduled deletion of the user's account",
		Description:   "From a signed-in browser session.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		session, err := browserSession(ctx, errDeletionWebSessionOnly)
		if err != nil {
			return nil, err
		}
		if err := h.svc.CancelAccountDeletion(ctx, session.User.ID); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})
}
