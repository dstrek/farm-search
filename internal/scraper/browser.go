package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"farm-search/internal/models"
)

// BrowserScraper uses headless Chrome to scrape REA
type BrowserScraper struct {
	allocCtx context.Context
	cancel   context.CancelFunc
	headless bool
}

// NewBrowserScraper creates a new browser-based scraper
func NewBrowserScraper(headless bool) *BrowserScraper {
	return &BrowserScraper{
		headless: headless,
	}
}

// Start initializes the browser
func (s *BrowserScraper) Start() error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", s.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		// Anti-detection flags
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins-discovery", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		// Window size to look like real browser
		chromedp.WindowSize(1920, 1080),
		// Real-looking user agent
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"),
	)

	s.allocCtx, s.cancel = chromedp.NewExecAllocator(context.Background(), opts...)
	return nil
}

// Stop closes the browser
func (s *BrowserScraper) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// ScrapeListings scrapes property listings from REA using a real browser
func (s *BrowserScraper) ScrapeListings(ctx context.Context, region, propertyType string, maxPages int) ([]models.Property, error) {
	var allListings []models.Property

	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		log.Printf("Scraping page %d of %s/%s...", page, region, propertyType)

		listings, hasMore, err := s.scrapePage(ctx, region, propertyType, page)
		if err != nil {
			log.Printf("Error scraping page %d: %v", page, err)
			break
		}

		allListings = append(allListings, listings...)
		log.Printf("Found %d listings on page %d (total: %d)", len(listings), page, len(allListings))

		if !hasMore || len(listings) == 0 {
			log.Printf("No more pages available")
			break
		}

		// Rate limiting between pages
		time.Sleep(2 * time.Second)
	}

	return allListings, nil
}

func (s *BrowserScraper) scrapePage(ctx context.Context, region, propertyType string, page int) ([]models.Property, bool, error) {
	// Build the search URL
	searchURL := fmt.Sprintf(
		"https://www.realestate.com.au/buy/property-%s-in-%s/list-%d",
		propertyType, region, page,
	)

	// Create a new browser context for this page
	taskCtx, cancel := chromedp.NewContext(s.allocCtx)
	defer cancel()

	// Set timeout
	taskCtx, cancel = context.WithTimeout(taskCtx, 45*time.Second)
	defer cancel()

	var html string
	var pageURL string

	// First run anti-detection JavaScript
	err := chromedp.Run(taskCtx,
		// Navigate to the page
		chromedp.Navigate(searchURL),
		// Wait for body to be ready
		chromedp.WaitReady("body"),
		// Run anti-detection scripts
		chromedp.Evaluate(`
			// Remove webdriver property
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
			// Fix plugins
			Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
			// Fix languages
			Object.defineProperty(navigator, 'languages', {get: () => ['en-AU', 'en']});
			// Fix permissions
			const originalQuery = window.navigator.permissions.query;
			window.navigator.permissions.query = (parameters) => (
				parameters.name === 'notifications' ?
					Promise.resolve({ state: Notification.permission }) :
					originalQuery(parameters)
			);
		`, nil),
		// Wait longer for content to load (Kasada might need time to verify)
		chromedp.Sleep(8*time.Second),
		// Get the full page HTML
		chromedp.OuterHTML("html", &html),
		chromedp.Location(&pageURL),
	)
	if err != nil {
		return nil, false, fmt.Errorf("navigation failed: %w", err)
	}

	log.Printf("Page loaded, URL: %s, HTML length: %d", pageURL, len(html))

	// Check if we're still on a challenge page
	if strings.Contains(html, "KPSDK") && len(html) < 5000 {
		return nil, false, fmt.Errorf("still blocked by bot protection")
	}

	// Parse the HTML to extract listings
	listings, hasMore := s.parseListingsPage(html, propertyType)

	return listings, hasMore, nil
}

// parseListingsPage extracts property data from the HTML page
func (s *BrowserScraper) parseListingsPage(html, propertyType string) ([]models.Property, bool) {
	var listings []models.Property

	// First try to extract from embedded JSON (ArgonautExchange)
	jsonPattern := regexp.MustCompile(`window\.ArgonautExchange\s*=\s*(\{.+?\});?\s*</script>`)
	matches := jsonPattern.FindStringSubmatch(html)

	if len(matches) >= 2 {
		// Parse the JSON data
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			listings = s.extractListingsFromJSON(data, propertyType)
		}
	}

	// If JSON extraction didn't work, fall back to HTML parsing
	if len(listings) == 0 {
		listings = s.parseListingCards(html, propertyType)
	}

	// Check if there are more pages
	hasMore := strings.Contains(html, `rel="next"`) ||
		strings.Contains(html, "aria-label=\"Go to next page\"") ||
		regexp.MustCompile(`data-testid="[^"]*next[^"]*"`).MatchString(html)

	return listings, hasMore
}

// parseListingCards extracts listings from HTML listing cards
func (s *BrowserScraper) parseListingCards(html, propertyType string) []models.Property {
	var listings []models.Property

	// Find all property links
	linkPattern := regexp.MustCompile(`href="(/property-[^"]+)"`)
	links := linkPattern.FindAllStringSubmatch(html, -1)

	// Extract listing IDs pattern
	listingIDPattern := regexp.MustCompile(`-(\d{6,})$`)

	seenIDs := make(map[string]bool)

	for _, link := range links {
		if len(link) < 2 {
			continue
		}

		path := link[1]

		// Extract listing ID (6+ digits at end of URL)
		idMatches := listingIDPattern.FindStringSubmatch(path)
		if len(idMatches) < 2 {
			continue
		}

		listingID := idMatches[1]
		if seenIDs[listingID] {
			continue
		}
		seenIDs[listingID] = true

		// Parse the URL to extract address info
		listing := s.parseListingURL(path, listingID, propertyType)
		if listing != nil {
			listings = append(listings, *listing)
		}
	}

	return listings
}

// parseListingURL extracts info from a property URL
func (s *BrowserScraper) parseListingURL(path, listingID, propertyType string) *models.Property {
	now := time.Now()
	listing := &models.Property{
		ExternalID:   listingID,
		Source:       "rea",
		URL:          "https://www.realestate.com.au" + path,
		State:        "NSW",
		ScrapedAt:    now,
		UpdatedAt:    now,
		PropertyType: sql.NullString{String: propertyType, Valid: true},
	}

	// Try to extract postcode (4 digits before the ID)
	postcodePattern := regexp.MustCompile(`-(\d{4})-\d+$`)
	if matches := postcodePattern.FindStringSubmatch(path); len(matches) > 1 {
		listing.Postcode = sql.NullString{String: matches[1], Valid: true}
	}

	// Extract suburb (word before -nsw-)
	suburbPattern := regexp.MustCompile(`-([a-z][a-z\+]+)-nsw-\d{4}`)
	if matches := suburbPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		suburb := strings.ReplaceAll(matches[1], "+", " ")
		listing.Suburb = sql.NullString{String: toTitleCase(suburb), Valid: true}
	}

	// Extract street address (between property type and suburb)
	// Pattern: /property-rural-123+example+street-suburb-nsw-2000-12345678
	addressPattern := regexp.MustCompile(`/property-[^/]+-([^/]+)-[a-z]+-nsw-\d{4}-\d+$`)
	if matches := addressPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		addr := strings.ReplaceAll(matches[1], "+", " ")
		addr = strings.ReplaceAll(addr, "-", " ")
		listing.Address = sql.NullString{String: toTitleCase(addr), Valid: true}
	}

	return listing
}

// extractListingsFromJSON extracts listings from the parsed JSON data
func (s *BrowserScraper) extractListingsFromJSON(data map[string]interface{}, propertyType string) []models.Property {
	var listings []models.Property

	// Navigate to find listings in various JSON structures
	// Try: rpiResults.tieredResults[].results[]
	if rpi, ok := data["rpiResults"].(map[string]interface{}); ok {
		if tiered, ok := rpi["tieredResults"].([]interface{}); ok {
			for _, tier := range tiered {
				if tierMap, ok := tier.(map[string]interface{}); ok {
					if results, ok := tierMap["results"].([]interface{}); ok {
						for _, result := range results {
							if listing := s.parseJSONListing(result, propertyType); listing != nil {
								listings = append(listings, *listing)
							}
						}
					}
				}
			}
		}
	}

	return listings
}

func (s *BrowserScraper) parseJSONListing(data interface{}, propertyType string) *models.Property {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	now := time.Now()
	listing := &models.Property{
		Source:       "rea",
		State:        "NSW",
		ScrapedAt:    now,
		UpdatedAt:    now,
		PropertyType: sql.NullString{String: propertyType, Valid: true},
	}

	// Extract ID
	if id, ok := m["id"].(string); ok {
		listing.ExternalID = id
	} else if id, ok := m["listingId"].(string); ok {
		listing.ExternalID = id
	}

	if listing.ExternalID == "" {
		return nil
	}

	// Extract URL
	if prettyUrl, ok := m["prettyUrl"].(string); ok {
		listing.URL = "https://www.realestate.com.au" + prettyUrl
	} else if link, ok := m["_links"].(map[string]interface{}); ok {
		if canonical, ok := link["canonical"].(map[string]interface{}); ok {
			if href, ok := canonical["href"].(string); ok {
				listing.URL = href
			}
		}
	}

	// Extract address
	if address, ok := m["address"].(map[string]interface{}); ok {
		if display, ok := address["display"].(map[string]interface{}); ok {
			if shortAddr, ok := display["shortAddress"].(string); ok {
				listing.Address = sql.NullString{String: shortAddr, Valid: true}
			}
		}
		if suburb, ok := address["suburb"].(string); ok {
			listing.Suburb = sql.NullString{String: suburb, Valid: true}
		}
		if postcode, ok := address["postcode"].(string); ok {
			listing.Postcode = sql.NullString{String: postcode, Valid: true}
		}

		// Extract coordinates
		if location, ok := address["location"].(map[string]interface{}); ok {
			if lat, ok := location["latitude"].(float64); ok {
				listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
			}
			if lng, ok := location["longitude"].(float64); ok {
				listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
			}
		}
	}

	// Extract price
	if price, ok := m["price"].(map[string]interface{}); ok {
		if display, ok := price["display"].(string); ok {
			listing.PriceText = sql.NullString{String: display, Valid: true}
		}
	}

	// Extract features
	if features, ok := m["generalFeatures"].(map[string]interface{}); ok {
		if beds, ok := features["bedrooms"].(map[string]interface{}); ok {
			if val, ok := beds["value"].(float64); ok {
				listing.Bedrooms = sql.NullInt64{Int64: int64(val), Valid: true}
			}
		}
		if baths, ok := features["bathrooms"].(map[string]interface{}); ok {
			if val, ok := baths["value"].(float64); ok {
				listing.Bathrooms = sql.NullInt64{Int64: int64(val), Valid: true}
			}
		}
	}

	// Extract land size
	if propertySizes, ok := m["propertySizes"].(map[string]interface{}); ok {
		if land, ok := propertySizes["land"].(map[string]interface{}); ok {
			if size, ok := land["displayValue"].(string); ok {
				listing.LandSizeSqm = sql.NullFloat64{Float64: parseLandSize(size), Valid: true}
			}
		}
	}

	// Extract images
	if media, ok := m["media"].([]interface{}); ok {
		var images []string
		for _, item := range media {
			if img, ok := item.(map[string]interface{}); ok {
				if imgType, ok := img["type"].(string); ok && imgType == "photo" {
					if url, ok := img["url"].(string); ok {
						images = append(images, url)
					}
				}
			}
		}
		if len(images) > 0 {
			imgJSON, _ := json.Marshal(images)
			listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
		}
	}

	return listing
}

// FetchListingDetails fetches full details for a single listing using browser
func (s *BrowserScraper) FetchListingDetails(ctx context.Context, listingURL string) (*models.Property, error) {
	taskCtx, cancel := chromedp.NewContext(s.allocCtx)
	defer cancel()

	taskCtx, cancel = context.WithTimeout(taskCtx, 30*time.Second)
	defer cancel()

	var html string

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(listingURL),
		chromedp.WaitVisible(`div[class*="property-info"], h1`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load listing page: %w", err)
	}

	return s.parseListingDetails(html, listingURL)
}

func (s *BrowserScraper) parseListingDetails(html, listingURL string) (*models.Property, error) {
	// Extract the listing ID from URL
	idPattern := regexp.MustCompile(`-(\d+)$`)
	matches := idPattern.FindStringSubmatch(listingURL)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract listing ID from URL")
	}

	now := time.Now()
	listing := &models.Property{
		ExternalID: matches[1],
		Source:     "rea",
		URL:        listingURL,
		State:      "NSW",
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Try to find embedded JSON with full listing data
	jsonPattern := regexp.MustCompile(`<script[^>]*type="application/ld\+json"[^>]*>(\{[^<]+\})</script>`)
	jsonMatches := jsonPattern.FindAllStringSubmatch(html, -1)

	for _, match := range jsonMatches {
		if len(match) < 2 {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
			continue
		}

		// Check if this is a RealEstateListing
		if schemaType, ok := data["@type"].(string); ok && strings.Contains(schemaType, "RealEstateListing") {
			if name, ok := data["name"].(string); ok {
				listing.Address = sql.NullString{String: name, Valid: true}
			}
			if desc, ok := data["description"].(string); ok {
				listing.Description = sql.NullString{String: desc, Valid: true}
			}
			if addr, ok := data["address"].(map[string]interface{}); ok {
				if locality, ok := addr["addressLocality"].(string); ok {
					listing.Suburb = sql.NullString{String: locality, Valid: true}
				}
				if postcode, ok := addr["postalCode"].(string); ok {
					listing.Postcode = sql.NullString{String: postcode, Valid: true}
				}
			}
			if geo, ok := data["geo"].(map[string]interface{}); ok {
				if lat, ok := geo["latitude"].(float64); ok {
					listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
				}
				if lng, ok := geo["longitude"].(float64); ok {
					listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
				}
			}
		}
	}

	// Extract price from page
	pricePattern := regexp.MustCompile(`class="[^"]*property-price[^"]*"[^>]*>([^<]+)<`)
	if matches := pricePattern.FindStringSubmatch(html); len(matches) > 1 {
		listing.PriceText = sql.NullString{String: strings.TrimSpace(matches[1]), Valid: true}
	}

	return listing, nil
}

// toTitleCase converts a string to title case
func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// parseLandSizeBrowser converts land size strings to square meters
func parseLandSizeBrowser(sizeStr string) float64 {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	sizeStr = strings.ReplaceAll(sizeStr, ",", "")

	// Extract numeric value
	numPattern := regexp.MustCompile(`([\d.]+)`)
	matches := numPattern.FindStringSubmatch(sizeStr)
	if len(matches) < 2 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	// Convert to square meters based on unit
	switch {
	case strings.Contains(sizeStr, "ha") || strings.Contains(sizeStr, "hectare"):
		return value * 10000
	case strings.Contains(sizeStr, "acre"):
		return value * 4046.86
	case strings.Contains(sizeStr, "m²") || strings.Contains(sizeStr, "sqm") || strings.Contains(sizeStr, "m2"):
		return value
	default:
		return value
	}
}

// Ensure we don't have unused imports
var _ = cdp.Node{}
