package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

// SaveSnapshot saves a snapshot atomically using a temporary file and atomic rename.
// sharedOnly records whether the snapshot contains only shared bookmarks.
func SaveSnapshot(filePath string, serverURL string, sharedOnly bool, bookmarks []linkding.Bookmark) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	snapshot := linkding.Snapshot{
		ExportedAt: time.Now().UTC(),
		ServerURL:  serverURL,
		TotalCount: len(bookmarks),
		SharedOnly: sharedOnly,
		Bookmarks:  bookmarks,
	}

	tempFile, err := os.CreateTemp(dir, "bookmarks-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup if rename or encode fails
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("encoding snapshot JSON: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", filePath, err)
	}

	return nil
}

// LoadSnapshot reads a bookmark snapshot from the specified JSON file.
// Supports both the envelope snapshot format {"exported_at": ..., "bookmarks": [...]}
// and a bare JSON array `[...]` of bookmarks for flexibility.
func LoadSnapshot(filePath string) (*linkding.Snapshot, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening snapshot file %s: %w", filePath, err)
	}
	defer file.Close()

	// Try reading as Snapshot envelope first
	var snapshot linkding.Snapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err == nil && len(snapshot.Bookmarks) > 0 {
		return &snapshot, nil
	}

	// Fallback: try reading as plain bookmark array
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("rewinding snapshot file: %w", err)
	}

	var bareList []linkding.Bookmark
	decoder = json.NewDecoder(file)
	if err := decoder.Decode(&bareList); err != nil {
		return nil, fmt.Errorf("parsing snapshot JSON: %w", err)
	}

	return &linkding.Snapshot{
		ExportedAt: time.Now().UTC(),
		TotalCount: len(bareList),
		Bookmarks:  bareList,
	}, nil
}

// MergeBookmarks merges newBookmarks with existingBookmarks.
// If an ID exists in both, the new bookmark replaces the old one (updating edits).
// All bookmarks are sorted descending by DateCreated (or DateAdded), then ID.
func MergeBookmarks(newBookmarks []linkding.Bookmark, existingBookmarks []linkding.Bookmark) []linkding.Bookmark {
	mergedMap := make(map[int64]linkding.Bookmark, len(existingBookmarks)+len(newBookmarks))

	// Populate existing bookmarks first
	for _, b := range existingBookmarks {
		mergedMap[b.ID] = b
	}

	// Overwrite or append with new bookmarks
	for _, b := range newBookmarks {
		mergedMap[b.ID] = b
	}

	result := make([]linkding.Bookmark, 0, len(mergedMap))
	for _, b := range mergedMap {
		result = append(result, b)
	}

	// Sort newest first
	sortBookmarksDescending(result)
	return result
}

func sortBookmarksDescending(bms []linkding.Bookmark) {
	sort.Slice(bms, func(i, j int) bool {
		return isBookmarkAfter(bms[i], bms[j])
	})
}

func isBookmarkAfter(a, b linkding.Bookmark) bool {
	aTime := a.DateCreated
	if aTime.IsZero() {
		aTime = a.DateAdded
	}
	bTime := b.DateCreated
	if bTime.IsZero() {
		bTime = b.DateAdded
	}
	if aTime.Equal(bTime) {
		return a.ID > b.ID
	}
	return aTime.After(bTime)
}
