package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Serve starts an HTTP static file server for siteDir at addr (e.g. ":8080").
func Serve(siteDir string, addr string) error {
	absDir, err := filepath.Abs(siteDir)
	if err != nil {
		return fmt.Errorf("resolving absolute directory: %w", err)
	}

	if _, err := os.Stat(absDir); err != nil {
		return fmt.Errorf("site directory does not exist: %w", err)
	}

	fileServer := http.FileServer(http.Dir(absDir))

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable caching for local development preview
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fileServer.ServeHTTP(w, r)
	}))

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		displayURL := formatDisplayURL(addr)
		log.Printf("Serving %s at %s (press Ctrl+C to stop)", absDir, displayURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-stop
	log.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func formatDisplayURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return fmt.Sprintf("http://localhost%s", addr)
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		port := strings.TrimPrefix(addr, "0.0.0.0:")
		return fmt.Sprintf("http://localhost:%s (bound to 0.0.0.0:%s)", port, port)
	}
	return fmt.Sprintf("http://%s", addr)
}
