package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// LogHandler returns a handler that writes every record to local and also
// exports records at level or above through the global logger provider set
// up by [Setup], after removing emails, credentials and secret query values
// from the message and attributes (see [Redact]); the local handler gets the
// records untouched. Request-scoped records logged with a context carry its
// trace and span IDs.
func LogHandler(local slog.Handler, level slog.Leveler) slog.Handler {
	return logHandlerWithProvider(local, level, global.GetLoggerProvider())
}

func logHandlerWithProvider(local slog.Handler, level slog.Leveler, provider log.LoggerProvider) slog.Handler {
	exported := minLevelHandler{
		Handler: redactHandler{otelslog.NewHandler("github.com/antoniojh10/finance-wingman/apps/api", otelslog.WithLoggerProvider(provider))},
		level:   level,
	}
	return multiHandler{local, exported}
}

// minLevelHandler drops records below level. The otelslog handler accepts
// every level, so debug noise would otherwise be exported.
type minLevelHandler struct {
	slog.Handler
	level slog.Leveler
}

func (h minLevelHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.level.Level() && h.Handler.Enabled(ctx, l)
}

func (h minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return minLevelHandler{Handler: h.Handler.WithAttrs(attrs), level: h.level}
}

func (h minLevelHandler) WithGroup(name string) slog.Handler {
	return minLevelHandler{Handler: h.Handler.WithGroup(name), level: h.level}
}

// multiHandler sends each record to every handler that is enabled for its
// level.
type multiHandler []slog.Handler

func (m multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m {
		if h.Enabled(ctx, r.Level) {
			// Handlers may retain the record's attributes; give each its own copy.
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (m multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (m multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithGroup(name)
	}
	return out
}
