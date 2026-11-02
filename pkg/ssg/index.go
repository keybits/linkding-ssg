package ssg

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

// SafeTagSlug transforms any arbitrary tag name into a safe filesystem and URL directory slug.
func SafeTagSlug(tag string) string {
	// Replace forward/backward slashes to prevent subfolder creation issues
	s := strings.ReplaceAll(tag, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return url.PathEscape(s)
}

// ShardedIDPath returns the relative URL path for a bookmark's permalink.
// To avoid directory clutter with 30k+ files, we shard by bookmark ID.
// For example, ID 12345 -> "b/012/12345/"
// For ID < 1000, e.g. 42 -> "b/000/42/"
func ShardedIDPath(id int64) string {
	shard := id / 1000
	return fmt.Sprintf("b/%03d/%d/", shard, id)
}

// ShardedIDFilePath returns the relative OS file path for the bookmark's index.html.
func ShardedIDFilePath(id int64) string {
	shard := id / 1000
	return filepath.Join("b", fmt.Sprintf("%03d", shard), fmt.Sprintf("%d", id), "index.html")
}

// MonthBucket aggregates bookmarks for a given YYYY-MM.
type MonthBucket struct {
	Year      int
	Month     time.Month
	Key       string // "2026-10"
	Path      string // "archive/2026/10/"
	Bookmarks []linkding.Bookmark
}

func (m MonthBucket) MonthName() string {
	return m.Month.String()
}

// YearBucket aggregates month buckets for a given year.
type YearBucket struct {
	Year   int
	Months []*MonthBucket
	Total  int
}

// TagBucket holds all bookmarks tagged with a specific tag name.
type TagBucket struct {
	Name      string
	Path      string // "tags/my-tag/"
	Bookmarks []linkding.Bookmark
}

// ArchiveIndex contains the processed and sorted structures ready for static rendering.
type ArchiveIndex struct {
	AllBookmarks []linkding.Bookmark
	Recent       []linkding.Bookmark
	Years        []*YearBucket
	Months       []*MonthBucket
	MonthsByKey  map[string]*MonthBucket
	Tags         []*TagBucket
	TagsByName   map[string]*TagBucket
}

// BuildIndex processes raw bookmarks into sorted temporal and tag structures.
func BuildIndex(bookmarks []linkding.Bookmark, recentLimit int) *ArchiveIndex {
	if recentLimit <= 0 {
		recentLimit = 50
	}

	// Copy slice so we don't mutate external input
	sorted := make([]linkding.Bookmark, len(bookmarks))
	copy(sorted, bookmarks)

	// Sort chronologically descending (newest first)
	sort.Slice(sorted, func(i, j int) bool {
		tI := sorted[i].Date()
		tJ := sorted[j].Date()
		if tI.Equal(tJ) {
			return sorted[i].ID > sorted[j].ID
		}
		return tI.After(tJ)
	})

	recentCount := recentLimit
	if len(sorted) < recentCount {
		recentCount = len(sorted)
	}
	recent := sorted[:recentCount]

	monthsByKey := make(map[string]*MonthBucket)
	tagsByName := make(map[string]*TagBucket)

	for _, b := range sorted {
		created := b.Date().UTC()
		monthKey := created.Format("2006-01")
		bucket, exists := monthsByKey[monthKey]
		if !exists {
			bucket = &MonthBucket{
				Year:  created.Year(),
				Month: created.Month(),
				Key:   monthKey,
				Path:  fmt.Sprintf("archive/%04d/%02d/", created.Year(), int(created.Month())),
			}
			monthsByKey[monthKey] = bucket
		}
		bucket.Bookmarks = append(bucket.Bookmarks, b)

		for _, rawTag := range b.TagNames {
			tag := strings.TrimSpace(rawTag)
			if tag == "" {
				continue
			}
			tagBucket, tagExists := tagsByName[tag]
			if !tagExists {
				tagBucket = &TagBucket{
					Name: tag,
					Path: fmt.Sprintf("tags/%s/", SafeTagSlug(tag)),
				}
				tagsByName[tag] = tagBucket
			}
			tagBucket.Bookmarks = append(tagBucket.Bookmarks, b)
		}
	}

	// Collect and sort MonthBuckets descending (newest month first)
	allMonths := make([]*MonthBucket, 0, len(monthsByKey))
	for _, m := range monthsByKey {
		allMonths = append(allMonths, m)
	}
	sort.Slice(allMonths, func(i, j int) bool {
		return allMonths[i].Key > allMonths[j].Key
	})

	// Group months into YearBuckets
	yearsMap := make(map[int]*YearBucket)
	for _, m := range allMonths {
		yb, exists := yearsMap[m.Year]
		if !exists {
			yb = &YearBucket{
				Year: m.Year,
			}
			yearsMap[m.Year] = yb
		}
		yb.Months = append(yb.Months, m)
		yb.Total += len(m.Bookmarks)
	}

	allYears := make([]*YearBucket, 0, len(yearsMap))
	for _, y := range yearsMap {
		allYears = append(allYears, y)
	}
	sort.Slice(allYears, func(i, j int) bool {
		return allYears[i].Year > allYears[j].Year
	})

	// Collect and sort tags alphabetically
	allTags := make([]*TagBucket, 0, len(tagsByName))
	for _, t := range tagsByName {
		allTags = append(allTags, t)
	}
	sort.Slice(allTags, func(i, j int) bool {
		// Primary: higher count first, secondary: alphabetical
		if len(allTags[i].Bookmarks) != len(allTags[j].Bookmarks) {
			return len(allTags[i].Bookmarks) > len(allTags[j].Bookmarks)
		}
		return allTags[i].Name < allTags[j].Name
	})

	return &ArchiveIndex{
		AllBookmarks: sorted,
		Recent:       recent,
		Years:        allYears,
		Months:       allMonths,
		MonthsByKey:  monthsByKey,
		Tags:         allTags,
		TagsByName:   tagsByName,
	}
}
