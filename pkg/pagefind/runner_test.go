package pagefind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePagefindBinary_Embedded(t *testing.T) {
	tempDir := t.TempDir()

	binPath, err := resolvePagefindBinary(IndexOptions{
		CacheDir: tempDir,
	})
	if err != nil {
		t.Fatalf("resolvePagefindBinary failed: %v", err)
	}

	if stat, err := os.Stat(binPath); err != nil || stat.Size() == 0 {
		t.Fatalf("expected extracted binary at %s, err: %v", binPath, err)
	}

	// Verify permissions are executable
	stat, err := os.Stat(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode()&0111 == 0 {
		t.Errorf("expected executable permissions on %s, got %v", binPath, stat.Mode())
	}

	// Calling again should reuse the cached file without error
	reusedPath, err := resolvePagefindBinary(IndexOptions{
		CacheDir: tempDir,
	})
	if err != nil {
		t.Fatalf("second resolvePagefindBinary failed: %v", err)
	}
	if reusedPath != binPath {
		t.Errorf("expected reused path %s, got %s", binPath, reusedPath)
	}
}

func TestIndexSite_EmbeddedExecution(t *testing.T) {
	tempDir := t.TempDir()
	siteDir := filepath.Join(tempDir, "site")
	if err := os.MkdirAll(siteDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create dummy HTML file for pagefind to index
	htmlContent := `<!DOCTYPE html><html><body><article data-pagefind-body><h1>Test Page</h1><p>Searchable content here.</p></article></body></html>`
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run IndexSite using embedded binary
	err := IndexSite(context.Background(), IndexOptions{
		SiteDir:  siteDir,
		CacheDir: filepath.Join(tempDir, "bin"),
	})
	if err != nil {
		t.Fatalf("IndexSite failed: %v", err)
	}

	// Verify that pagefind generated the pagefind output directory
	pagefindDir := filepath.Join(siteDir, "pagefind")
	if stat, err := os.Stat(pagefindDir); err != nil || !stat.IsDir() {
		t.Fatalf("pagefind directory was not created: %v", err)
	}
}
