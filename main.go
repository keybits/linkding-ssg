package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"github.com/lmorchard/linkding-ssg/pkg/config"
	"github.com/lmorchard/linkding-ssg/pkg/linkding"
	"github.com/lmorchard/linkding-ssg/pkg/pagefind"
	"github.com/lmorchard/linkding-ssg/pkg/server"
	"github.com/lmorchard/linkding-ssg/pkg/ssg"
	"github.com/lmorchard/linkding-ssg/pkg/storage"
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
	case "index":
		err = runIndex(args)
	case "serve":
		err = runServe(args)
	case "sync":
		err = runSync(args)
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
  index    Index the generated static site using Pagefind
  serve    Start a local static HTTP preview server
  sync     Run full pipeline (backup -> build -> index)

Global Configuration:
  Environment variables supported:
    LINKDING_URL    Base URL of your Linkding instance (e.g. https://bookmarks.example.com)
    LINKDING_TOKEN  Linkding REST API token
`)
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	apiURL := fs.String("url", getEnvOrDefault("LINKDING_URL", ""), "Linkding base URL (or LINKDING_URL env)")
	token := fs.String("token", getEnvOrDefault("LINKDING_TOKEN", ""), "Linkding API token (or LINKDING_TOKEN env)")
	output := fs.String("output", "data/bookmarks.json", "Path to snapshot JSON output file")
	pageSize := fs.Int("page-size", 100, "Number of bookmarks to fetch per API request")
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
	log.Println("Starting bookmark export...")
	bookmarks, err := client.FetchAllBookmarks(ctx, *pageSize, func(fetched, total int) {
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

	log.Printf("Successfully fetched %d bookmarks. Writing atomic snapshot to %s...", len(bookmarks), *output)
	if err := storage.SaveSnapshot(*output, *apiURL, bookmarks); err != nil {
		return fmt.Errorf("saving snapshot: %w", err)
	}

	log.Printf("Backup complete: %s (%d bookmarks)", *output, len(bookmarks))
	return nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	input := fs.String("input", "data/bookmarks.json", "Path to snapshot JSON file")
	outputDir := fs.String("output-dir", "dist", "Directory to write static HTML site")
	baseURL := fs.String("base-url", "", "Base URL path prefix (e.g. /bookmarks/)")
	recentLimit := fs.Int("recent-limit", 50, "Number of bookmarks to show on home page")
	templatesDir := fs.String("templates-dir", "", "Optional custom directory for overriding HTML templates")
	runIndexer := fs.Bool("index", false, "Run Pagefind indexer automatically after build")
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
		TemplatesDir: *templatesDir,
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
		if err := pagefind.IndexSite(context.Background(), *outputDir); err != nil {
			return fmt.Errorf("indexing site: %w", err)
		}
		log.Println("Search indexing complete!")
	}

	return nil
}

func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	siteDir := fs.String("site-dir", "dist", "Directory of static site to index")
	fs.Parse(args)

	log.Printf("Indexing static site in %s using Pagefind...", *siteDir)
	if err := pagefind.IndexSite(context.Background(), *siteDir); err != nil {
		return err
	}
	log.Println("Search indexing complete!")
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	siteDir := fs.String("site-dir", "dist", "Directory to serve")
	addr := fs.String("addr", ":8080", "Address and port to bind (e.g. :8080 or 127.0.0.1:8080)")
	fs.Parse(args)

	return server.Serve(*siteDir, *addr)
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	apiURL := fs.String("url", getEnvOrDefault("LINKDING_URL", ""), "Linkding base URL (or LINKDING_URL env)")
	token := fs.String("token", getEnvOrDefault("LINKDING_TOKEN", ""), "Linkding API token (or LINKDING_TOKEN env)")
	dataSnapshot := fs.String("snapshot", "data/bookmarks.json", "Path to snapshot JSON file")
	outputDir := fs.String("output-dir", "dist", "Directory to write static HTML site")
	baseURL := fs.String("base-url", "", "Base URL path prefix")
	skipIndex := fs.Bool("skip-index", false, "Skip Pagefind search indexing step")
	fs.Parse(args)

	if err := runBackup([]string{"--url", *apiURL, "--token", *token, "--output", *dataSnapshot}); err != nil {
		return err
	}

	buildArgs := []string{"--input", *dataSnapshot, "--output-dir", *outputDir, "--base-url", *baseURL}
	if !*skipIndex {
		buildArgs = append(buildArgs, "--index")
	}

	return runBuild(buildArgs)
}
