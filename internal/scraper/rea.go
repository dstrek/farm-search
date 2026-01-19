package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"farm-search/internal/models"
)

// REAScraper handles scraping from realestate.com.au
type REAScraper struct {
	client    *http.Client
	userAgent string
}

// NewREAScraper creates a new REA scraper
func NewREAScraper() *REAScraper {
	return &REAScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// ScrapeListings scrapes property listings from REA
func (s *REAScraper) ScrapeListings(ctx context.Context, region, propertyType string, maxPages int) ([]models.Property, error) {
	var allListings []models.Property

	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		listings, hasMore, err := s.scrapePage(ctx, region, propertyType, page)
		if err != nil {
			log.Printf("Error scraping page %d: %v", page, err)
			break
		}

		allListings = append(allListings, listings...)

		if !hasMore || len(listings) == 0 {
			break
		}

		// Rate limiting between pages
		time.Sleep(1 * time.Second)
	}

	return allListings, nil
}

func (s *REAScraper) scrapePage(ctx context.Context, region, propertyType string, page int) ([]models.Property, bool, error) {
	// Build the search URL
	// REA URL pattern: https://www.realestate.com.au/buy/property-rural-in-nsw/list-1
	searchURL := fmt.Sprintf(
		"https://www.realestate.com.au/buy/property-%s-in-%s/list-%d",
		propertyType, region, page,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, false, err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	// Parse the HTML/JSON response
	listings, hasMore := s.parseListingsPage(string(body), propertyType)

	return listings, hasMore, nil
}

// parseListingsPage extracts property data from the HTML page
// REA embeds JSON data in script tags which we can parse
func (s *REAScraper) parseListingsPage(html, propertyType string) ([]models.Property, bool) {
	var listings []models.Property

	// Look for the JSON data embedded in the page
	// REA uses a pattern like: window.ArgonautExchange={"..."}
	jsonPattern := regexp.MustCompile(`window\.ArgonautExchange\s*=\s*(\{.+?\});?\s*</script>`)
	matches := jsonPattern.FindStringSubmatch(html)

	if len(matches) < 2 {
		// Try alternative pattern for different page structures
		jsonPattern = regexp.MustCompile(`"listingsMap"\s*:\s*(\{[^}]+\})`)
		matches = jsonPattern.FindStringSubmatch(html)
	}

	// If we can't find embedded JSON, try parsing listing cards from HTML
	if len(matches) < 2 {
		return s.parseListingCards(html, propertyType), false
	}

	// Parse the JSON data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &data); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		return s.parseListingCards(html, propertyType), false
	}

	// Extract listings from the JSON structure
	// The structure varies, so we need to handle different formats
	listings = s.extractListingsFromJSON(data, propertyType)

	// Check if there are more pages
	hasMore := strings.Contains(html, `rel="next"`) || strings.Contains(html, "Next page")

	return listings, hasMore
}

// parseListingCards extracts listings from HTML listing cards
func (s *REAScraper) parseListingCards(html, propertyType string) []models.Property {
	var listings []models.Property

	// Find listing cards - REA uses data-testid attributes
	cardPattern := regexp.MustCompile(`<article[^>]*data-testid="[^"]*listing-card[^"]*"[^>]*>(.*?)</article>`)
	cards := cardPattern.FindAllStringSubmatch(html, -1)

	// Also try the residential-card pattern
	if len(cards) == 0 {
		cardPattern = regexp.MustCompile(`<div[^>]*class="[^"]*residential-card[^"]*"[^>]*>(.*?)</div>\s*</div>\s*</div>`)
		cards = cardPattern.FindAllStringSubmatch(html, -1)
	}

	// Extract listing IDs and basic info using various patterns
	listingIDPattern := regexp.MustCompile(`/property-[^/]+-[^/]+-(\d+)`)
	pricePattern := regexp.MustCompile(`<span[^>]*class="[^"]*price[^"]*"[^>]*>([^<]+)</span>`)
	
	// Find all property links
	linkPattern := regexp.MustCompile(`href="(/property-[^"]+)"`)
	links := linkPattern.FindAllStringSubmatch(html, -1)

	seenIDs := make(map[string]bool)

	for _, link := range links {
		if len(link) < 2 {
			continue
		}

		path := link[1]
		
		// Extract listing ID
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

	// Try to find additional details from the page
	for i := range listings {
		// Look for price near the listing
		for _, match := range pricePattern.FindAllStringSubmatch(html, -1) {
			if len(match) > 1 && listings[i].PriceText.String == "" {
				listings[i].PriceText = sql.NullString{String: strings.TrimSpace(match[1]), Valid: true}
				break
			}
		}
	}

	return listings
}

// parseListingURL extracts info from a property URL
func (s *REAScraper) parseListingURL(path, listingID, propertyType string) *models.Property {
	// URL format: /property-rural-123+example+street-sometown-nsw-2000-12345678
	// Split by - and parse components

	parts := strings.Split(strings.TrimPrefix(path, "/property-"), "-")
	if len(parts) < 3 {
		return nil
	}

	// Last part is the ID, second to last is postcode, before that is state
	// Address components are +-separated

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

	// Try to extract postcode (4 digits)
	postcodePattern := regexp.MustCompile(`-(\d{4})-\d+$`)
	if matches := postcodePattern.FindStringSubmatch(path); len(matches) > 1 {
		listing.Postcode = sql.NullString{String: matches[1], Valid: true}
	}

	// Extract suburb (usually before the state abbreviation)
	suburbPattern := regexp.MustCompile(`-([a-z]+)-nsw-\d{4}`)
	if matches := suburbPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		suburb := strings.ReplaceAll(matches[1], "+", " ")
		suburb = strings.Title(suburb)
		listing.Suburb = sql.NullString{String: suburb, Valid: true}
	}

	// Extract street address
	addressPattern := regexp.MustCompile(`property-[^-]+-(.+)-[a-z]+-nsw-\d{4}`)
	if matches := addressPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		addr := strings.ReplaceAll(matches[1], "+", " ")
		addr = strings.ReplaceAll(addr, "-", " ")
		// Title case the address
		words := strings.Fields(addr)
		for i, word := range words {
			words[i] = strings.Title(word)
		}
		listing.Address = sql.NullString{String: strings.Join(words, " "), Valid: true}
	}

	return listing
}

// extractListingsFromJSON extracts listings from the parsed JSON data
func (s *REAScraper) extractListingsFromJSON(data map[string]interface{}, propertyType string) []models.Property {
	var listings []models.Property

	// Navigate the JSON structure to find listings
	// This varies based on REA's data structure
	
	// Try common paths
	if results, ok := data["rpiResults"].(map[string]interface{}); ok {
		if tieredResults, ok := results["tieredResults"].([]interface{}); ok {
			for _, tier := range tieredResults {
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

func (s *REAScraper) parseJSONListing(data interface{}, propertyType string) *models.Property {
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
	}

	// Extract coordinates
	if geo, ok := m["address"].(map[string]interface{}); ok {
		if location, ok := geo["location"].(map[string]interface{}); ok {
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
				// Parse the size string (e.g., "100 ha", "5000 m²")
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

// parseLandSize converts land size strings to square meters
func parseLandSize(sizeStr string) float64 {
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
		// Assume square meters if no unit
		return value
	}
}

// FetchListingDetails fetches full details for a single listing
func (s *REAScraper) FetchListingDetails(ctx context.Context, listingURL string) (*models.Property, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", listingURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return s.parseListingDetails(string(body), listingURL)
}

func (s *REAScraper) parseListingDetails(html, listingURL string) (*models.Property, error) {
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

// Helper to URL encode address for geocoding
func urlEncode(s string) string {
	return url.QueryEscape(s)
}
