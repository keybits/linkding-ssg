package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/lmorchard/linkding-ssg/pkg/config"
	"github.com/lmorchard/linkding-ssg/pkg/linkding"
	"github.com/lmorchard/linkding-ssg/pkg/pagefind"
	"github.com/lmorchard/linkding-ssg/pkg/server"
	"github.com/lmorchard/linkding-ssg/pkg/ssg"
	"github.com/lmorchard/linkding-ssg/pkg/storage"
	"github.com/lmorchard/linkding-ssg/templates"
)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load optional .env file if present
	if err := config.LoadEnv(".env"); err != nil {
		log.Printf("Warning: failed to read .env file: %v", err)
	}
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "backup":
		err = runBackup(args)
	case "build":
		err = runBuild(args)
	case "eject":
		err = runEject(args)
	case "index":
		err = runIndex(args)
	case "serve":
		err = runServe(args)
	case "sync":
		err = runSync(args)
	case "version", "-v", "--version":
		fmt.Printf("linkding-ssg %s (commit: %s, built: %s)\n", version, commit, date)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func printUsage() {
	fmt.Printf(`linkding-ssg - Backup Linkding bookmarks and generate a stable static HTML site

Commands:
  backup   Fetch bookmarks from Linkding API and save snapshot to JSON
  build    Generate static HTML site from local JSON snapshot
  eject    Export embedded templates and static assets to data/ for customization
  index    Index the generated static site using Pagefind
  serve    Start a local static HTTP preview server
  sync     Run full pipeline (backup -> build -> index)
Environment Variables & .env Configuration:
  LINKDING_URL            Linkding base URL (e.g. https://bookmarks.example.com)
  LINKDING_TOKEN          Linkding REST API token
  LINKDING_PAGE_SIZE      Page size for API fetches (default: 100)
  LINKDING_FULL_BACKUP    Force full backup (true/false)
  LINKDING_SNAPSHOT_PATH  Snapshot JSON file path (default: data/bookmarks.json)
  LINKDING_OUTPUT_DIR     Static site output directory (default: dist)
  LINKDING_BASE_URL       Site base URL prefix (e.g. / or /bookmarks/)
  LINKDING_RECENT_LIMIT   Recent bookmarks count on home page (default: 50)
  LINKDING_RSS_LIMIT      Recent bookmarks count in RSS feeds (default: 15)
  LINKDING_SITE_TITLE     Site title for headers and feeds (default: Bookmarks)
  LINKDING_TEMPLATES_DIR  Custom templates override directory (default: data/templates if present)
  LINKDING_STATIC_DIR     Custom static assets override directory (default: data/static if present)
  LINKDING_CONCURRENCY    Worker concurrency for site generation (default: CPU count)
  LINKDING_PAGEFIND_PATH  Custom path to pagefind binary (default: auto/embedded)
  LINKDING_AUTO_INDEX     Run Pagefind search indexing after build (true/false)
`)
}
func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

func getEnvIntOrDefault(key string, fallback int) int {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvBoolOrDefault(key string, fallback bool) bool {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		lower := strings.ToLower(strings.TrimSpace(val))
		if lower == "1" || lower == "true" || lower == "yes" {
			return true
		}
		if lower == "0" || lower == "false" || lower == "no" {
			return false
		}
	}
	return fallback
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	apiURL := fs.String("url", getEnvOrDefault("LINKDING_URL", ""), "Linkding base URL (or LINKDING_URL env)")
	token := fs.String("token", getEnvOrDefault("LINKDING_TOKEN", ""), "Linkding API token (or LINKDING_TOKEN env)")
	output := fs.String("output", getEnvOrDefault("LINKDING_SNAPSHOT_PATH", "data/bookmarks.json"), "Path to snapshot JSON output file (or LINKDING_SNAPSHOT_PATH env)")
	pageSize := fs.Int("page-size", getEnvIntOrDefault("LINKDING_PAGE_SIZE", 100), "Number of bookmarks to fetch per API request (or LINKDING_PAGE_SIZE env)")
	forceFull := fs.Bool("full", getEnvBoolOrDefault("LINKDING_FULL_BACKUP", false), "Force a full backup instead of incremental (or LINKDING_FULL_BACKUP env)")
	fs.Parse(args)
	if *apiURL == "" || *token == "" {
		return fmt.Errorf("both --url and --token (or LINKDING_URL and LINKDING_TOKEN env vars) are required")
	}

	log.Printf("Connecting to Linkding at %s...", *apiURL)
	client, err := linkding.NewClient(linkding.ClientOptions{
		BaseURL: *apiURL,
		Token:   *token,
	})
	if err != nil {
		return fmt.Errorf("creating API client: %w", err)
	}

	ctx := context.Background()
	var bookmarks []linkding.Bookmark

	// Check for existing snapshot for incremental sync
	var existingBookmarks []linkding.Bookmark
	knownIDs := make(map[int64]struct{})
	if !*forceFull {
		if prevSnapshot, err := storage.LoadSnapshot(*output); err == nil && len(prevSnapshot.Bookmarks) > 0 {
			existingBookmarks = prevSnapshot.Bookmarks
			for _, b := range existingBookmarks {
				knownIDs[b.ID] = struct{}{}
			}
			log.Printf("Found existing snapshot with %d bookmarks. Running incremental backup...", len(existingBookmarks))
		}
	}

	if len(knownIDs) > 0 {
		fetchedNew, err := client.FetchBookmarksIncremental(ctx, *pageSize, func(b linkding.Bookmark) bool {
			_, found := knownIDs[b.ID]
			return found
		}, func(fetched, total int) {
			log.Printf("Fetched %d new/recent bookmarks (total upstream: %d)...", fetched, total)
		})
		if err != nil {
			return fmt.Errorf("fetching incremental bookmarks: %w", err)
		}

		log.Printf("Fetched %d new bookmarks. Merging with %d existing bookmarks...", len(fetchedNew), len(existingBookmarks))
		bookmarks = storage.MergeBookmarks(fetchedNew, existingBookmarks)
	} else {
		if *forceFull {
			log.Println("Starting full backup (--full specified)...")
		} else {
			log.Println("No existing snapshot found. Starting full initial backup...")
		}
		var err error
		bookmarks, err = client.FetchAllBookmarks(ctx, *pageSize, func(fetched, total int) {
			if total > 0 {
				percent := float64(fetched) / float64(total) * 100
				log.Printf("Fetched %d / %d bookmarks (%.1f%%)...", fetched, total, percent)
			} else {
				log.Printf("Fetched %d bookmarks...", fetched)
			}
		})
		if err != nil {
			return fmt.Errorf("fetching bookmarks: %w", err)
		}
	}
	log.Printf("Successfully fetched %d bookmarks. Writing atomic snapshot to %s...", len(bookmarks), *output)
	if err := storage.SaveSnapshot(*output, *apiURL, bookmarks); err != nil {
		return fmt.Errorf("saving snapshot: %w", err)
	}

	log.Printf("Backup complete: %s (%d bookmarks)", *output, len(bookmarks))
	return nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	input := fs.String("input", getEnvOrDefault("LINKDING_SNAPSHOT_PATH", "data/bookmarks.json"), "Path to snapshot JSON file (or LINKDING_SNAPSHOT_PATH env)")
	outputDir := fs.String("output-dir", getEnvOrDefault("LINKDING_OUTPUT_DIR", "dist"), "Directory to write static HTML site (or LINKDING_OUTPUT_DIR env)")
	baseURL := fs.String("base-url", getEnvOrDefault("LINKDING_BASE_URL", ""), "Base URL path prefix (or LINKDING_BASE_URL env)")
	recentLimit := fs.Int("recent-limit", getEnvIntOrDefault("LINKDING_RECENT_LIMIT", 50), "Number of bookmarks to show on home page (or LINKDING_RECENT_LIMIT env)")
	rssLimit := fs.Int("rss-limit", getEnvIntOrDefault("LINKDING_RSS_LIMIT", 15), "Number of recent items to include in RSS feeds (or LINKDING_RSS_LIMIT env)")
	siteTitle := fs.String("site-title", getEnvOrDefault("LINKDING_SITE_TITLE", "Bookmarks"), "Site title for RSS feeds (or LINKDING_SITE_TITLE env)")
	templatesDir := fs.String("templates-dir", getEnvOrDefault("LINKDING_TEMPLATES_DIR", ""), "Optional custom directory for overriding HTML templates (or LINKDING_TEMPLATES_DIR env)")
	staticDir := fs.String("static-dir", getEnvOrDefault("LINKDING_STATIC_DIR", ""), "Optional custom directory for overriding static assets (or LINKDING_STATIC_DIR env)")
	concurrency := fs.Int("concurrency", getEnvIntOrDefault("LINKDING_CONCURRENCY", 0), "Number of parallel workers (default CPU count, or LINKDING_CONCURRENCY env)")
	pagefindPath := fs.String("pagefind-path", getEnvOrDefault("LINKDING_PAGEFIND_PATH", ""), "Path to pagefind executable (or LINKDING_PAGEFIND_PATH env)")
	runIndexer := fs.Bool("index", getEnvBoolOrDefault("LINKDING_AUTO_INDEX", false), "Run Pagefind indexer automatically after build (or LINKDING_AUTO_INDEX env)")
	fs.Parse(args)
	log.Printf("Loading bookmarks from %s...", *input)
	snapshot, err := storage.LoadSnapshot(*input)
	if err != nil {
		return fmt.Errorf("loading snapshot: %w", err)
	}
	log.Printf("Loaded %d bookmarks (exported at %s)", len(snapshot.Bookmarks), snapshot.ExportedAt.Format("2006-01-02 15:04:05"))

	generator, err := ssg.NewGenerator(ssg.GeneratorOptions{
		OutputDir:    *outputDir,
		BaseURL:      *baseURL,
		RecentLimit:  *recentLimit,
		RSSLimit:     *rssLimit,
		SiteTitle:    *siteTitle,
		TemplatesDir: *templatesDir,
		StaticDir:    *staticDir,
		Concurrency:  *concurrency,
	})
	if err != nil {
		return fmt.Errorf("initializing generator: %w", err)
	}

	log.Printf("Generating static site into %s...", *outputDir)
	if err := generator.Generate(snapshot.Bookmarks); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}
	log.Println("Static HTML site generation complete!")

	if *runIndexer {
		log.Println("Running Pagefind indexer...")
		if err := pagefind.IndexSite(context.Background(), pagefind.IndexOptions{
			SiteDir:      *outputDir,
			PagefindPath: *pagefindPath,
		}); err != nil {
			return fmt.Errorf("indexing site: %w", err)
		}
		log.Println("Search indexing complete!")
	}

	return nil
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	siteDir := fs.String("site-dir", getEnvOrDefault("LINKDING_OUTPUT_DIR", "dist"), "Directory of static site to index (or LINKDING_OUTPUT_DIR env)")
	pagefindPath := fs.String("pagefind-path", getEnvOrDefault("LINKDING_PAGEFIND_PATH", ""), "Path to pagefind executable (or LINKDING_PAGEFIND_PATH env)")
	fs.Parse(args)
	log.Printf("Indexing static site in %s using Pagefind...", *siteDir)
	if err := pagefind.IndexSite(context.Background(), pagefind.IndexOptions{
		SiteDir:      *siteDir,
		PagefindPath: *pagefindPath,
	}); err != nil {
		return err
	}
	log.Println("Search indexing complete!")
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	siteDir := fs.String("site-dir", getEnvOrDefault("LINKDING_OUTPUT_DIR", "dist"), "Directory to serve (or LINKDING_OUTPUT_DIR env)")
	addr := fs.String("addr", getEnvOrDefault("LINKDING_SERVE_ADDR", ":8080"), "Address and port to bind (or LINKDING_SERVE_ADDR env)")
	fs.Parse(args)

	return server.Serve(*siteDir, *addr)
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	apiURL := fs.String("url", getEnvOrDefault("LINKDING_URL", ""), "Linkding base URL (or LINKDING_URL env)")
	token := fs.String("token", getEnvOrDefault("LINKDING_TOKEN", ""), "Linkding API token (or LINKDING_TOKEN env)")
	dataSnapshot := fs.String("snapshot", getEnvOrDefault("LINKDING_SNAPSHOT_PATH", "data/bookmarks.json"), "Path to snapshot JSON file (or LINKDING_SNAPSHOT_PATH env)")
	outputDir := fs.String("output-dir", getEnvOrDefault("LINKDING_OUTPUT_DIR", "dist"), "Directory to write static HTML site (or LINKDING_OUTPUT_DIR env)")
	baseURL := fs.String("base-url", getEnvOrDefault("LINKDING_BASE_URL", ""), "Base URL path prefix (or LINKDING_BASE_URL env)")
	forceFull := fs.Bool("full", getEnvBoolOrDefault("LINKDING_FULL_BACKUP", false), "Force a full backup instead of incremental (or LINKDING_FULL_BACKUP env)")
	rssLimit := fs.Int("rss-limit", getEnvIntOrDefault("LINKDING_RSS_LIMIT", 15), "Number of recent items to include in RSS feeds (or LINKDING_RSS_LIMIT env)")
	siteTitle := fs.String("site-title", getEnvOrDefault("LINKDING_SITE_TITLE", "Bookmarks"), "Site title for RSS feeds (or LINKDING_SITE_TITLE env)")
	concurrency := fs.Int("concurrency", getEnvIntOrDefault("LINKDING_CONCURRENCY", 0), "Number of parallel workers (or LINKDING_CONCURRENCY env)")
	pagefindPath := fs.String("pagefind-path", getEnvOrDefault("LINKDING_PAGEFIND_PATH", ""), "Path to pagefind executable (or LINKDING_PAGEFIND_PATH env)")
	skipIndex := fs.Bool("skip-index", false, "Skip Pagefind search indexing step")
	backupArgs := []string{"--url", *apiURL, "--token", *token, "--output", *dataSnapshot}
	if *forceFull {
		backupArgs = append(backupArgs, "--full")
	}
	if err := runBackup(backupArgs); err != nil {
		return err
	}
	buildArgs := []string{
		"--input", *dataSnapshot,
		"--output-dir", *outputDir,
		"--base-url", *baseURL,
		fmt.Sprintf("--rss-limit=%d", *rssLimit),
		fmt.Sprintf("--site-title=%s", *siteTitle),
		fmt.Sprintf("--concurrency=%d", *concurrency),
	}
	if *pagefindPath != "" {
		buildArgs = append(buildArgs, "--pagefind-path", *pagefindPath)
	}
	if !*skipIndex {
		buildArgs = append(buildArgs, "--index")
	}
	return runBuild(buildArgs)
}

func runEject(args []string) error {
	fs := flag.NewFlagSet("eject", flag.ExitOnError)
	targetDir := fs.String("target-dir", "data", "Target directory to export templates/ and static/ into")
	fs.Parse(args)

	log.Printf("Exporting embedded HTML templates and static assets into %s...", *targetDir)
	if err := templates.ExportAssets(*targetDir); err != nil {
		return fmt.Errorf("exporting assets: %w", err)
	}

	log.Printf("Assets successfully ejected!")
	log.Printf("  Templates: %s/templates/", *targetDir)
	log.Printf("  Static:    %s/static/", *targetDir)
	log.Printf("Edit these files to customize your site. linkding-ssg will automatically use them when present.")
	return nil
}
