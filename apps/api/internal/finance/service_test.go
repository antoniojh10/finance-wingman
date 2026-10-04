package finance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTodayUsesConfiguredTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{loc: loc, now: func() time.Time {
		// 03:00 UTC on Oct 5 is still Oct 4 in Mexico City (UTC-6).
		return time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	}}
	if got := formatDate(s.Today()); got != "2026-10-04" {
		t.Fatalf("Today() = %s, want 2026-10-04", got)
	}
}

func TestActorContext(t *testing.T) {
	if _, ok := ActorFrom(context.Background()); ok {
		t.Fatal("expected no actor in an empty context")
	}
	id := uuid.New()
	got, ok := ActorFrom(WithActor(context.Background(), id))
	if !ok || got != id {
		t.Fatalf("ActorFrom() = %v, %v; want %v", got, ok, id)
	}
}

func TestErrorMessages(t *testing.T) {
	if got := Invalid("amount", "must be positive").Error(); got != "amount: must be positive" {
		t.Fatalf("unexpected message %q", got)
	}
	if got := NotFound("account").Error(); got != "account not found" {
		t.Fatalf("unexpected message %q", got)
	}
}
