package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/geocoder89/eventhub/internal/http/handlers"
)

func TestBuildCursorPageResponse(t *testing.T) {
	next := "abc123"
	total := 42

	resp := handlers.BuildCursorPageResponse(20, []string{"a", "b"}, true, &next, &total)

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var decoded struct {
		Limit      int      `json:"limit"`
		Count      int      `json:"count"`
		Items      []string `json:"items"`
		HasMore    bool     `json:"hasMore"`
		NextCursor *string  `json:"nextCursor"`
		Total      *int     `json:"total"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if decoded.Limit != 20 {
		t.Fatalf("limit=%d want=20", decoded.Limit)
	}
	if decoded.Count != 2 {
		t.Fatalf("count=%d want=2", decoded.Count)
	}
	if len(decoded.Items) != 2 {
		t.Fatalf("items len=%d want=2", len(decoded.Items))
	}
	if !decoded.HasMore {
		t.Fatalf("hasMore=false want=true")
	}
	if decoded.NextCursor == nil || *decoded.NextCursor != next {
		t.Fatalf("nextCursor=%v want=%s", decoded.NextCursor, next)
	}
	if decoded.Total == nil || *decoded.Total != total {
		t.Fatalf("total=%v want=%d", decoded.Total, total)
	}
}
