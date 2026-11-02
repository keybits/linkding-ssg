package pagefind

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/lmorchard/linkding-ssg/pkg/pagefind/bin"
)

// IndexOptions configures pagefind indexing.
type IndexOptions struct {
	SiteDir      string
	PagefindPath string // optional explicit path to pagefind binary
	CacheDir     string // optional directory to extract embedded binary (default: data/bin)
}

// IndexSite runs pagefind against siteDir.
// It resolves the pagefind executable in the following order:
// 1. Explicit opts.PagefindPath if provided and valid.
// 2. System PATH for an existing `pagefind` executable.
// 3. Embedded standalone pagefind binary (extracted to opts.CacheDir or data/bin/pagefind).
// Zero network downloads, zero node, zero npx.
func IndexSite(ctx context.Context, opts IndexOptions) error {
	binPath, err := resolvePagefindBinary(opts)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, binPath, "--site", opts.SiteDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pagefind execution failed (%s): %w", binPath, err)
	}

	return nil
}

func resolvePagefindBinary(opts IndexOptions) (string, error) {
	// 1. Check explicit path
	if opts.PagefindPath != "" {
		if stat, err := os.Stat(opts.PagefindPath); err == nil && !stat.IsDir() {
			return opts.PagefindPath, nil
		}
		// If not a path, check LookPath for custom binary name
		if p, err := exec.LookPath(opts.PagefindPath); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("specified pagefind path not found or not executable: %s", opts.PagefindPath)
	}

	// 2. Check system PATH
	if p, err := exec.LookPath("pagefind"); err == nil {
		return p, nil
	}

	// 3. Check embedded standalone binary
	embeddedGz := bin.GetEmbeddedPagefindGz()
	if len(embeddedGz) == 0 {
		return "", fmt.Errorf("pagefind not found in PATH, and no embedded binary is available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Determine cache directory
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join("data", "bin")
	}

	binaryName := "pagefind"
	if runtime.GOOS == "windows" {
		binaryName = "pagefind.exe"
	}
	extractedPath := filepath.Join(cacheDir, binaryName)

	// If already extracted, verify and reuse
	if stat, err := os.Stat(extractedPath); err == nil && stat.Size() > 0 {
		return extractedPath, nil
	}

	log.Printf("Extracting standalone Pagefind binary for %s/%s into %s...", runtime.GOOS, runtime.GOARCH, extractedPath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("creating pagefind cache directory %s: %w", cacheDir, err)
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(embeddedGz))
	if err != nil {
		return "", fmt.Errorf("reading embedded pagefind gz: %w", err)
	}
	defer gzReader.Close()

	// Atomic write
	tempFile, err := os.CreateTemp(cacheDir, "pagefind-*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating temp file for pagefind: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, gzReader); err != nil {
		return "", fmt.Errorf("extracting embedded pagefind: %w", err)
	}

	if err := tempFile.Chmod(0755); err != nil {
		return "", fmt.Errorf("chmodding pagefind executable: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp pagefind: %w", err)
	}

	if err := os.Rename(tempPath, extractedPath); err != nil {
		return "", fmt.Errorf("renaming temp pagefind to %s: %w", extractedPath, err)
	}

	return extractedPath, nil
}
