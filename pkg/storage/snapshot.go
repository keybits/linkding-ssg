package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

// SaveSnapshot saves a snapshot atomically using a temporary file and atomic rename.
func SaveSnapshot(filePath string, serverURL string, bookmarks []linkding.Bookmark) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	snapshot := linkding.Snapshot{
		ExportedAt: time.Now().UTC(),
		ServerURL:  serverURL,
		TotalCount: len(bookmarks),
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
