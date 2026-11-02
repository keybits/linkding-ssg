package ssg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

func TestGenerator_Generate(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "dist")

	bookmarks := []linkding.Bookmark{
		{
			ID:          1001,
			URL:         "https://example.com/one",
			Title:       "Sample One",
			Description: "First test bookmark",
			Notes:       "Some helpful notes",
			TagNames:    []string{"dev", "testing"},
			DateCreated: time.Date(2025, 4, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			ID:          42,
			URL:         "https://example.com/two",
			Title:       "Sample Two",
			Description: "Second test bookmark",
			TagNames:    []string{"dev"},
			DateCreated: time.Date(2025, 5, 20, 10, 0, 0, 0, time.UTC),
		},
	}
	pagesDir := filepath.Join(tempDir, "pages")
	_ = os.MkdirAll(pagesDir, 0755)
	_ = os.WriteFile(filepath.Join(pagesDir, "about.md"), []byte("# About Us\n\nThis is the about page."), 0644)

	gen, err := NewGenerator(GeneratorOptions{
		OutputDir:   outDir,
		RecentLimit: 10,
		PagesDir:    pagesDir,
	})
	if err != nil {
		t.Fatalf("NewGenerator failed: %v", err)
	}

	if err := gen.Generate(bookmarks); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify static CSS was copied
	cssPath := filepath.Join(outDir, "static", "style.css")
	if _, err := os.Stat(cssPath); err != nil {
		t.Errorf("static/style.css missing: %v", err)
	}

	// Verify index.html exists and has content
	indexPath := filepath.Join(outDir, "index.html")
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("reading index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), "Sample Two") {
		t.Errorf("index.html missing bookmark title 'Sample Two'")
	}

	// Verify timeline archive
	archivePath := filepath.Join(outDir, "archive", "index.html")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive/index.html missing: %v", err)
	}

	// Verify month archive (2025-05)
	monthPath := filepath.Join(outDir, "archive", "2025", "05", "index.html")
	if _, err := os.Stat(monthPath); err != nil {
		t.Errorf("archive/2025/05/index.html missing: %v", err)
	}

	// Verify tags
	tagPath := filepath.Join(outDir, "tags", "dev", "index.html")
	if _, err := os.Stat(tagPath); err != nil {
		t.Errorf("tags/dev/index.html missing: %v", err)
	}

	// Verify sharded bookmark permalinks
	// ID 42 -> b/000/42/index.html
	bm42Path := filepath.Join(outDir, "b", "000", "42", "index.html")
	bm42Bytes, err := os.ReadFile(bm42Path)
	if err != nil {
		t.Fatalf("b/000/42/index.html missing: %v", err)
	}
	if !strings.Contains(string(bm42Bytes), "data-pagefind-body") {
		t.Errorf("b/000/42/index.html missing data-pagefind-body attribute")
	}

	// ID 1001 -> b/001/1001/index.html
	bm1001Path := filepath.Join(outDir, "b", "001", "1001", "index.html")
	if _, err := os.Stat(bm1001Path); err != nil {
		t.Errorf("b/001/1001/index.html missing: %v", err)
	}

	// Verify search page exists
	searchPath := filepath.Join(outDir, "search", "index.html")
	if _, err := os.Stat(searchPath); err != nil {
		t.Errorf("search/index.html missing: %v", err)
	}

	// Verify custom about page was generated
	aboutPath := filepath.Join(outDir, "about", "index.html")
	aboutBytes, err := os.ReadFile(aboutPath)
	if err != nil {
		t.Errorf("about/index.html missing: %v", err)
	} else {
		if !strings.Contains(string(aboutBytes), "This is the about page.") {
			t.Errorf("about page missing content")
		}
	}

	// Verify index.html links to custom page in header
	if !strings.Contains(string(indexBytes), `<a href="/about/">About Us</a>`) {
		t.Errorf("header missing link to custom page About Us: %s", string(indexBytes))
	}
}
