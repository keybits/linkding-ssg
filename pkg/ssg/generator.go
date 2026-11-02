package ssg

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
	"github.com/lmorchard/linkding-ssg/templates"
)

// GeneratorOptions configures the static site generator.
type GeneratorOptions struct {
	OutputDir    string
	BaseURL      string
	RecentLimit  int
	TemplatesDir string // optional template directory override
}

// Generator builds the static HTML site from linkding bookmarks.
type Generator struct {
	opts      GeneratorOptions
	templates *templates.TemplateSet
}

// NewGenerator initializes a new Generator.
func NewGenerator(opts GeneratorOptions) (*Generator, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = "dist"
	}
	if opts.RecentLimit <= 0 {
		opts.RecentLimit = 50
	}

	tmplSet, err := templates.LoadTemplates(opts.TemplatesDir, opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return &Generator{
		opts:      opts,
		templates: tmplSet,
	}, nil
}

// Generate renders the full site into OutputDir.
func (g *Generator) Generate(bookmarks []linkding.Bookmark) error {
	start := time.Now()

	// Clean or ensure output directory exists
	if err := os.MkdirAll(g.opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// 1. Copy static assets (CSS, JS, icons)
	log.Printf("Copying static assets to %s/static...", g.opts.OutputDir)
	if err := g.copyStaticAssets(); err != nil {
		return fmt.Errorf("copying static assets: %w", err)
	}

	// 2. Build indexed data structures
	log.Printf("Indexing %d bookmarks across years, months, and tags...", len(bookmarks))
	idx := BuildIndex(bookmarks, g.opts.RecentLimit)
	log.Printf("Indexed %d years, %d months, and %d unique tags.", len(idx.Years), len(idx.Months), len(idx.Tags))

	// 3. Render Home / Recent Bookmarks (index.html)
	log.Printf("Rendering recent bookmarks index (latest %d links)...", len(idx.Recent))
	if err := g.renderFile("index.html", g.templates.IndexTemplate, map[string]any{
		"Recent": idx.Recent,
		"Total":  len(idx.AllBookmarks),
	}); err != nil {
		return fmt.Errorf("rendering index: %w", err)
	}

	// 4. Render Timeline Archive (archive/index.html)
	log.Println("Rendering timeline archive overview...")
	if err := g.renderFile(filepath.Join("archive", "index.html"), g.templates.ArchiveTemplate, map[string]any{
		"Years": idx.Years,
	}); err != nil {
		return fmt.Errorf("rendering archive: %w", err)
	}

	// 5. Render Month Archives (archive/YYYY/MM/index.html)
	log.Printf("Rendering %d monthly archive pages...", len(idx.Months))
	for _, m := range idx.Months {
		monthPath := filepath.Join("archive", fmt.Sprintf("%04d", m.Year), fmt.Sprintf("%02d", int(m.Month)), "index.html")
		if err := g.renderFile(monthPath, g.templates.MonthTemplate, map[string]any{
			"Month": m,
		}); err != nil {
			return fmt.Errorf("rendering month %s: %w", m.Key, err)
		}
	}

	// 6. Render Tags index (tags/index.html)
	log.Printf("Rendering tag directory (%d tags)...", len(idx.Tags))
	if err := g.renderFile(filepath.Join("tags", "index.html"), g.templates.TagsTemplate, map[string]any{
		"Tags": idx.Tags,
	}); err != nil {
		return fmt.Errorf("rendering tags: %w", err)
	}

	// 7. Render Individual Tag Archives (tags/{tag}/index.html)
	log.Printf("Rendering %d tag detail pages...", len(idx.Tags))
	for _, tag := range idx.Tags {
		tagPath := filepath.Join("tags", tag.Name, "index.html")
		if err := g.renderFile(tagPath, g.templates.TagDetailTemplate, map[string]any{
			"Tag": tag,
		}); err != nil {
			return fmt.Errorf("rendering tag detail %s: %w", tag.Name, err)
		}
	}

	// 8. Render Sharded Individual Bookmark Permalinks (b/{shard}/{id}/index.html)
	totalBookmarks := len(idx.AllBookmarks)
	log.Printf("Rendering %d individual sharded bookmark permalinks...", totalBookmarks)
	reportInterval := 5000
	if totalBookmarks < 5000 {
		reportInterval = 1000
	}
	for i, b := range idx.AllBookmarks {
		if i > 0 && i%reportInterval == 0 {
			percent := float64(i) / float64(totalBookmarks) * 100
			log.Printf("Rendered %d / %d permalinks (%.1f%%)...", i, totalBookmarks, percent)
		}
		bmRelPath := ShardedIDFilePath(b.ID)
		if err := g.renderFile(bmRelPath, g.templates.BookmarkTemplate, map[string]any{
			"Bookmark": b,
		}); err != nil {
			return fmt.Errorf("rendering bookmark %d: %w", b.ID, err)
		}
	}

	// 9. Render Search Page (search/index.html)
	log.Println("Rendering search page...")
	if err := g.renderFile(filepath.Join("search", "index.html"), g.templates.SearchTemplate, map[string]any{}); err != nil {
		return fmt.Errorf("rendering search page: %w", err)
	}

	elapsed := time.Since(start)
	totalPages := 1 /*index*/ + 1 /*archive*/ + len(idx.Months) + 1 /*tags*/ + len(idx.Tags) + len(idx.AllBookmarks) + 1 /*search*/
	log.Printf("Static generation finished in %v: wrote %d HTML pages to %s.", elapsed.Round(time.Millisecond), totalPages, g.opts.OutputDir)
	return nil
}

func (g *Generator) renderFile(relPath string, tmpl *template.Template, data any) error {
	fullPath := filepath.Join(g.opts.OutputDir, relPath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", fullPath, err)
	}
	defer f.Close()

	if err := tmpl.ExecuteTemplate(f, "base", data); err != nil {
		return fmt.Errorf("executing template for %s: %w", relPath, err)
	}

	return nil
}

func (g *Generator) copyStaticAssets() error {
	staticSub, err := fs.Sub(templates.EmbeddedFS, "static")
	if err != nil {
		return fmt.Errorf("getting static sub-FS: %w", err)
	}

	targetDir := filepath.Join(g.opts.OutputDir, "static")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating static target dir: %w", err)
	}

	return fs.WalkDir(staticSub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		destPath := filepath.Join(targetDir, path)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		srcFile, err := staticSub.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
