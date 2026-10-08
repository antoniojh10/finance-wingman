package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
)

const redacted = "[redacted]"

var (
	// emailPattern matches addresses echoed by SMTP servers and mail APIs in
	// error messages.
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`)
	// secretParamPattern matches secret-bearing query or form parameters
	// (key=value) such as magic link tokens and OAuth codes.
	secretParamPattern = regexp.MustCompile(`(?i)\b(token|code|state|code_challenge|code_verifier|refresh_token|access_token|client_secret|q)=[^&\s"']+`)
	// bearerPattern matches Authorization header values.
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=\-]{8,}`)
)

// Redact removes personal data and credentials from free text that is about
// to leave the process: email addresses, bearer credentials and the values of
// secret query parameters.
func Redact(s string) string {
	s = emailPattern.ReplaceAllString(s, redacted)
	s = secretParamPattern.ReplaceAllString(s, "${1}="+redacted)
	return bearerPattern.ReplaceAllString(s, "${1} "+redacted)
}

// redactHandler applies [Redact] to the message and to every string or error
// attribute before delegating. Errors are the main carrier: SMTP and mail API
// failures echo the recipient, database errors may echo values.
type redactHandler struct {
	slog.Handler
}

func (h redactHandler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, Redact(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(redactAttr(a))
		return true
	})
	return h.Handler.Handle(ctx, clean)
}

func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		clean[i] = redactAttr(a)
	}
	return redactHandler{h.Handler.WithAttrs(clean)}
}

func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{h.Handler.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(v.String()))
	case slog.KindGroup:
		group := v.Group()
		clean := make([]any, len(group))
		for i, g := range group {
			clean[i] = redactAttr(g)
		}
		return slog.Group(a.Key, clean...)
	case slog.KindAny:
		switch x := v.Any().(type) {
		case error:
			return slog.String(a.Key, Redact(x.Error()))
		case fmt.Stringer:
			return slog.String(a.Key, Redact(x.String()))
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
