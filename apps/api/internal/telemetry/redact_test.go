package telemetry

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestRedact(t *testing.T) {
	tests := map[string]struct {
		in   string
		gone []string
	}{
		"email":        {"550 5.1.1 <ana.perez+x@example.co.uk>: recipient rejected", []string{"ana.perez", "example.co.uk"}},
		"magic link":   {"GET /login?token=abc123&next=/home failed", []string{"abc123"}},
		"oauth query":  {"redirect https://x.test/cb?code=SECRETCODE&state=SECRETSTATE", []string{"SECRETCODE", "SECRETSTATE"}},
		"search term":  {"fetch /transactions?q=rent+payment", []string{"rent+payment"}},
		"bearer token": {"upstream said: Bearer abcdefghijklmnop.qrs", []string{"abcdefghijklmnop"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Redact(tt.in)
			for _, secret := range tt.gone {
				if strings.Contains(got, secret) {
					t.Fatalf("Redact(%q) = %q still contains %q", tt.in, got, secret)
				}
			}
			if !strings.Contains(got, redacted) {
				t.Fatalf("Redact(%q) = %q has no marker", tt.in, got)
			}
		})
	}
}

func TestRedactKeepsPlainText(t *testing.T) {
	in := "GET /api/v1/accounts 200 12ms"
	if got := Redact(in); got != in {
		t.Fatalf("Redact changed %q to %q", in, got)
	}
}

func TestLogHandlerRedactsExportedRecordsOnly(t *testing.T) {
	logger, stdout, exporter := newTestLogger(t, slog.LevelInfo)

	err := fmt.Errorf("send login code: %w", errors.New("resend responded 422: invalid to ana@example.com"))
	logger.With("recipient", "luis@example.com").Error("request login for ana@example.com", "error", err, "url", "/cb?code=SECRETCODE", slog.Group("req", "token", "x", "note", "tok=1 token=SECRETTOKEN"))

	if !strings.Contains(stdout.String(), "ana@example.com") {
		t.Fatalf("local output should keep the original record: %s", stdout)
	}

	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	if len(exporter.records) != 1 {
		t.Fatalf("exported %d records, want 1", len(exporter.records))
	}
	var dump strings.Builder
	record := exporter.records[0]
	dump.WriteString(record.Body().String())
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		dump.WriteString(" " + string(kv.Key) + "=" + kv.Value.Emit())
		return true
	})
	for _, secret := range []string{"ana@example.com", "luis@example.com", "SECRETCODE", "SECRETTOKEN"} {
		if strings.Contains(dump.String(), secret) {
			t.Fatalf("exported record leaks %q: %s", secret, dump.String())
		}
	}
	if !strings.Contains(dump.String(), "send login code") {
		t.Fatalf("redaction dropped the error context: %s", dump.String())
	}
}
