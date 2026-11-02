package linkding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestClient_FetchAllBookmarks(t *testing.T) {
	// Create mock server with 250 dummy bookmarks across 3 pages (page size 100)
	totalBookmarks := 250
	allMockBookmarks := make([]Bookmark, totalBookmarks)
	now := time.Now().UTC()
	for i := range totalBookmarks {
		allMockBookmarks[i] = Bookmark{
			ID:          int64(i + 1),
			URL:         "https://example.com/" + strconv.Itoa(i+1),
			Title:       "Example " + strconv.Itoa(i+1),
			Description: "Description for bookmark " + strconv.Itoa(i+1),
			TagNames:    []string{"test", "mock"},
			DateCreated: now.Add(-time.Duration(i) * time.Hour),
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token secret123" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/bookmarks/" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")
		limit, _ := strconv.Atoi(limitStr)
		if limit <= 0 {
			limit = 100
		}
		offset, _ := strconv.Atoi(offsetStr)

		if offset >= totalBookmarks {
			json.NewEncoder(w).Encode(BookmarkListResponse{
				Count:   totalBookmarks,
				Results: []Bookmark{},
			})
			return
		}

		end := offset + limit
		if end > totalBookmarks {
			end = totalBookmarks
		}

		pageItems := allMockBookmarks[offset:end]
		var nextURL *string
		if end < totalBookmarks {
			next := "http://" + r.Host + "/api/bookmarks/?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(end)
			nextURL = &next
		}

		json.NewEncoder(w).Encode(BookmarkListResponse{
			Count:   totalBookmarks,
			Next:    nextURL,
			Results: pageItems,
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Token:   "secret123",
	})
	if err != nil {
		t.Fatalf("unexpected NewClient err: %v", err)
	}

	var progressReports int
	results, err := client.FetchAllBookmarks(context.Background(), 100, func(fetched, total int) {
		progressReports++
		if total != totalBookmarks {
			t.Errorf("expected total %d, got %d", totalBookmarks, total)
		}
	})

	if err != nil {
		t.Fatalf("FetchAllBookmarks failed: %v", err)
	}

	if len(results) != totalBookmarks {
		t.Fatalf("expected %d bookmarks, got %d", totalBookmarks, len(results))
	}

	if progressReports != 3 {
		t.Errorf("expected 3 progress reports, got %d", progressReports)
	}
}
