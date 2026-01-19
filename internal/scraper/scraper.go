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
	MaxPages      int
	DelayBetween  time.Duration
	Workers       int
	PropertyTypes []string
	Regions       []string
	UseBrowser    bool   // Use headless browser to bypass bot protection
	Headless      bool   // Run browser in headless mode (no visible window)
	Source        string // Which source to scrape: "rea", "farmproperty", or "all"
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
		UseBrowser: false,          // Default to HTTP (FarmProperty doesn't need browser)
		Headless:   true,           // Run headless by default
		Source:     "farmproperty", // Default to FarmProperty (no bot protection)
	}
}

// Scraper orchestrates property scraping from multiple sources
type Scraper struct {
	db           *db.DB
	config       Config
	rea          *REAScraper
	browser      *BrowserScraper
	farmProperty *FarmPropertyScraper
	geo          *Geocoder
}

// New creates a new Scraper instance
func New(database *db.DB, config Config) *Scraper {
	s := &Scraper{
		db:           database,
		config:       config,
		rea:          NewREAScraper(),
		farmProperty: NewFarmPropertyScraper(),
		geo:          NewGeocoder(),
	}

	if config.UseBrowser {
		s.browser = NewBrowserScraper(config.Headless)
	}

	return s
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

	// Scrape REA if selected
	if s.config.Source == "rea" || s.config.Source == "all" {
		for _, propType := range s.config.PropertyTypes {
			for _, region := range s.config.Regions {
				log.Printf("Scraping REA %s properties in %s...", propType, region)

				var listings []models.Property
				var err error

				// Use browser or HTTP scraper based on config
				if s.browser != nil {
					listings, err = s.browser.ScrapeListings(ctx, region, propType, s.config.MaxPages)
				} else {
					listings, err = s.rea.ScrapeListings(ctx, region, propType, s.config.MaxPages)
				}

				if err != nil {
					log.Printf("Error scraping REA %s/%s: %v", region, propType, err)
					continue
				}

				mu.Lock()
				allListings = append(allListings, listings...)
				mu.Unlock()

				log.Printf("Found %d listings from REA for %s/%s", len(listings), region, propType)

				// Respect rate limits
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(s.config.DelayBetween):
				}
			}
		}
	}

	_ = startTime // Used later

	log.Printf("Total listings found: %d", len(allListings))

	// Geocode listings that don't have coordinates
	geocoded := 0
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

	// Save to database
	saved, err := s.saveListings(allListings)
	if err != nil {
		return fmt.Errorf("failed to save listings: %w", err)
	}

	duration := time.Since(startTime)
	log.Printf("Scraping complete: %d saved in %s", saved, duration)

	return nil
}

func (s *Scraper) saveListings(listings []models.Property) (int, error) {
	saved := 0

	for _, listing := range listings {
		err := s.db.UpsertProperty(&listing)
		if err != nil {
			log.Printf("Failed to save listing %s: %v", listing.ExternalID, err)
			continue
		}
		saved++
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
