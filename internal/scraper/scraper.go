package scraper

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"farm-search/internal/db"
	"farm-search/internal/models"
)

// Config holds scraper configuration
type Config struct {
	MaxPages       int
	DelayBetween   time.Duration
	Workers        int
	PropertyTypes  []string
	Regions        []string
	UseBrowser     bool   // Use headless browser to bypass bot protection
	Headless       bool   // Run browser in headless mode (no visible window)
	Source         string // Which source to scrape: "rea", "farmproperty", "farmbuy", "domain", "domain-web", or "all"
	SkipGeocode    bool   // Skip geocoding for properties without coordinates
	CookieFile     string // Path to JSON file containing cookies for REA authentication
	UserDataDir    string // Path to Chrome user data directory for persistent sessions
	ScrapingBeeKey string // ScrapingBee API key for bypassing bot protection (used for REA)
	DomainAPIKey   string // Domain.com.au API key for their official API
	DomainWebURL   string // Custom URL for domain-web scraper (overrides default)
	FullRefresh    bool   // Continue scraping all pages even if properties already exist
}

// DefaultConfig returns default scraper settings
func DefaultConfig() Config {
	return Config{
		MaxPages:     10,
		DelayBetween: 2 * time.Second,
		Workers:      3,
		PropertyTypes: []string{
			"rural",
			"farm",
			"acreage-semi-rural",
		},
		Regions: []string{
			"nsw",
		},
		UseBrowser:  false,          // Default to HTTP (FarmProperty doesn't need browser)
		Headless:    true,           // Run headless by default
		Source:      "farmproperty", // Default to FarmProperty (no bot protection)
		SkipGeocode: true,           // Skip geocoding by default (run separately)
	}
}

// Scraper orchestrates property scraping from multiple sources
type Scraper struct {
	db           *db.DB
	config       Config
	rea          *REAScraper
	browser      *BrowserScraper
	farmProperty *FarmPropertyScraper
	farmBuy      *FarmBuyScraper
	domain       *DomainScraper
	domainWeb    *DomainWebScraper
	geo          *Geocoder
}

// New creates a new Scraper instance
func New(database *db.DB, config Config) *Scraper {
	s := &Scraper{
		db:           database,
		config:       config,
		farmProperty: NewFarmPropertyScraper(),
		farmBuy:      NewFarmBuyScraper(),
		domainWeb:    NewDomainWebScraper(),
		geo:          NewGeocoder(),
	}

	// Use ScrapingBee for REA if API key is provided
	if config.ScrapingBeeKey != "" {
		s.rea = NewREAScraperWithScrapingBee(config.ScrapingBeeKey)
		log.Println("REA scraper configured to use ScrapingBee")
	} else {
		s.rea = NewREAScraper()
	}

	// Initialize Domain API scraper if API key is provided
	if config.DomainAPIKey != "" {
		s.domain = NewDomainScraper(config.DomainAPIKey)
		log.Println("Domain API scraper configured with API key")
	}

	if config.UseBrowser {
		s.browser = NewBrowserScraper(config.Headless)
	}

	return s
}

// LoadCookies loads cookies from a file for the browser scraper
func (s *Scraper) LoadCookies(filepath string) error {
	if s.browser == nil {
		return fmt.Errorf("browser scraper not initialized (use -browser flag)")
	}
	return s.browser.LoadCookiesFromFile(filepath)
}

// SetUserDataDir sets the Chrome user data directory for persistent sessions
func (s *Scraper) SetUserDataDir(dir string) error {
	if s.browser == nil {
		return fmt.Errorf("browser scraper not initialized (use -browser flag)")
	}
	s.browser.SetUserDataDir(dir)
	return nil
}

// Run executes the scraping process
func (s *Scraper) Run(ctx context.Context) error {
	log.Println("Starting scraper...")
	startTime := time.Now()

	// Start browser if using browser mode for REA
	if s.config.UseBrowser && s.browser != nil && (s.config.Source == "rea" || s.config.Source == "all") {
		if err := s.browser.Start(); err != nil {
			return fmt.Errorf("failed to start browser: %w", err)
		}
		defer s.browser.Stop()
		log.Println("Browser started in headless mode")
	}

	var allListings []models.Property
	var mu sync.Mutex

	// Scrape FarmProperty if selected
	if s.config.Source == "farmproperty" || s.config.Source == "all" {
		for _, region := range s.config.Regions {
			log.Printf("Scraping FarmProperty for %s...", region)

			listings, err := s.farmProperty.ScrapeListings(ctx, region, s.config.MaxPages)
			if err != nil {
				log.Printf("Error scraping FarmProperty %s: %v", region, err)
				continue
			}

			mu.Lock()
			allListings = append(allListings, listings...)
			mu.Unlock()

			log.Printf("Found %d listings from FarmProperty for %s", len(listings), region)

			// Respect rate limits
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.config.DelayBetween):
			}
		}
	}

	// Scrape FarmBuy if selected
	if s.config.Source == "farmbuy" || s.config.Source == "all" {
		for _, region := range s.config.Regions {
			log.Printf("Scraping FarmBuy for %s...", region)

			listings, err := s.farmBuy.ScrapeListings(ctx, region, s.config.MaxPages)
			if err != nil {
				log.Printf("Error scraping FarmBuy %s: %v", region, err)
				continue
			}

			mu.Lock()
			allListings = append(allListings, listings...)
			mu.Unlock()

			log.Printf("Found %d listings from FarmBuy for %s", len(listings), region)

			// Respect rate limits
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.config.DelayBetween):
			}
		}
	}

	// Scrape REA if selected
	if s.config.Source == "rea" || s.config.Source == "all" {
		// Log which method we're using
		if s.config.ScrapingBeeKey != "" {
			log.Println("Using ScrapingBee for REA scraping")
		} else if s.browser != nil {
			log.Println("Using browser for REA scraping (may be blocked by Kasada)")
		} else {
			log.Println("Using direct HTTP for REA scraping (will likely be blocked)")
		}

		// Create exists checker to stop pagination when we hit already-scraped properties
		// Pass nil if full refresh is enabled to scrape all pages
		var existsChecker ExistsChecker
		if !s.config.FullRefresh {
			existsChecker = func(externalIDs []string) (map[string]bool, error) {
				return s.db.PropertiesExist(externalIDs, "rea")
			}
		}

		// REA uses a single combined URL for all rural property types
		// so we only need to iterate over regions, not property types
		for _, region := range s.config.Regions {
			log.Printf("Scraping REA rural properties in %s...", region)

			var listings []models.Property
			var err error

			// Priority: ScrapingBee > Browser > Direct HTTP
			// Note: REA scraper already uses ScrapingBee if configured
			if s.browser != nil && s.config.ScrapingBeeKey == "" {
				listings, err = s.browser.ScrapeListings(ctx, region, "rural", s.config.MaxPages)
			} else {
				listings, err = s.rea.ScrapeListingsWithExistsCheck(ctx, region, "rural", s.config.MaxPages, existsChecker)
			}

			if err != nil {
				log.Printf("Error scraping REA %s: %v", region, err)
				continue
			}

			mu.Lock()
			allListings = append(allListings, listings...)
			mu.Unlock()

			log.Printf("Found %d listings from REA for %s", len(listings), region)

			// Respect rate limits (longer delay for ScrapingBee to conserve credits)
			delay := s.config.DelayBetween
			if s.config.ScrapingBeeKey != "" {
				delay = 3 * time.Second // Minimum 3s delay for ScrapingBee
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	// Scrape Domain if selected and API key is configured
	if (s.config.Source == "domain" || s.config.Source == "all") && s.domain != nil {
		// Create exists checker to stop pagination when we hit already-scraped properties
		// Pass nil if full refresh is enabled to scrape all pages
		var existsChecker ExistsChecker
		if !s.config.FullRefresh {
			existsChecker = func(externalIDs []string) (map[string]bool, error) {
				return s.db.PropertiesExist(externalIDs, "domain")
			}
		}

		for _, region := range s.config.Regions {
			log.Printf("Fetching Domain API listings for %s...", region)

			listings, err := s.domain.ScrapeListingsWithExistsCheck(ctx, region, s.config.MaxPages, existsChecker)
			if err != nil {
				log.Printf("Error fetching Domain %s: %v", region, err)
				continue
			}

			mu.Lock()
			allListings = append(allListings, listings...)
			mu.Unlock()

			log.Printf("Found %d listings from Domain for %s", len(listings), region)

			// Respect rate limits
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.config.DelayBetween):
			}
		}
	} else if s.config.Source == "domain" && s.domain == nil {
		log.Println("Warning: Domain source selected but no API key provided (use -domain-api-key flag)")
	}

	// Scrape Domain via web scraping if selected
	if s.config.Source == "domain-web" || s.config.Source == "all" {
		log.Println("Scraping Domain (web)...")

		// Create exists checker to stop pagination when we hit already-scraped properties
		// Pass nil if full refresh is enabled to scrape all pages
		var existsChecker ExistsChecker
		if !s.config.FullRefresh {
			existsChecker = func(externalIDs []string) (map[string]bool, error) {
				return s.db.PropertiesExist(externalIDs, "domain-web")
			}
		} else {
			log.Println("Full refresh enabled - will scrape all pages")
		}

		// Use custom URL if provided, otherwise use default
		config := DefaultDomainWebConfig()
		if s.config.DomainWebURL != "" {
			config.StartURL = s.config.DomainWebURL
		}

		listings, err := s.domainWeb.ScrapeListingsWithExistsCheck(ctx, s.config.MaxPages, config, existsChecker)
		if err != nil {
			log.Printf("Error scraping Domain (web): %v", err)
		} else {
			mu.Lock()
			allListings = append(allListings, listings...)
			mu.Unlock()

			log.Printf("Found %d listings from Domain (web)", len(listings))
		}
	}

	_ = startTime // Used later

	log.Printf("Total listings found: %d", len(allListings))

	// Geocode listings that don't have coordinates (unless skipped)
	geocoded := 0
	if !s.config.SkipGeocode {
		for i := range allListings {
			if !allListings[i].Latitude.Valid || !allListings[i].Longitude.Valid {
				addr := formatAddress(&allListings[i])
				if addr != "" {
					lat, lng, err := s.geo.Geocode(ctx, addr)
					if err != nil {
						log.Printf("Geocoding failed for %s: %v", addr, err)
					} else {
						allListings[i].Latitude.Float64 = lat
						allListings[i].Latitude.Valid = true
						allListings[i].Longitude.Float64 = lng
						allListings[i].Longitude.Valid = true
						geocoded++
					}

					// Rate limit geocoding
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(1 * time.Second):
					}
				}
			}
		}
		log.Printf("Geocoded %d listings", geocoded)
	} else {
		log.Printf("Skipped geocoding (run with -geocode to enable)")
	}

	// Save to database
	saved, err := s.saveListings(allListings)
	if err != nil {
		return fmt.Errorf("failed to save listings: %w", err)
	}

	// Find and link duplicate properties (same property on multiple sites)
	if err := s.db.FindDuplicateProperties(); err != nil {
		log.Printf("Warning: failed to find duplicate properties: %v", err)
	}

	duration := time.Since(startTime)
	log.Printf("Scraping complete: %d saved in %s", saved, duration)

	return nil
}

func (s *Scraper) saveListings(listings []models.Property) (int, error) {
	saved := 0
	skipped := 0

	for _, listing := range listings {
		// Skip properties without coordinates - they can't be shown on the map
		if !listing.Latitude.Valid || !listing.Longitude.Valid {
			skipped++
			continue
		}

		err := s.db.UpsertProperty(&listing)
		if err != nil {
			log.Printf("Failed to save listing %s: %v", listing.ExternalID, err)
			continue
		}
		saved++
	}

	if skipped > 0 {
		log.Printf("Skipped %d properties without coordinates", skipped)
	}

	return saved, nil
}

func formatAddress(p *models.Property) string {
	addr := ""
	if p.Address.Valid && p.Address.String != "" {
		addr = p.Address.String
	}
	if p.Suburb.Valid && p.Suburb.String != "" {
		if addr != "" {
			addr += ", "
		}
		addr += p.Suburb.String
	}
	if addr != "" {
		addr += ", NSW, Australia"
	}
	return addr
}
