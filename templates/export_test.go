package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportAssets(t *testing.T) {
	tempDir := t.TempDir()

	err := ExportAssets(tempDir)
	if err != nil {
		t.Fatalf("ExportAssets failed: %v", err)
	}

	// Verify templates were exported
	tmplFile := filepath.Join(tempDir, "templates", "base.html")
	content, err := os.ReadFile(tmplFile)
	if err != nil {
		t.Fatalf("reading exported base.html: %v", err)
	}
	if !strings.Contains(string(content), "<!DOCTYPE html>") {
		t.Errorf("base.html missing DOCTYPE")
	}

	// Verify static assets were exported
	cssFile := filepath.Join(tempDir, "static", "style.css")
	cssContent, err := os.ReadFile(cssFile)
	if err != nil {
		t.Fatalf("reading exported style.css: %v", err)
	}
	if !strings.Contains(string(cssContent), "--font-sans") {
		t.Errorf("style.css missing variable")
	}
}

func TestLoadTemplates_Override(t *testing.T) {
	tempDir := t.TempDir()

	// Export standard assets first
	if err := ExportAssets(tempDir); err != nil {
		t.Fatal(err)
	}

	// Modify base.html in custom dir
	tmplDir := filepath.Join(tempDir, "templates")
	basePath := filepath.Join(tmplDir, "base.html")
	customHTML := `{{define "base"}}CUSTOM_LAYOUT:{{template "content" .}}{{end}}`
	if err := os.WriteFile(basePath, []byte(customHTML), 0644); err != nil {
		t.Fatal(err)
	}

	set, err := LoadTemplates(tmplDir, "", "My Site")
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	var sb strings.Builder
	if err := set.IndexTemplate.ExecuteTemplate(&sb, "base", map[string]any{}); err != nil {
		t.Fatalf("ExecuteTemplate failed: %v", err)
	}

	if !strings.HasPrefix(sb.String(), "CUSTOM_LAYOUT:") {
		t.Errorf("expected custom layout prefix, got %q", sb.String())
	}
}
