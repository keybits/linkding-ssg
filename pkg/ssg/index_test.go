package ssg

import (
	"strings"
	"testing"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

func TestShardedIDPath(t *testing.T) {
	tests := []struct {
		id       int64
		expected string
	}{
		{id: 5, expected: "b/000/5/"},
		{id: 999, expected: "b/000/999/"},
		{id: 1000, expected: "b/001/1000/"},
		{id: 12345, expected: "b/012/12345/"},
		{id: 30501, expected: "b/030/30501/"},
	}

	for _, tt := range tests {
		got := ShardedIDPath(tt.id)
		if got != tt.expected {
			t.Errorf("ShardedIDPath(%d) = %q, expected %q", tt.id, got, tt.expected)
		}
	}
}

func TestBuildIndex(t *testing.T) {
	bookmarks := []linkding.Bookmark{
		{
			ID:          1,
			URL:         "https://example.com/1",
			Title:       "Post 1",
			TagNames:    []string{"go", "web"},
			DateCreated: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:          2,
			URL:         "https://example.com/2",
			Title:       "Post 2",
			TagNames:    []string{"go"},
			DateCreated: time.Date(2025, 2, 20, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:          3,
			URL:         "https://example.com/3",
			Title:       "Post 3",
			TagNames:    []string{"rust"},
			DateCreated: time.Date(2024, 12, 5, 10, 0, 0, 0, time.UTC),
		},
	}

	idx := BuildIndex(bookmarks, 2)

	if len(idx.Recent) != 2 {
		t.Fatalf("expected 2 recent bookmarks, got %d", len(idx.Recent))
	}
	// Post 2 is newest (Feb 2025)
	if idx.Recent[0].ID != 2 {
		t.Errorf("expected newest bookmark to be ID 2, got %d", idx.Recent[0].ID)
	}

	// Verify Months
	if len(idx.Months) != 3 {
		t.Fatalf("expected 3 month buckets, got %d", len(idx.Months))
	}
	if idx.Months[0].Key != "2025-02" {
		t.Errorf("expected first month to be 2025-02, got %s", idx.Months[0].Key)
	}

	// Verify Years
	if len(idx.Years) != 2 {
		t.Fatalf("expected 2 years (2025, 2024), got %d", len(idx.Years))
	}
	if idx.Years[0].Year != 2025 || idx.Years[0].Total != 2 {
		t.Errorf("expected 2025 with 2 items, got %+v", idx.Years[0])
	}

	// Verify Tags: 'go' has 2 bookmarks, should be first
	if len(idx.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(idx.Tags))
	}
	if idx.Tags[0].Name != "go" || len(idx.Tags[0].Bookmarks) != 2 {
		t.Errorf("expected top tag to be 'go' with 2 items, got %+v", idx.Tags[0])
	}

	// Verify path formatting
	if !strings.HasPrefix(idx.Tags[0].Path, "tags/go/") {
		t.Errorf("unexpected tag path: %s", idx.Tags[0].Path)
	}
}
