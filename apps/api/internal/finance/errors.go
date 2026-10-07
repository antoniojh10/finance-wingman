package finance

import "fmt"

type ErrorKind int

const (
	KindInvalid ErrorKind = iota + 1
	KindNotFound
	KindConflict
	KindForbidden
)

// Error is a domain error that transport layers translate into the
// appropriate response (HTTP status, MCP tool error, ...).
type Error struct {
	Kind    ErrorKind
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

func Invalid(field, message string) *Error {
	return &Error{Kind: KindInvalid, Field: field, Message: message}
}

func NotFound(entity string) *Error {
	return &Error{Kind: KindNotFound, Message: entity + " not found"}
}

func Conflict(message string) *Error {
	return &Error{Kind: KindConflict, Message: message}
}

func Forbidden(message string) *Error {
	return &Error{Kind: KindForbidden, Message: message}
}
