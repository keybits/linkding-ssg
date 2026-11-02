package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

//go:embed static/* html/*
var EmbeddedFS embed.FS

// TemplateSet manages parsed HTML templates.
type TemplateSet struct {
	IndexTemplate     *template.Template
	ArchiveTemplate   *template.Template
	MonthTemplate     *template.Template
	TagsTemplate      *template.Template
	TagDetailTemplate *template.Template
	BookmarkTemplate  *template.Template
	SearchTemplate    *template.Template
}

func funcMap(baseURL string, siteTitle string) template.FuncMap {
	if siteTitle == "" {
		siteTitle = "Bookmarks"
	}
	cleanBase := strings.TrimRight(baseURL, "/")
	relURL := func(target string) string {
		target = strings.TrimLeft(target, "/")
		if cleanBase == "" {
			return "/" + target
		}
		return cleanBase + "/" + target
	}
	return template.FuncMap{
		"relURL": relURL,
		"siteTitle": func() string {
			return siteTitle
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() || t.Year() <= 1 {
				return ""
			}
			return t.UTC().Format("Jan 02, 2006")
		},
		"formatDateTime": func(t time.Time) string {
			if t.IsZero() || t.Year() <= 1 {
				return ""
			}
			return t.UTC().Format("2006-01-02 15:04:05 UTC")
		},
		"bookmarkDate": func(b linkding.Bookmark) time.Time {
			return b.Date()
		},
		"bookmarkURL": func(id int64) string {
			shard := id / 1000
			return relURL(fmt.Sprintf("b/%03d/%d/", shard, id))
		},
		"tagURL": func(tag string) string {
			safe := strings.ReplaceAll(tag, "/", "_")
			safe = strings.ReplaceAll(safe, "\\", "_")
			return relURL(fmt.Sprintf("tags/%s/", url.PathEscape(safe)))
		},
		"monthURL": func(year int, month time.Month) string {
			return relURL(fmt.Sprintf("archive/%04d/%02d/", year, int(month)))
		},
		"titleOrURL": func(b linkding.Bookmark) string {
			return b.DisplayTitle()
		},
	}
}

// LoadTemplates loads templates from overrideDir if provided/existing, or embedded FS.
func LoadTemplates(overrideDir string, baseURL string, siteTitle string) (*TemplateSet, error) {
	useDisk := false
	if overrideDir != "" {
		if stat, err := os.Stat(overrideDir); err == nil && stat.IsDir() {
			useDisk = true
		}
	}

	readTemplateFile := func(filename string) ([]byte, error) {
		if useDisk {
			diskPath := filepath.Join(overrideDir, filename)
			if content, err := os.ReadFile(diskPath); err == nil {
				return content, nil
			}
		}
		// Fall back to embedded FS
		return fs.ReadFile(EmbeddedFS, "html/"+filename)
	}

	parseOne := func(name string, files ...string) (*template.Template, error) {
		t := template.New(name).Funcs(funcMap(baseURL, siteTitle))
		for _, f := range files {
			content, err := readTemplateFile(f)
			if err != nil {
				return nil, fmt.Errorf("reading template %s: %w", f, err)
			}
			t, err = t.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("parsing template %s: %w", f, err)
			}
		}
		return t, nil
	}
	indexTmpl, err := parseOne("index.html", "base.html", "bookmark_card.html", "index.html")
	if err != nil {
		return nil, err
	}

	archiveTmpl, err := parseOne("archive.html", "base.html", "archive.html")
	if err != nil {
		return nil, err
	}

	monthTmpl, err := parseOne("month.html", "base.html", "bookmark_card.html", "month.html")
	if err != nil {
		return nil, err
	}

	tagsTmpl, err := parseOne("tags.html", "base.html", "tags.html")
	if err != nil {
		return nil, err
	}

	tagDetailTmpl, err := parseOne("tag_detail.html", "base.html", "bookmark_card.html", "tag_detail.html")
	if err != nil {
		return nil, err
	}

	bookmarkTmpl, err := parseOne("bookmark.html", "base.html", "bookmark_card.html", "bookmark.html")
	if err != nil {
		return nil, err
	}

	searchTmpl, err := parseOne("search.html", "base.html", "search.html")
	if err != nil {
		return nil, err
	}

	return &TemplateSet{
		IndexTemplate:     indexTmpl,
		ArchiveTemplate:   archiveTmpl,
		MonthTemplate:     monthTmpl,
		TagsTemplate:      tagsTmpl,
		TagDetailTemplate: tagDetailTmpl,
		BookmarkTemplate:  bookmarkTmpl,
		SearchTemplate:    searchTmpl,
	}, nil
}
