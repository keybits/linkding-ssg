package ssg

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"strings"
)

// ShardedAssetPath returns the relative storage and URL path for an asset.
// Rather than dumping 12,000+ files into a single flat static/ directory,
// we shard by a 2-character SHA-256 hash prefix of the filename (similar to Git's loose object store):
// e.g. "a9bee948ef696aee7a7a2c76c520ce45.png" -> "static/assets/a9/a9bee948ef696aee7a7a2c76c520ce45.png"
// e.g. "https_github_com.png" -> "static/assets/e3/https_github_com.png"
func ShardedAssetPath(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	filename := ExtractAssetFilename(rawURL)
	if filename == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(filename))
	prefix := hex.EncodeToString(hash[:1]) // 2 hex chars: 00..ff (256 buckets)

	return filepath.Join("static", "assets", prefix, filename)
}

// ExtractAssetFilename parses the base filename from a raw URL or path.
func ExtractAssetFilename(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	return filepath.Base(u.Path)
}
