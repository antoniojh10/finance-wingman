package finance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Channels a change can come from.
const (
	ChannelWeb = "web"
	ChannelMCP = "mcp"
)

// Activity actions and entity types.
const (
	ActionTransactionCreated = "transaction.created"
	ActionTransactionUpdated = "transaction.updated"
	ActionTransactionDeleted = "transaction.deleted"

	ActionAccountCreated    = "account.created"
	ActionAccountUpdated    = "account.updated"
	ActionAccountArchived   = "account.archived"
	ActionAccountUnarchived = "account.unarchived"
	ActionAccountDeleted    = "account.deleted"

	ActionCategoryCreated    = "category.created"
	ActionCategoryUpdated    = "category.updated"
	ActionCategoryArchived   = "category.archived"
	ActionCategoryUnarchived = "category.unarchived"
	ActionCategoryDeleted    = "category.deleted"

	ActionBudgetSet     = "budget.set"
	ActionBudgetCleared = "budget.cleared"

	ActionRecurringCreated = "recurring_item.created"
	ActionRecurringUpdated = "recurring_item.updated"

	ActionSuggestionDismissed = "recurring_suggestion.dismissed"

	ActionExportRequested = "export.requested"

	EntityTransaction         = "transaction"
	EntityAccount             = "account"
	EntityCategory            = "category"
	EntityBudget              = "budget"
	EntityRecurringItem       = "recurring_item"
	EntityRecurringSuggestion = "recurring_suggestion"
	EntityWorkspace           = "workspace"
)

// EntityTypes lists the kinds of record the log can name, in the order the
// API documents them. Keep it in sync with the enum of the entity_type
// query parameter.
var EntityTypes = []string{
	EntityTransaction, EntityAccount, EntityCategory, EntityBudget,
	EntityRecurringItem, EntityRecurringSuggestion, EntityWorkspace,
}

// Channel says how a request reached the API: the web app or an MCP client,
// with the OAuth client behind it when known.
type Channel struct {
	Kind       string
	ClientID   string
	ClientName string
}

type channelKey struct{}

// WithChannel attaches the channel to the context so changes are logged
// with it.
func WithChannel(ctx context.Context, ch Channel) context.Context {
	return context.WithValue(ctx, channelKey{}, ch)
}

// ChannelFrom returns the channel stored in the context, defaulting to web.
func ChannelFrom(ctx context.Context) Channel {
	if ch, ok := ctx.Value(channelKey{}).(Channel); ok && ch.Kind != "" {
		return ch
	}
	return Channel{Kind: ChannelWeb}
}

// ActivityDetails are the facts logged about a change. They are metadata
// only: ids, the kind of record and the names of the fields an update
// touched. Never add amounts, descriptions, names or other values here; the
// log must not reveal more than the ids it mentions.
type ActivityDetails struct {
	Type                 string     `json:"type,omitempty" doc:"Kind of the record: transaction or recurring item type, account type or category kind" example:"expense"`
	AccountID            *uuid.UUID `json:"account_id,omitempty" format:"uuid"`
	DestinationAccountID *uuid.UUID `json:"destination_account_id,omitempty" format:"uuid"`
	CategoryID           *uuid.UUID `json:"category_id,omitempty" format:"uuid" doc:"Category of a budget"`
	Currency             string     `json:"currency,omitempty" doc:"ISO 4217 currency of an account or budget" example:"MXN"`
	Month                string     `json:"month,omitempty" pattern:"^\\d{4}-\\d{2}$" doc:"Month (YYYY-MM) a budget applies from"`
	Format               string     `json:"format,omitempty" enum:"json,csv" doc:"File format of an export"`
	Changed              []string   `json:"changed,omitempty" doc:"Fields modified by an update (names only, never values)"`
}

// ActivityEntry is one line of the workspace activity log.
type ActivityEntry struct {
	ID         uuid.UUID       `json:"id"`
	Action     string          `json:"action" example:"transaction.created"`
	EntityType string          `json:"entity_type" example:"transaction"`
	EntityID   uuid.UUID       `json:"entity_id" format:"uuid"`
	Actor      *UserRef        `json:"actor,omitempty" doc:"Member who made the change. Omitted when the user was erased"`
	Channel    string          `json:"channel" enum:"web,mcp"`
	ClientID   *string         `json:"client_id" nullable:"true" doc:"OAuth client behind an MCP change"`
	ClientName *string         `json:"client_name" nullable:"true" doc:"Name the OAuth client registered with, as it was when the change was made"`
	Details    ActivityDetails `json:"details"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ActivityFilter selects entries; nil fields match everything. Cursor is
// the NextCursor of the previous page.
type ActivityFilter struct {
	ActorID    *uuid.UUID
	Channel    *string
	EntityType *string
	EntityID   *uuid.UUID
	Cursor     string
	Limit      int
}

type ActivityPage struct {
	Items      []ActivityEntry `json:"items"`
	NextCursor *string         `json:"next_cursor" nullable:"true" doc:"Pass as cursor to get the next page; null on the last page"`
	Limit      int             `json:"limit"`
}

// record appends an entry attributed to the context's actor and channel. It
// must run on the same database transaction as the change it describes, so
// a rolled back change leaves no entry.
func (s *Service) record(ctx context.Context, action, entityType string, entityID uuid.UUID, details ActivityDetails) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	ch := ChannelFrom(ctx)
	params := store.InsertActivityParams{
		Channel:    ch.Kind,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    raw,
	}
	if actor, ok := ActorFrom(ctx); ok {
		params.ActorID = &actor
	}
	if ch.ClientID != "" {
		params.ClientID = &ch.ClientID
		params.ClientName = &ch.ClientName
	}
	return s.q.InsertActivity(ctx, params)
}

// RecordExport logs that the workspace data was exported. The export is
// not a change to a record, so the entry names the workspace itself.
func (s *Service) RecordExport(ctx context.Context, workspaceID uuid.UUID, format string) error {
	return s.record(ctx, ActionExportRequested, EntityWorkspace, workspaceID, ActivityDetails{Format: format})
}

// ListActivity returns the workspace activity newest first, a page at a
// time.
func (s *Service) ListActivity(ctx context.Context, f ActivityFilter) (ActivityPage, error) {
	if f.Channel != nil && *f.Channel != ChannelWeb && *f.Channel != ChannelMCP {
		return ActivityPage{}, Invalid("channel", "must be web or mcp")
	}
	if f.EntityType != nil && !slices.Contains(EntityTypes, *f.EntityType) {
		return ActivityPage{}, Invalid("entity_type", "must be one of "+strings.Join(EntityTypes, ", "))
	}
	limit := DefaultPageSize
	if f.Limit > 0 {
		limit = min(f.Limit, MaxPageSize)
	}
	params := store.ListActivityParams{
		ActorID:    f.ActorID,
		Channel:    f.Channel,
		EntityType: f.EntityType,
		EntityID:   f.EntityID,
		RowLimit:   int32(limit + 1),
	}
	if f.Cursor != "" {
		at, id, err := decodeActivityCursor(f.Cursor)
		if err != nil {
			return ActivityPage{}, err
		}
		params.BeforeCreatedAt, params.BeforeID = &at, &id
	}
	rows, err := s.q.ListActivity(ctx, params)
	if err != nil {
		return ActivityPage{}, err
	}
	page := ActivityPage{Items: make([]ActivityEntry, 0, min(len(rows), limit)), Limit: limit}
	for i, r := range rows {
		if i == limit {
			last := page.Items[limit-1]
			cursor := encodeActivityCursor(last.CreatedAt, last.ID)
			page.NextCursor = &cursor
			break
		}
		var details ActivityDetails
		if err := json.Unmarshal(r.Details, &details); err != nil {
			return ActivityPage{}, err
		}
		page.Items = append(page.Items, ActivityEntry{
			ID:         r.ID,
			Action:     r.Action,
			EntityType: r.EntityType,
			EntityID:   r.EntityID,
			Actor:      userRef(r.ActorID, r.ActorName, r.ActorEmail),
			Channel:    r.Channel,
			ClientID:   r.ClientID,
			ClientName: r.ClientName,
			Details:    details,
			CreatedAt:  r.CreatedAt,
		})
	}
	return page, nil
}

// The cursor is the position of the last entry of a page: its timestamp
// and id, opaque to clients.
func encodeActivityCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeActivityCursor(cursor string) (time.Time, uuid.UUID, error) {
	invalid := Invalid("cursor", "is not a valid cursor: pass the next_cursor of the previous page")
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, invalid
	}
	ts, rawID, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, invalid
	}
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, invalid
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return time.Time{}, uuid.Nil, invalid
	}
	return at, id, nil
}
