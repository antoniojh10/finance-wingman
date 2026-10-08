package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTestLimiter(perMinute, burst int) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := New(perMinute, burst)
	l.SetClock(clock.now)
	return l, clock
}

func TestAllowBurstThenBlocks(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter(60, 3)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	ok, wait := l.Allow("a")
	if ok {
		t.Fatal("fourth request should be blocked")
	}
	if wait != time.Second {
		t.Fatalf("expected a 1s wait at 1 token/s, got %v", wait)
	}
	if got := RetryAfterSeconds(wait); got != 1 {
		t.Fatalf("retry after: %d", got)
	}
}

func TestKeysAreIndependent(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter(60, 1)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("a should be allowed")
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("b should not be affected by a")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("a should be blocked")
	}
}

func TestRefillsOverTime(t *testing.T) {
	t.Parallel()
	l, clock := newTestLimiter(60, 2)
	l.Allow("a")
	l.Allow("a")
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("bucket should be empty")
	}
	clock.t = clock.t.Add(1500 * time.Millisecond)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("one token should have refilled")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("only one token should have refilled")
	}
	// The bucket never holds more than the burst.
	clock.t = clock.t.Add(time.Hour)
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d after a long pause should pass", i)
		}
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("burst must cap the refill")
	}
}

func TestWaitGrowsWithDeficit(t *testing.T) {
	t.Parallel()
	l, _ := newTestLimiter(6, 1) // one token every 10s
	l.Allow("a")
	_, wait := l.Allow("a")
	if wait != 10*time.Second {
		t.Fatalf("wait: %v", wait)
	}
	if got := RetryAfterSeconds(wait); got != 10 {
		t.Fatalf("retry after: %d", got)
	}
	if got := RetryAfterSeconds(1500 * time.Millisecond); got != 2 {
		t.Fatalf("retry after should round up, got %d", got)
	}
}

func TestIdleBucketsAreSwept(t *testing.T) {
	t.Parallel()
	l, clock := newTestLimiter(60, 1)
	for _, key := range []string{"a", "b", "c"} {
		l.Allow(key)
	}
	clock.t = clock.t.Add(2 * time.Minute)
	l.Allow("d")
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) != 1 {
		t.Fatalf("expected only the active bucket to remain, got %d", len(l.buckets))
	}
}

func TestNilLimiterAllowsEverything(t *testing.T) {
	t.Parallel()
	if New(0, 10) != nil {
		t.Fatal("a zero rate should disable the limiter")
	}
	var l *Limiter
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("nil limiter must allow")
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		remote string
		xff    []string
		hops   int
		want   string
	}{
		{"peer without proxy", "203.0.113.9:4000", nil, 0, "203.0.113.9"},
		{"forwarded header ignored when untrusted", "203.0.113.9:4000", []string{"1.2.3.4"}, 0, "203.0.113.9"},
		{"one hop takes the rightmost entry", "10.0.0.1:4000", []string{"6.6.6.6, 198.51.100.7"}, 1, "198.51.100.7"},
		{"forged leftmost entries are ignored", "10.0.0.1:4000", []string{"6.6.6.6", "7.7.7.7, 198.51.100.7"}, 1, "198.51.100.7"},
		{"two hops", "10.0.0.1:4000", []string{"198.51.100.7, 10.1.1.1"}, 2, "198.51.100.7"},
		{"missing header falls back to the peer", "10.0.0.1:4000", nil, 1, "10.0.0.1"},
		{"too few entries falls back to the peer", "10.0.0.1:4000", []string{"198.51.100.7"}, 2, "10.0.0.1"},
		{"garbage entry falls back to the peer", "10.0.0.1:4000", []string{"not-an-ip"}, 1, "10.0.0.1"},
		{"ipv6 collapses to its /64", "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:4000", nil, 0, "2001:db8:1:2::/64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			for _, h := range tt.xff {
				r.Header.Add("X-Forwarded-For", h)
			}
			if got := ClientIP(r, tt.hops); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForwardedClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		secret string
		header map[string]string
		want   string
		ok     bool
	}{
		{"valid", "s3cret", map[string]string{"X-Client-IP": "198.51.100.7", "X-Client-IP-Secret": "s3cret"}, "198.51.100.7", true},
		{"ipv6 collapses to its /64", "s3cret", map[string]string{"X-Client-IP": "2001:db8:1:2::9", "X-Client-IP-Secret": "s3cret"}, "2001:db8:1:2::/64", true},
		{"wrong secret", "s3cret", map[string]string{"X-Client-IP": "198.51.100.7", "X-Client-IP-Secret": "nope"}, "", false},
		{"missing secret", "s3cret", map[string]string{"X-Client-IP": "198.51.100.7"}, "", false},
		{"not configured", "", map[string]string{"X-Client-IP": "198.51.100.7", "X-Client-IP-Secret": ""}, "", false},
		{"malformed address", "s3cret", map[string]string{"X-Client-IP": "garbage", "X-Client-IP-Secret": "s3cret"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.header {
				r.Header.Set(k, v)
			}
			got, ok := ForwardedClientIP(r, tt.secret)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
