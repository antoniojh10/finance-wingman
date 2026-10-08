package mcpserver

import (
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/ratelimit"
)

func TestToolCallsAreRateLimitedPerUser(t *testing.T) {
	h := newHarness(t)
	h.actor = uuid.New()
	h.server.SetLimiter(ratelimit.New(60, 2))

	h.mustCall("list_accounts", map[string]any{}, nil)
	h.mustCall("list_accounts", map[string]any{}, nil)
	h.mustFail("list_accounts", map[string]any{}, "Rate limit reached")
	h.mustFail("list_accounts", map[string]any{}, "wait about 1 seconds")

	// Another user has their own budget.
	h.actor = uuid.New()
	h.mustCall("list_accounts", map[string]any{}, nil)
}

func TestToolCallsAreNotLimitedByDefault(t *testing.T) {
	h := newHarness(t)
	h.actor = uuid.New()
	for i := 0; i < 30; i++ {
		h.mustCall("list_accounts", map[string]any{}, nil)
	}
}
