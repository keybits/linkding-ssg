package ssg

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

// SyncOptions configures local asset caching and copying.
type SyncOptions struct {
	OutputDir   string
	DataDir     string // e.g. "data"
	AssetDirs   []string // e.g. ["/home/.../data/favicons", "/home/.../data/previews"]
	ApiToken    string
	Concurrency int
}

// SyncAssets ensures all referenced favicons and preview images exist in OutputDir/static/.
// It checks local asset source directories on disk first (e.g. from linkding volume mounts),
// then downloads any remaining missing assets from the remote URLs (with auth token).
func SyncAssets(ctx context.Context, bookmarks []linkding.Bookmark, opts SyncOptions) error {
	staticDir := filepath.Join(opts.OutputDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return fmt.Errorf("creating static directory: %w", err)
	}

	// 1. Gather all unique asset URLs and filenames needed
	type assetTask struct {
		remoteURL  string
		fileName   string
		shardedRel string // e.g. "static/assets/a9/a9bee...png"
	}
	needed := make(map[string]assetTask)
	for _, b := range bookmarks {
		if b.FaviconURL != "" {
			fn := ExtractAssetFilename(b.FaviconURL)
			if fn != "" {
				needed[fn] = assetTask{
					remoteURL:  b.FaviconURL,
					fileName:   fn,
					shardedRel: ShardedAssetPath(b.FaviconURL),
				}
			}
		}
		if b.PreviewImageURL != "" {
			fn := ExtractAssetFilename(b.PreviewImageURL)
			if fn != "" {
				needed[fn] = assetTask{
					remoteURL:  b.PreviewImageURL,
					fileName:   fn,
					shardedRel: ShardedAssetPath(b.PreviewImageURL),
				}
			}
		}
	}

	if len(needed) == 0 {
		return nil
	}

	// 2. Identify which assets are missing from OutputDir/static
	var missing []assetTask
	for _, task := range needed {
		destPath := filepath.Join(opts.OutputDir, task.shardedRel)
		if stat, err := os.Stat(destPath); err == nil && stat.Size() > 0 {
			continue // already present in sharded output directory
		}
		missing = append(missing, task)
	}

	if len(missing) == 0 {
		log.Printf("All %d referenced bookmark assets already cached in %s.", len(needed), staticDir)
		return nil
	}

	log.Printf("Syncing %d missing assets into %s (out of %d total)...", len(missing), staticDir, len(needed))

	// 3. Try copying from local AssetDirs on host if configured
	var stillMissing []assetTask
	copiedFromDisk := 0
	for _, task := range missing {
		copied := false
		for _, srcDir := range opts.AssetDirs {
			if srcDir == "" {
				continue
			}
			candidate := filepath.Join(srcDir, task.fileName)
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() && stat.Size() > 0 {
				destPath := filepath.Join(opts.OutputDir, task.shardedRel)
				_ = os.MkdirAll(filepath.Dir(destPath), 0755)
				if err := copyFile(candidate, destPath); err == nil {
					copiedFromDisk++
					break
				}
			}
		}
		if !copied {
			stillMissing = append(stillMissing, task)
		}
	}

	if copiedFromDisk > 0 {
		log.Printf("Copied %d assets from local disk volumes.", copiedFromDisk)
	}

	if len(stillMissing) == 0 {
		return nil
	}

	// 4. Download remaining assets concurrently over HTTP
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	if concurrency > len(stillMissing) {
		concurrency = len(stillMissing)
	}

	log.Printf("Downloading %d assets from remote Linkding server (using %d workers)...", len(stillMissing), concurrency)

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	ch := make(chan assetTask, len(stillMissing))
	for _, task := range stillMissing {
		ch <- task
	}
	close(ch)

	var wg sync.WaitGroup
	var downloaded uint64
	var failed uint64

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range ch {
				select {
				case <-ctx.Done():
					return
				default:
				}

				destPath := filepath.Join(opts.OutputDir, task.shardedRel)
				_ = os.MkdirAll(filepath.Dir(destPath), 0755)
				if err := downloadAsset(ctx, client, task.remoteURL, destPath, opts.ApiToken); err != nil {
					atomic.AddUint64(&failed, 1)
				} else {
					atomic.AddUint64(&downloaded, 1)
				}
			}
		}()
	}

	wg.Wait()
	log.Printf("Asset sync complete: %d downloaded, %d copied locally, %d failed/unavailable.",
		downloaded, copiedFromDisk, failed)

	return nil
}

func extractFilename(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	return filepath.Base(u.Path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func downloadAsset(ctx context.Context, client *http.Client, rawURL, destPath, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(destPath), "asset-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return err
	}
	if err := tempFile.Chmod(0644); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, destPath)
}
