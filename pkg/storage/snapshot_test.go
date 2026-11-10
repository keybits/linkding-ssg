package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

func TestSnapshotSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "data", "bookmarks.json")

	bookmarks := []linkding.Bookmark{
		{
			ID:          1,
			URL:         "https://golang.org",
			Title:       "Go Programming Language",
			Description: "An open source programming language",
			TagNames:    []string{"golang", "programming"},
			DateCreated: time.Date(2025, 5, 10, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:          2,
			URL:         "https://linkding.link",
			Title:       "linkding",
			Description: "Self-hosted bookmark manager",
			TagNames:    []string{"selfhosted", "bookmarks"},
			DateCreated: time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC),
		},
	}

	err := SaveSnapshot(snapshotPath, "https://my-linkding.example.com", true, bookmarks)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	loaded, err := LoadSnapshot(snapshotPath)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}

	if loaded.TotalCount != 2 {
		t.Errorf("expected 2 bookmarks, got %d", loaded.TotalCount)
	}
	if loaded.ServerURL != "https://my-linkding.example.com" {
		t.Errorf("expected server url match, got %s", loaded.ServerURL)
	}
	if !loaded.SharedOnly {
		t.Errorf("expected shared_only to round-trip as true")
	}
	if len(loaded.Bookmarks) != 2 {
		t.Fatalf("expected 2 bookmarks in list, got %d", len(loaded.Bookmarks))
	}
	if loaded.Bookmarks[0].Title != "Go Programming Language" {
		t.Errorf("unexpected bookmark 0 title: %s", loaded.Bookmarks[0].Title)
	}
}

func TestLoadBareJSONArray(t *testing.T) {
	tempDir := t.TempDir()
	barePath := filepath.Join(tempDir, "bare.json")

	bareJSON := `[
		{"id": 99, "url": "https://example.com", "title": "Example", "tag_names": ["sample"], "date_created": "2024-01-01T00:00:00Z"}
	]`

	if err := os.WriteFile(barePath, []byte(bareJSON), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSnapshot(barePath)
	if err != nil {
		t.Fatalf("LoadSnapshot failed on bare array: %v", err)
	}

	if loaded.TotalCount != 1 || len(loaded.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(loaded.Bookmarks))
	}
	if loaded.Bookmarks[0].ID != 99 {
		t.Errorf("expected id 99, got %d", loaded.Bookmarks[0].ID)
	}
}

func TestMergeBookmarks(t *testing.T) {
	existing := []linkding.Bookmark{
		{ID: 1, Title: "One Old", DateCreated: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Title: "Two", DateCreated: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)},
	}

	newItems := []linkding.Bookmark{
		{ID: 3, Title: "Three Newest", DateCreated: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)},
		{ID: 1, Title: "One Updated", DateCreated: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	merged := MergeBookmarks(newItems, existing)

	if len(merged) != 3 {
		t.Fatalf("expected 3 bookmarks after merge, got %d", len(merged))
	}

	// Should be sorted descending: ID 3 (Jan 3), ID 2 (Jan 2), ID 1 (Jan 1)
	if merged[0].ID != 3 || merged[0].Title != "Three Newest" {
		t.Errorf("unexpected item 0: %+v", merged[0])
	}
	if merged[1].ID != 2 {
		t.Errorf("unexpected item 1: %+v", merged[1])
	}
	if merged[2].ID != 1 || merged[2].Title != "One Updated" {
		t.Errorf("unexpected item 2: %+v", merged[2])
	}
}
