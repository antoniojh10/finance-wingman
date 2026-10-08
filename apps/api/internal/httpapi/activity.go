package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type listActivityInput struct {
	ActorID    string `query:"actor_id" format:"uuid" doc:"Only changes made by this member"`
	Channel    string `query:"channel" enum:"web,mcp" doc:"Only changes made from the web app or from MCP clients"`
	EntityType string `query:"entity_type" enum:"transaction" doc:"Only changes to this kind of record"`
	EntityID   string `query:"entity_id" format:"uuid" doc:"Only changes to this record"`
	Cursor     string `query:"cursor" maxLength:"200" doc:"next_cursor of the previous page"`
	Limit      int    `query:"limit" minimum:"1" maximum:"200" default:"50"`
}

func (h *financeHandlers) registerActivity(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-activity",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/activity",
		Summary:     "List workspace activity",
		Description: "Who changed what in the workspace, newest first, with the channel it came from (web app or MCP client). " +
			"Entries hold metadata only (ids, type and the names of changed fields), never amounts or descriptions. " +
			"Paginated with an opaque cursor: pass next_cursor as cursor until it is null.",
		Tags: []string{"Activity"},
	}, func(ctx context.Context, in *listActivityInput) (*bodyOutput[finance.ActivityPage], error) {
		actorID, err := parseOptionalID("query.actor_id", in.ActorID)
		if err != nil {
			return nil, err
		}
		entityID, err := parseOptionalID("query.entity_id", in.EntityID)
		if err != nil {
			return nil, err
		}
		page, err := h.svc.ListActivity(ctx, finance.ActivityFilter{
			ActorID:    actorID,
			Channel:    optionalString(in.Channel),
			EntityType: optionalString(in.EntityType),
			EntityID:   entityID,
			Cursor:     in.Cursor,
			Limit:      in.Limit,
		})
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.ActivityPage]{Body: page}, nil
	})
}
