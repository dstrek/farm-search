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
	source := flag.String("source", "farmproperty", "Source to scrape: farmproperty, farmbuy, rea, or all")
	geocode := flag.Bool("geocode", false, "Enable geocoding for properties without coordinates")
	useBrowser := flag.Bool("browser", false, "Use headless browser (only needed for REA)")
	headless := flag.Bool("headless", true, "Run browser in headless mode (set false to see browser)")
	cookieFile := flag.String("cookies", "", "Path to JSON file with cookies for REA (export from browser)")
	userDataDir := flag.String("profile", "", "Path to Chrome user data directory (use existing browser session)")
	scrapingBeeKey := flag.String("scrapingbee", "", "ScrapingBee API key for bypassing bot protection (REA)")
	flag.Parse()

	// Also check environment variable for ScrapingBee key
	if *scrapingBeeKey == "" {
		*scrapingBeeKey = os.Getenv("SCRAPINGBEE_API_KEY")
	}

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
	config.SkipGeocode = !*geocode
	config.UseBrowser = *useBrowser
	config.Headless = *headless
	config.CookieFile = *cookieFile
	config.ScrapingBeeKey = *scrapingBeeKey

	// Create scraper
	s := scraper.New(database, config)

	// Load cookies if specified
	if *cookieFile != "" {
		if !*useBrowser {
			log.Println("Warning: -cookies flag requires -browser flag, enabling browser mode")
			config.UseBrowser = true
			s = scraper.New(database, config)
		}
		if err := s.LoadCookies(*cookieFile); err != nil {
			log.Fatalf("Failed to load cookies: %v", err)
		}
	}

	// Set user data directory if specified (for using existing Chrome profile)
	if *userDataDir != "" {
		if !*useBrowser {
			log.Println("Warning: -profile flag requires -browser flag, enabling browser mode")
			config.UseBrowser = true
			s = scraper.New(database, config)
		}
		if err := s.SetUserDataDir(*userDataDir); err != nil {
			log.Fatalf("Failed to set user data directory: %v", err)
		}
	}

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
