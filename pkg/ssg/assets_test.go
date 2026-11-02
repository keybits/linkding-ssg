package ssg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lmorchard/linkding-ssg/pkg/linkding"
)

func TestSyncAssets_LocalAndRemote(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "dist")
	srcVolume := filepath.Join(tempDir, "volume")
	_ = os.MkdirAll(srcVolume, 0755)

	// Local file in volume
	localFile := filepath.Join(srcVolume, "test_local.png")
	_ = os.WriteFile(localFile, []byte("local-png-data"), 0644)

	// Remote server for remote file
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/static/test_remote.png" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("remote-png-data"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	bookmarks := []linkding.Bookmark{
		{
			ID:              1,
			FaviconURL:      "http://example.com/static/test_local.png",
			PreviewImageURL: srv.URL + "/static/test_remote.png",
		},
	}

	err := SyncAssets(context.Background(), bookmarks, SyncOptions{
		OutputDir: outDir,
		AssetDirs: []string{srcVolume},
	})
	if err != nil {
		t.Fatalf("SyncAssets failed: %v", err)
	}

	// Verify test_local.png was copied
	destLocal := filepath.Join(outDir, "static", "test_local.png")
	dataLocal, err := os.ReadFile(destLocal)
	if err != nil || string(dataLocal) != "local-png-data" {
		t.Errorf("expected local file copied, got %s, err: %v", string(dataLocal), err)
	}

	// Verify test_remote.png was downloaded
	destRemote := filepath.Join(outDir, "static", "test_remote.png")
	dataRemote, err := os.ReadFile(destRemote)
	if err != nil || string(dataRemote) != "remote-png-data" {
		t.Errorf("expected remote file downloaded, got %s, err: %v", string(dataRemote), err)
	}
}
