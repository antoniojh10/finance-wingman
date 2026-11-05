package httpapi

import "testing"

func TestBuildVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"no metadata", nil, Version},
		{"app version", map[string]string{"APP_VERSION": "abc1234"}, Version + "+abc1234"},
		{"commit sha is shortened", map[string]string{"RAILWAY_GIT_COMMIT_SHA": "0123456789abcdef"}, Version + "+0123456"},
		{"app version wins", map[string]string{"APP_VERSION": "v7", "RAILWAY_GIT_COMMIT_SHA": "0123456789abcdef"}, Version + "+v7"},
		{"blank values are ignored", map[string]string{"APP_VERSION": "  ", "RAILWAY_GIT_COMMIT_SHA": ""}, Version},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := BuildVersion(func(k string) string { return tt.env[k] }); got != tt.want {
				t.Errorf("BuildVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
