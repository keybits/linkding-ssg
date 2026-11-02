package ssg

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

func TestGenerateRSS(t *testing.T) {
	tempDir := t.TempDir()
	rssPath := filepath.Join(tempDir, "feed.xml")

	bookmarks := []linkding.Bookmark{
		{
			ID:              1,
			URL:             "https://example.com/one",
			Title:           "First Post",
			Description:     "A great description",
			Notes:           "Private or public notes",
			TagNames:        []string{"golang", "rss"},
			PreviewImageURL: "https://example.com/image.png",
			DateCreated:     time.Date(2025, 10, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:          2,
			URL:         "https://example.com/two",
			Title:       "Second Post",
			Description: "Second description",
			DateCreated: time.Date(2025, 10, 14, 12, 0, 0, 0, time.UTC),
		},
	}

	err := GenerateRSS(rssPath, "Recent Bookmarks", "https://example.com/", "A test feed", bookmarks, 15)
	if err != nil {
		t.Fatalf("GenerateRSS failed: %v", err)
	}

	data, err := os.ReadFile(rssPath)
	if err != nil {
		t.Fatalf("reading rss file: %v", err)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatalf("unmarshaling generated rss XML: %v", err)
	}

	if feed.Channel.Title != "Recent Bookmarks" {
		t.Errorf("expected title 'Recent Bookmarks', got %q", feed.Channel.Title)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(feed.Channel.Items))
	}
	if feed.Channel.Items[0].Title != "First Post" {
		t.Errorf("expected item 0 title 'First Post', got %q", feed.Channel.Items[0].Title)
	}
	if feed.Channel.Items[0].Enclosure == nil || feed.Channel.Items[0].Enclosure.Type != "image/png" {
		t.Errorf("expected image/png enclosure, got %+v", feed.Channel.Items[0].Enclosure)
	}
	if !strings.Contains(feed.Channel.Items[0].Description, "Private or public notes") {
		t.Errorf("expected description to contain notes, got %s", feed.Channel.Items[0].Description)
	}
}

func TestGenerateRSS_Limit(t *testing.T) {
	tempDir := t.TempDir()
	rssPath := filepath.Join(tempDir, "feed.xml")

	bookmarks := make([]linkding.Bookmark, 30)
	now := time.Now().UTC()
	for i := range 30 {
		bookmarks[i] = linkding.Bookmark{
			ID:          int64(i + 1),
			URL:         "https://example.com",
			Title:       "Post",
			DateCreated: now,
		}
	}

	// Request limit of 15
	err := GenerateRSS(rssPath, "Limit Test", "https://example.com/", "A test feed", bookmarks, 15)
	if err != nil {
		t.Fatalf("GenerateRSS failed: %v", err)
	}

	data, err := os.ReadFile(rssPath)
	if err != nil {
		t.Fatalf("reading rss: %v", err)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}

	if len(feed.Channel.Items) != 15 {
		t.Errorf("expected exactly 15 items, got %d", len(feed.Channel.Items))
	}
}
