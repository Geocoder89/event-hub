package handlers

import (
	"strings"
	"testing"
)

func TestIfNoneMatchMatches(t *testing.T) {
	current := `"abc123"`

	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "empty header", header: "", want: false},
		{name: "wildcard", header: "*", want: true},
		{name: "exact quoted", header: `"abc123"`, want: true},
		{name: "weak etag", header: `W/"abc123"`, want: true},
		{name: "list contains current", header: `"zzz", W/"abc123"`, want: true},
		{name: "mismatch", header: `"nope"`, want: false},
		{name: "missing quotes mismatch", header: `abc123`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ifNoneMatchMatches(tt.header, current)
			if got != tt.want {
				t.Fatalf("ifNoneMatchMatches(%q, %q)=%v want=%v", tt.header, current, got, tt.want)
			}
		})
	}
}

func TestBuildETag_Deterministic(t *testing.T) {
	payload := map[string]any{
		"count": 2,
		"items": []string{"a", "b"},
	}

	etag1, err := buildETag(payload)
	if err != nil {
		t.Fatalf("buildETag(payload) error: %v", err)
	}

	etag2, err := buildETag(payload)
	if err != nil {
		t.Fatalf("buildETag(payload) error: %v", err)
	}

	if etag1 != etag2 {
		t.Fatalf("expected deterministic etag, got %q and %q", etag1, etag2)
	}

	if !strings.HasPrefix(etag1, `"`) || !strings.HasSuffix(etag1, `"`) {
		t.Fatalf("expected quoted etag, got %q", etag1)
	}
}
