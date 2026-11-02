package ssg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPages(t *testing.T) {
	tempDir := t.TempDir()
	pagesDir := filepath.Join(tempDir, "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Create about.md with frontmatter
	aboutMD := `---
title: "About This Archive"
---
Welcome to my bookmarks archive!

Here you will find links saved over the years.
`
	if err := os.WriteFile(filepath.Join(pagesDir, "about.md"), []byte(aboutMD), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create colophon.md with # Heading title
	colophonMD := `# Colophon & Tools

Built with **linkding-ssg**, Go, and Pagefind.
`
	if err := os.WriteFile(filepath.Join(pagesDir, "colophon.md"), []byte(colophonMD), 0644); err != nil {
		t.Fatal(err)
	}

	pages, err := LoadPages(pagesDir)
	if err != nil {
		t.Fatalf("LoadPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	// Sorted alphabetically by Title: "About This Archive", "Colophon & Tools"
	if pages[0].Slug != "about" || pages[0].Title != "About This Archive" {
		t.Errorf("unexpected page 0: %+v", pages[0])
	}
	if !strings.Contains(string(pages[0].ContentHTML), "<p>Welcome to my bookmarks archive!</p>") {
		t.Errorf("expected HTML content in page 0, got %s", pages[0].ContentHTML)
	}

	if pages[1].Slug != "colophon" || pages[1].Title != "Colophon & Tools" {
		t.Errorf("unexpected page 1: %+v", pages[1])
	}
	if !strings.Contains(string(pages[1].ContentHTML), "<strong>linkding-ssg</strong>") {
		t.Errorf("expected rendered strong tag, got %s", pages[1].ContentHTML)
	}
}

func TestLoadPages_MissingDir(t *testing.T) {
	pages, err := LoadPages("non-existent-pages-dir")
	if err != nil {
		t.Fatalf("expected nil error for missing pages dir, got %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(pages))
	}
}
