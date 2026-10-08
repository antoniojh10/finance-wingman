// Package ratelimit implements an in-process token bucket limiter and the
// helpers to key it by client IP.
//
// State lives in memory, so limits apply per API instance: with several
// replicas each one enforces its own budget. Production runs a single
// instance; revisit (e.g. a shared store) before scaling out.
package ratelimit

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a token bucket per key. It is safe for concurrent use.
type Limiter struct {
	rate  float64 // tokens added per second
	burst float64

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
	now       func() time.Time
}

// New returns a limiter that refills perMinute tokens per minute per key and
// allows bursts of up to burst requests. A perMinute below 1 returns nil,
// which Allow treats as "no limit".
func New(perMinute, burst int) *Limiter {
	if perMinute < 1 {
		return nil
	}
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		rate:    float64(perMinute) / 60,
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// SetClock overrides the time source; intended for tests.
func (l *Limiter) SetClock(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = now
}

// Allow takes a token for key. When none is left it reports how long to wait
// before one is available. A nil Limiter allows everything.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
	return false, wait.Round(time.Millisecond)
}

// sweep drops buckets that have been idle long enough to be full again, so
// the map does not grow with every key ever seen. Callers hold l.mu.
func (l *Limiter) sweep(now time.Time) {
	idle := time.Duration(l.burst / l.rate * float64(time.Second))
	if idle < time.Minute {
		idle = time.Minute
	}
	if now.Sub(l.lastSweep) < idle {
		return
	}
	l.lastSweep = now
	for key, b := range l.buckets {
		if now.Sub(b.last) >= idle {
			delete(l.buckets, key)
		}
	}
}

// RetryAfterSeconds renders a wait as whole seconds, rounded up, for the
// Retry-After header.
func RetryAfterSeconds(wait time.Duration) int {
	secs := int((wait + time.Second - 1) / time.Second)
	return max(secs, 1)
}

// ClientIP returns the address to rate limit a request by.
//
// With trustedHops == 0 it is the TCP peer. Otherwise the server sits behind
// that many reverse proxies, each appending the address it saw to
// X-Forwarded-For, so the trustworthy entry is the one trustedHops from the
// right: everything to its left is client supplied and can be forged. If the
// header has fewer entries than expected the request did not come through the
// proxy and the TCP peer is used. IPv6 addresses collapse to their /64 so one
// host cannot dodge the limit by rotating addresses inside its prefix.
func ClientIP(r *http.Request, trustedHops int) string {
	ip := peerIP(r)
	if trustedHops > 0 {
		var hops []string
		for _, h := range r.Header.Values("X-Forwarded-For") {
			for _, part := range strings.Split(h, ",") {
				hops = append(hops, strings.TrimSpace(part))
			}
		}
		if len(hops) >= trustedHops {
			if parsed := net.ParseIP(hops[len(hops)-trustedHops]); parsed != nil {
				ip = parsed
			}
		}
	}
	if ip == nil {
		return r.RemoteAddr
	}
	return keyFor(ip)
}

// ForwardedClientIP returns the client address that the web server reports in
// X-Client-IP, accepted only when X-Client-IP-Secret matches secret. The web
// server reaches the API over the private network, where no proxy appends the
// browser's address, so it vouches for it with the shared secret. ok is false
// when no secret is configured or the headers are missing, wrong or malformed.
func ForwardedClientIP(r *http.Request, secret string) (key string, ok bool) {
	if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Client-IP-Secret")), []byte(secret)) != 1 {
		return "", false
	}
	ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Client-IP")))
	if ip == nil {
		return "", false
	}
	return keyFor(ip), true
}

func keyFor(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func peerIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}
