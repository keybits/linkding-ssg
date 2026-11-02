package ssg

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	PubDate     string        `xml:"pubDate"`
	GUID        RSSGUID       `xml:"guid"`
	Categories  []string      `xml:"category,omitempty"`
	Enclosure   *RSSEnclosure `xml:"enclosure,omitempty"`
}

type RSSGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr"`
}

type RSSEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// GenerateRSS creates an RSS 2.0 XML file from a slice of bookmarks.
func GenerateRSS(filePath, title, siteLink, description string, bookmarks []linkding.Bookmark, limit int) error {
	if limit <= 0 {
		limit = 15
	}
	if len(bookmarks) > limit {
		bookmarks = bookmarks[:limit]
	}

	var items []RSSItem
	for _, b := range bookmarks {
		descParts := []string{}
		if desc := b.DisplayDescription(); desc != "" {
			descParts = append(descParts, desc)
		}
		if b.Notes != "" {
			descParts = append(descParts, "<p><strong>Notes:</strong> "+b.Notes+"</p>")
		}
		if len(b.TagNames) > 0 {
			descParts = append(descParts, "<p><strong>Tags:</strong> #"+strings.Join(b.TagNames, " #")+"</p>")
		}

		itemDate := b.Date()
		pubDate := ""
		if !itemDate.IsZero() && itemDate.Year() > 1 {
			pubDate = itemDate.UTC().Format(time.RFC1123Z)
		}

		item := RSSItem{
			Title:       b.DisplayTitle(),
			Link:        b.URL,
			Description: strings.Join(descParts, "<br/>\n"),
			PubDate:     pubDate,
			GUID: RSSGUID{
				Value:       fmt.Sprintf("linkding-ssg:bookmark:%d", b.ID),
				IsPermaLink: false,
			},
			Categories: b.TagNames,
		}

		if b.PreviewImageURL != "" {
			mediaType := "image/jpeg"
			lower := strings.ToLower(b.PreviewImageURL)
			if strings.HasSuffix(lower, ".png") {
				mediaType = "image/png"
			} else if strings.HasSuffix(lower, ".webp") {
				mediaType = "image/webp"
			}
			item.Enclosure = &RSSEnclosure{
				URL:  b.PreviewImageURL,
				Type: mediaType,
			}
		}

		items = append(items, item)
	}

	feed := RSSFeed{
		Version: "2.0",
		Channel: RSSChannel{
			Title:         title,
			Link:          siteLink,
			Description:   description,
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:         items,
		},
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating RSS file %s: %w", filePath, err)
	}
	defer f.Close()

	if _, err := f.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"); err != nil {
		return err
	}

	encoder := xml.NewEncoder(f)
	encoder.Indent("", "  ")
	if err := encoder.Encode(feed); err != nil {
		return fmt.Errorf("encoding RSS XML: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return err
	}

	return nil
}

// FullURL resolves a relative path against baseURL.
func FullURL(baseURL, relPath string) string {
	relPath = strings.TrimLeft(relPath, "/")
	if baseURL == "" {
		return "/" + relPath
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + relPath
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + relPath
	return u.String()
}
