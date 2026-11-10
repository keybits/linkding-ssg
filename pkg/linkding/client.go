package linkding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Bookmark represents the linkding bookmark model as returned by the REST API.
type Bookmark struct {
	ID                    int64     `json:"id"`
	URL                   string    `json:"url"`
	Title                 string    `json:"title"`
	Description           string    `json:"description"`
	Notes                 string    `json:"notes"`
	WebsiteTitle          string    `json:"website_title,omitempty"`
	WebsiteDescription    string    `json:"website_description,omitempty"`
	WebArchiveSnapshotURL string    `json:"web_archive_snapshot_url,omitempty"`
	FaviconURL            string    `json:"favicon_url,omitempty"`
	PreviewImageURL       string    `json:"preview_image_url,omitempty"`
	IsArchived            bool      `json:"is_archived"`
	Unread                bool      `json:"unread"`
	Shared                bool      `json:"shared"`
	TagNames              []string  `json:"tag_names"`
	DateAdded             time.Time `json:"date_added"`
	DateCreated           time.Time `json:"date_created"`
	DateModified          time.Time `json:"date_modified"`
}

// UnmarshalJSON ensures both date_added and date_created are populated.
func (b *Bookmark) UnmarshalJSON(data []byte) error {
	type Alias Bookmark
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(b),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if b.DateCreated.IsZero() && !b.DateAdded.IsZero() {
		b.DateCreated = b.DateAdded
	} else if b.DateAdded.IsZero() && !b.DateCreated.IsZero() {
		b.DateAdded = b.DateCreated
	}
	return nil
}
// Date returns the best timestamp available (DateCreated, DateAdded, or DateModified).
func (b Bookmark) Date() time.Time {
	if !b.DateCreated.IsZero() {
		return b.DateCreated
	}
	if !b.DateAdded.IsZero() {
		return b.DateAdded
	}
	return b.DateModified
}

func (b Bookmark) DisplayTitle() string {
	if strings.TrimSpace(b.Title) != "" {
		return b.Title
	}
	if strings.TrimSpace(b.WebsiteTitle) != "" {
		return b.WebsiteTitle
	}
	return b.URL
}

// DisplayDescription returns the best description available.
func (b Bookmark) DisplayDescription() string {
	if strings.TrimSpace(b.Description) != "" {
		return b.Description
	}
	return b.WebsiteDescription
}

// BookmarkListResponse is the paginated response envelope from GET /api/bookmarks/.
type BookmarkListResponse struct {
	Count    int        `json:"count"`
	Next     *string    `json:"next"`
	Previous *string    `json:"previous"`
	Results  []Bookmark `json:"results"`
}

// Snapshot represents our local backup envelope preserving all bookmarks and metadata.
type Snapshot struct {
	ExportedAt   time.Time  `json:"exported_at"`
	ServerURL    string     `json:"server_url"`
	TotalCount   int        `json:"total_count"`
	SharedOnly   bool       `json:"shared_only,omitempty"`
	Bookmarks    []Bookmark `json:"bookmarks"`
}

// Client interacts with the linkding REST API.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	sharedOnly bool
}

// ClientOptions configures a linkding API Client.
type ClientOptions struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	// SharedOnly restricts bulk fetches to bookmarks marked as shared.
	SharedOnly bool
}

// NewClient creates a new linkding API Client.
func NewClient(opts ClientOptions) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimRight(opts.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host (e.g. http://localhost:8000)")
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Client{
		baseURL:    parsedURL,
		token:      opts.Token,
		httpClient: httpClient,
		sharedOnly: opts.SharedOnly,
	}, nil
}

// ListOptions specifies parameters for listing bookmarks.
type ListOptions struct {
	Limit  int
	Offset int
	Query  string
	// SharedOnly requests only shared bookmarks (linkding `shared=yes` filter).
	SharedOnly bool
}

// GetBookmarks fetches a single page of bookmarks.
func (c *Client) GetBookmarks(ctx context.Context, opts ListOptions) (*BookmarkListResponse, error) {
	relPath := "/api/bookmarks/"
	reqURL, err := c.baseURL.Parse(relPath)
	if err != nil {
		return nil, fmt.Errorf("parsing relative path: %w", err)
	}

	query := reqURL.Query()
	if opts.Limit > 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	} else {
		query.Set("limit", "100")
	}
	if opts.Offset > 0 {
		query.Set("offset", strconv.Itoa(opts.Offset))
	}
	if opts.Query != "" {
		query.Set("q", opts.Query)
	}
	if opts.SharedOnly {
		query.Set("shared", "yes")
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "linkding-ssg/1.0")

	// Retry on transient 5xx or network errors up to 3 times
	var resp *http.Response
	var reqErr error
	backoffs := []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond, 3000 * time.Millisecond}

	for attempt := 0; attempt <= len(backoffs); attempt++ {
		resp, reqErr = c.httpClient.Do(req)
		if reqErr == nil && resp.StatusCode < 500 {
			break
		}
		if attempt < len(backoffs) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffs[attempt]):
			}
		}
	}

	if reqErr != nil {
		return nil, fmt.Errorf("performing request: %w", reqErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("linkding API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodySnippet)))
	}

	var listResp BookmarkListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decoding JSON response: %w", err)
	}

	return &listResp, nil
}

// ProgressCallback receives the count of bookmarks fetched so far and the total count.
type ProgressCallback func(fetched int, total int)

// StopPredicate returns true if fetching should stop upon seeing a bookmark.
type StopPredicate func(b Bookmark) bool

 // FetchAllBookmarks iterates through all pages and collects all bookmarks.
 func (c *Client) FetchAllBookmarks(ctx context.Context, pageSize int, progress ProgressCallback) ([]Bookmark, error) {
	return c.FetchBookmarksIncremental(ctx, pageSize, nil, progress)
}

// FetchBookmarksIncremental fetches bookmarks until the stop predicate returns true or all bookmarks are read.
func (c *Client) FetchBookmarksIncremental(ctx context.Context, pageSize int, shouldStop StopPredicate, progress ProgressCallback) ([]Bookmark, error) {
	if pageSize <= 0 {
		pageSize = 100
	}

	var all []Bookmark
	offset := 0
	totalCount := -1
	stopped := false

	for {
		resp, err := c.GetBookmarks(ctx, ListOptions{
			Limit:      pageSize,
			Offset:     offset,
			SharedOnly: c.sharedOnly,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching bookmarks at offset %d: %w", offset, err)
		}

		if totalCount == -1 {
			totalCount = resp.Count
			if capNeeded := resp.Count; capNeeded > 0 {
				all = make([]Bookmark, 0, capNeeded)
			}
		}

		for _, b := range resp.Results {
			if shouldStop != nil && shouldStop(b) {
				stopped = true
				break
			}
			// The `shared` query param is undocumented in linkding's API, so also
			// filter locally in case the server ignores it.
			if c.sharedOnly && !b.Shared {
				continue
			}
			all = append(all, b)
		}

		if progress != nil {
			progress(len(all), totalCount)
		}

		if stopped || resp.Next == nil || *resp.Next == "" || len(resp.Results) == 0 || len(all) >= totalCount {
			break
		}

		offset += len(resp.Results)
	}

	return all, nil
}
