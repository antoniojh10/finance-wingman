package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/ratelimit"
)

// Limits configures request rate limiting. A nil Limits, or a nil limiter
// inside it, disables that limit.
type Limits struct {
	// Unauthenticated limits, per client IP, the endpoints reachable without
	// a session: login, code verification and the OAuth endpoints.
	Unauthenticated *ratelimit.Limiter
	// Authenticated limits, per session, every authenticated REST operation.
	Authenticated *ratelimit.Limiter
	// TrustedProxyHops is the number of reverse proxies in front of the API
	// whose X-Forwarded-For entry can be trusted (see ratelimit.ClientIP).
	TrustedProxyHops int
}

// unauthenticatedPaths are limited per client IP.
var unauthenticatedPaths = map[string]bool{
	apiPrefix + "/auth/login":  true,
	apiPrefix + "/auth/verify": true,
	"/oauth/register":          true,
	"/oauth/authorize":         true,
	"/oauth/token":             true,
	"/oauth/revoke":            true,
}

// limitUnauthenticated rejects requests to the unauthenticated endpoints once
// their client IP exhausts its budget.
func limitUnauthenticated(limits *Limits) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if limits == nil || limits.Unauthenticated == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodOptions && unauthenticatedPaths[r.URL.Path] {
				ip := ratelimit.ClientIP(r, limits.TrustedProxyHops)
				if ok, wait := limits.Unauthenticated.Allow(ip); !ok {
					writeRateLimited(w, wait)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitSessions rejects authenticated REST operations once the session
// exhausts its budget. It must run after authMiddleware.
func limitSessions(api huma.API, limiter *ratelimit.Limiter) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		session, ok := sessionFrom(ctx.Context())
		if !ok {
			next(ctx)
			return
		}
		if ok, wait := limiter.Allow(session.ID.String()); !ok {
			ctx.SetHeader("Retry-After", strconv.Itoa(ratelimit.RetryAfterSeconds(wait)))
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, rateLimitedMessage(wait))
			return
		}
		next(ctx)
	}
}

func rateLimitedMessage(wait time.Duration) string {
	return "rate limit exceeded: retry in " + strconv.Itoa(ratelimit.RetryAfterSeconds(wait)) + " seconds"
}

// writeRateLimited writes a 429 for the plain net/http endpoints. The body is
// a problem document that also carries the OAuth-style error fields, since
// OAuth clients read those.
func writeRateLimited(w http.ResponseWriter, wait time.Duration) {
	message := rateLimitedMessage(wait)
	w.Header().Set("Retry-After", strconv.Itoa(ratelimit.RetryAfterSeconds(wait)))
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":             http.StatusText(http.StatusTooManyRequests),
		"status":            http.StatusTooManyRequests,
		"detail":            message,
		"error":             "rate_limited",
		"error_description": message,
	})
}
