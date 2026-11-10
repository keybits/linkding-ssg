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

func TestClient_FetchAllBookmarks_SharedOnly(t *testing.T) {
	allMockBookmarks := []Bookmark{
		{ID: 4, URL: "https://example.com/4", Shared: true},
		{ID: 3, URL: "https://example.com/3", Shared: false},
		{ID: 2, URL: "https://example.com/2", Shared: true},
		{ID: 1, URL: "https://example.com/1", Shared: false},
	}

	for _, tc := range []struct {
		name          string
		serverFilters bool
	}{
		{name: "server honors shared param", serverFilters: true},
		{name: "server ignores shared param", serverFilters: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotSharedParam string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotSharedParam = r.URL.Query().Get("shared")
				results := allMockBookmarks
				if tc.serverFilters && gotSharedParam == "yes" {
					results = nil
					for _, b := range allMockBookmarks {
						if b.Shared {
							results = append(results, b)
						}
					}
				}
				json.NewEncoder(w).Encode(BookmarkListResponse{
					Count:   len(results),
					Results: results,
				})
			}))
			defer server.Close()

			client, err := NewClient(ClientOptions{
				BaseURL:    server.URL,
				Token:      "secret",
				SharedOnly: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			results, err := client.FetchAllBookmarks(context.Background(), 100, nil)
			if err != nil {
				t.Fatalf("FetchAllBookmarks failed: %v", err)
			}

			if gotSharedParam != "yes" {
				t.Errorf("expected shared=yes query param, got %q", gotSharedParam)
			}
			if len(results) != 2 {
				t.Fatalf("expected 2 shared bookmarks, got %d", len(results))
			}
			for _, b := range results {
				if !b.Shared {
					t.Errorf("got unshared bookmark %d", b.ID)
				}
			}
		})
	}
}

func TestClient_FetchBookmarksIncremental(t *testing.T) {
	// Mock server with 100 items (IDs 100 down to 1)
	totalBookmarks := 100
	allMockBookmarks := make([]Bookmark, totalBookmarks)
	now := time.Now().UTC()
	for i := range totalBookmarks {
		allMockBookmarks[i] = Bookmark{
			ID:          int64(totalBookmarks - i),
			URL:         "https://example.com/" + strconv.Itoa(totalBookmarks-i),
			Title:       "Example " + strconv.Itoa(totalBookmarks-i),
			DateCreated: now.Add(-time.Duration(i) * time.Hour),
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 20
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		end := offset + limit
		if end > totalBookmarks {
			end = totalBookmarks
		}

		var nextURL *string
		if end < totalBookmarks {
			next := "http://" + r.Host + "/api/bookmarks/?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(end)
			nextURL = &next
		}

		json.NewEncoder(w).Encode(BookmarkListResponse{
			Count:   totalBookmarks,
			Next:    nextURL,
			Results: allMockBookmarks[offset:end],
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		BaseURL: server.URL,
		Token:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Stop when we see ID <= 90 (we should only get 10 items: 100 down to 91)
	results, err := client.FetchBookmarksIncremental(context.Background(), 20, func(b Bookmark) bool {
		return b.ID <= 90
	}, nil)

	if err != nil {
		t.Fatalf("FetchBookmarksIncremental failed: %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 new items, got %d", len(results))
	}
	if results[0].ID != 100 {
		t.Errorf("expected first item to be ID 100, got %d", results[0].ID)
	}
	if results[9].ID != 91 {
		t.Errorf("expected 10th item to be ID 91, got %d", results[9].ID)
	}
}
