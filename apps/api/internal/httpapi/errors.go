package httpapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// toHTTPError converts domain errors into Huma errors with the right status.
// Unexpected errors are logged and hidden behind a generic 500.
func toHTTPError(ctx context.Context, logger *slog.Logger, err error) error {
	var domainErr *finance.Error
	if errors.As(err, &domainErr) {
		switch domainErr.Kind {
		case finance.KindInvalid:
			return huma.Error422UnprocessableEntity(domainErr.Error(), &huma.ErrorDetail{
				Location: domainErr.Field,
				Message:  domainErr.Message,
			})
		case finance.KindNotFound:
			return huma.Error404NotFound(domainErr.Message)
		case finance.KindConflict:
			return huma.Error409Conflict(domainErr.Message)
		case finance.KindForbidden:
			return huma.Error403Forbidden(domainErr.Message)
		}
	}
	logger.ErrorContext(ctx, "unexpected error", "error", err)
	return huma.Error500InternalServerError("internal server error")
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, huma.Error422UnprocessableEntity("invalid id", &huma.ErrorDetail{
			Location: "path.id",
			Message:  "must be a valid UUID",
			Value:    raw,
		})
	}
	return id, nil
}

// parseOptionalID parses an optional UUID query parameter.
func parseOptionalID(location, raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("invalid "+location, &huma.ErrorDetail{
			Location: location,
			Message:  "must be a valid UUID",
			Value:    raw,
		})
	}
	return &id, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
