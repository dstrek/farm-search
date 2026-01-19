package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"farm-search/internal/db"
	"farm-search/internal/scraper"
)

func main() {
	// Parse command line flags
	dbPath := flag.String("db", "", "Path to SQLite database")
	maxPages := flag.Int("pages", 5, "Maximum pages to scrape per property type")
	workers := flag.Int("workers", 3, "Number of concurrent workers")
	delay := flag.Duration("delay", 2*time.Second, "Delay between requests")
	source := flag.String("source", "farmproperty", "Source to scrape: farmproperty, rea, or all")
	useBrowser := flag.Bool("browser", false, "Use headless browser (only needed for REA)")
	headless := flag.Bool("headless", true, "Run browser in headless mode (set false to see browser)")
	flag.Parse()

	// Determine database path
	if *dbPath == "" {
		// Try relative path from current working directory
		cwd, _ := os.Getwd()
		*dbPath = filepath.Join(cwd, "data", "farm-search.db")
	}

	log.Printf("Using database: %s", *dbPath)

	// Initialize database
	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Configure scraper
	config := scraper.DefaultConfig()
	config.MaxPages = *maxPages
	config.Workers = *workers
	config.DelayBetween = *delay
	config.Source = *source
	config.UseBrowser = *useBrowser
	config.Headless = *headless

	// Create scraper
	s := scraper.New(database, config)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Run the scraper
	log.Println("Starting property scraper...")
	startTime := time.Now()

	if err := s.Run(ctx); err != nil {
		if ctx.Err() == context.Canceled {
			log.Println("Scraper cancelled by user")
		} else {
			log.Fatalf("Scraper failed: %v", err)
		}
	}

	log.Printf("Scraping completed in %s", time.Since(startTime))
}
