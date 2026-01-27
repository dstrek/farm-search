package scraper

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"farm-search/internal/models"
)

// DomainWebScraper handles scraping from domain.com.au via traditional web scraping
// This is an alternative to the API-based DomainScraper that doesn't require an API key
type DomainWebScraper struct {
	client    *http.Client
	userAgent string
	baseURL   string
}

// NewDomainWebScraper creates a new Domain web scraper
func NewDomainWebScraper() *DomainWebScraper {
	return &DomainWebScraper{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		baseURL:   "https://www.domain.com.au",
	}
}

// DomainWebConfig holds configuration for the web scraper
type DomainWebConfig struct {
	// StartURL is the full URL to start scraping from (with all filters applied)
	// Example: https://www.domain.com.au/sale/illawarra-and-south-coast-nsw/?ptype=vacant-land&price=0-2000000&landsize=100000-any&landsizeunit=ha&sort=dateupdated-desc
	StartURL string
}

// DefaultDomainWebConfig returns default configuration
func DefaultDomainWebConfig() DomainWebConfig {
	return DomainWebConfig{
		// Default search: Illawarra, Southern Highlands, Hunter Valley, Central Coast regions
		// Price up to $2M, land size 10+ hectares, sorted by most recently updated
		StartURL: "https://www.domain.com.au/sale/illawarra-and-south-coast-nsw/?suburb=goulburn-nsw-2580,marulan-nsw-2579,bowral-nsw-2576,berry-nsw-2535,tallong-nsw-2579,kangaroo-valley-nsw-2577,nowra-nsw-2541,katoomba-nsw-2780,lithgow-nsw-2790,cessnock-nsw-2325,mellong-nsw-2756,taralga-nsw-2580,braidwood-nsw-2622&area=southern-highlands-nsw,hunter-valley-upper-nsw,central-coast-and-region-nsw&price=0-2000000&landsize=100000-any&landsizeunit=ha&sort=dateupdated-desc",
	}
}

// ScrapeListings scrapes property listings from Domain website
func (s *DomainWebScraper) ScrapeListings(ctx context.Context, maxPages int) ([]models.Property, error) {
	return s.ScrapeListingsWithConfig(ctx, maxPages, DefaultDomainWebConfig())
}

// ScrapeListingsWithConfig scrapes property listings from Domain website with custom config
func (s *DomainWebScraper) ScrapeListingsWithConfig(ctx context.Context, maxPages int, config DomainWebConfig) ([]models.Property, error) {
	return s.ScrapeListingsWithExistsCheck(ctx, maxPages, config, nil)
}

// ScrapeListingsWithExistsCheck scrapes property listings with optional duplicate detection
func (s *DomainWebScraper) ScrapeListingsWithExistsCheck(ctx context.Context, maxPages int, config DomainWebConfig, existsChecker ExistsChecker) ([]models.Property, error) {
	var allListings []models.Property

	for page := 1; maxPages <= 0 || page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		log.Printf("Scraping Domain (web) page %d...", page)

		listings, hasMore, err := s.scrapePage(ctx, config.StartURL, page)
		if err != nil {
			log.Printf("Error scraping Domain page %d: %v", page, err)
			break
		}

		// Check for duplicates if checker is provided
		if existsChecker != nil && len(listings) > 0 {
			externalIDs := make([]string, len(listings))
			for i, l := range listings {
				externalIDs[i] = l.ExternalID
			}

			existsMap, err := existsChecker(externalIDs)
			if err != nil {
				log.Printf("Warning: failed to check existing properties: %v", err)
			} else {
				newCount := 0
				for _, l := range listings {
					if !existsMap[l.ExternalID] {
						newCount++
					}
				}

				log.Printf("Page %d: %d listings, %d new, %d already scraped", page, len(listings), newCount, len(listings)-newCount)

				// If no new properties, stop pagination (results sorted by newest first)
				if newCount == 0 {
					log.Printf("No new properties found on page %d, stopping pagination", page)
					break
				}
			}
		}

		allListings = append(allListings, listings...)
		log.Printf("Found %d listings on page %d (total: %d)", len(listings), page, len(allListings))

		if !hasMore || len(listings) == 0 {
			break
		}

		// Rate limiting between pages
		time.Sleep(2 * time.Second)
	}

	return allListings, nil
}

func (s *DomainWebScraper) scrapePage(ctx context.Context, startURL string, page int) ([]models.Property, bool, error) {
	// Build the URL with pagination
	searchURL := startURL
	if page > 1 {
		if strings.Contains(searchURL, "?") {
			searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
		} else {
			searchURL = fmt.Sprintf("%s?page=%d", searchURL, page)
		}
	}

	body, err := s.fetch(ctx, searchURL)
	if err != nil {
		return nil, false, err
	}

	// Log page size for debugging
	log.Printf("Received %d bytes from Domain for page %d", len(body), page)

	// Parse the page
	listings, hasMore := s.parsePage(body)

	return listings, hasMore, nil
}

func (s *DomainWebScraper) parsePage(html string) ([]models.Property, bool) {
	var listings []models.Property
	hasMore := false

	// Domain embeds JSON data in the page that we can extract
	// Look for __NEXT_DATA__ script tag (Next.js app)
	// The JSON is large, so we find the start and end markers manually
	startMarker := `<script id="__NEXT_DATA__" type="application/json">`
	endMarker := `</script>`
	if startIdx := strings.Index(html, startMarker); startIdx != -1 {
		jsonStart := startIdx + len(startMarker)
		// Find the closing script tag after the start
		remaining := html[jsonStart:]
		if endIdx := strings.Index(remaining, endMarker); endIdx != -1 {
			jsonStr := remaining[:endIdx]
			listings, hasMore = s.parseNextDataWithPagination(jsonStr)
			if len(listings) > 0 {
				log.Printf("Extracted %d listings from __NEXT_DATA__", len(listings))
			}
		}
	}

	// If no Next.js data, try other patterns
	if len(listings) == 0 {
		// Try to find listing cards in HTML
		listings = s.parseListingCards(html)
		hasMore = s.hasMorePages(html)
	}

	// If still no listings, try extracting from window.__INITIAL_STATE__ or similar
	if len(listings) == 0 {
		listings = s.parseInitialState(html)
		hasMore = s.hasMorePages(html)
	}

	return listings, hasMore
}

// parseNextData extracts listings from Next.js __NEXT_DATA__ script
func (s *DomainWebScraper) parseNextData(jsonStr string) []models.Property {
	listings, _ := s.parseNextDataWithPagination(jsonStr)
	return listings
}

// parseNextDataWithPagination extracts listings and pagination info from Next.js __NEXT_DATA__ script
func (s *DomainWebScraper) parseNextDataWithPagination(jsonStr string) ([]models.Property, bool) {
	var listings []models.Property
	hasMore := false

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		log.Printf("Failed to parse __NEXT_DATA__: %v", err)
		return listings, hasMore
	}

	// Navigate to props -> pageProps -> componentProps -> listingsMap
	props, ok := data["props"].(map[string]interface{})
	if !ok {
		return listings, hasMore
	}

	pageProps, ok := props["pageProps"].(map[string]interface{})
	if !ok {
		return listings, hasMore
	}

	componentProps, ok := pageProps["componentProps"].(map[string]interface{})
	if !ok {
		// Fall back to trying pageProps directly
		return s.tryLegacyPaths(pageProps), hasMore
	}

	// Check pagination from componentProps
	currentPage := 1
	totalPages := 1
	if cp, ok := componentProps["currentPage"].(float64); ok {
		currentPage = int(cp)
	}
	if tp, ok := componentProps["totalPages"].(float64); ok {
		totalPages = int(tp)
	}
	hasMore = currentPage < totalPages

	// Try listingsMap (current Domain structure as of 2026)
	if listingsMap, ok := componentProps["listingsMap"].(map[string]interface{}); ok && len(listingsMap) > 0 {
		for _, v := range listingsMap {
			if listing := s.parseDomainListing(v); listing != nil {
				listings = append(listings, *listing)
			}
		}
		return listings, hasMore
	}

	// Fall back to other paths
	listingsData := s.findListingsInData(componentProps)
	for _, item := range listingsData {
		if listing := s.parseDomainListing(item); listing != nil {
			listings = append(listings, *listing)
		}
	}

	return listings, hasMore
}

// tryLegacyPaths tries alternative data paths for older page structures
func (s *DomainWebScraper) tryLegacyPaths(pageProps map[string]interface{}) []models.Property {
	var listings []models.Property
	listingsData := s.findListingsInData(pageProps)
	for _, item := range listingsData {
		if listing := s.parseDomainListing(item); listing != nil {
			listings = append(listings, *listing)
		}
	}
	return listings
}

// parseDomainListing parses Domain's specific listing structure
func (s *DomainWebScraper) parseDomainListing(data interface{}) *models.Property {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	now := time.Now()
	listing := &models.Property{
		Source:    "domain-web",
		State:     "NSW",
		ScrapedAt: now,
		UpdatedAt: now,
	}

	// Extract ID
	if id, ok := m["id"].(float64); ok {
		listing.ExternalID = strconv.FormatInt(int64(id), 10)
	} else if id, ok := m["id"].(string); ok {
		listing.ExternalID = id
	}

	if listing.ExternalID == "" {
		return nil
	}

	// Get the listingModel which contains most of the data
	listingModel, ok := m["listingModel"].(map[string]interface{})
	if !ok {
		// Fall back to flat structure
		return s.parseListingData(m)
	}

	// Extract URL from listingModel
	if url, ok := listingModel["url"].(string); ok {
		if !strings.HasPrefix(url, "http") {
			url = s.baseURL + url
		}
		listing.URL = url
	} else {
		listing.URL = fmt.Sprintf("%s/listing/%s", s.baseURL, listing.ExternalID)
	}

	// Extract address from listingModel
	if addr, ok := listingModel["address"].(map[string]interface{}); ok {
		// Build full address from street and suburb
		var addrParts []string
		if street, ok := addr["street"].(string); ok && street != "" {
			addrParts = append(addrParts, street)
		}
		if len(addrParts) > 0 {
			listing.Address = sql.NullString{String: strings.Join(addrParts, " "), Valid: true}
		}

		if suburb, ok := addr["suburb"].(string); ok {
			listing.Suburb = sql.NullString{String: suburb, Valid: true}
		}
		if state, ok := addr["state"].(string); ok {
			listing.State = state
		}
		if postcode, ok := addr["postcode"].(string); ok {
			listing.Postcode = sql.NullString{String: postcode, Valid: true}
		}

		// Extract coordinates
		if lat, ok := addr["lat"].(float64); ok {
			listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
		}
		if lng, ok := addr["lng"].(float64); ok {
			listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
		}
	}

	// Extract price
	if price, ok := listingModel["price"].(string); ok {
		listing.PriceText = sql.NullString{String: price, Valid: true}
	}

	// Extract features (land size, property type, bedrooms, bathrooms)
	if features, ok := listingModel["features"].(map[string]interface{}); ok {
		// Property type
		if propType, ok := features["propertyType"].(string); ok {
			listing.PropertyType = sql.NullString{String: propType, Valid: true}
		} else if propType, ok := features["propertyTypeFormatted"].(string); ok {
			listing.PropertyType = sql.NullString{String: propType, Valid: true}
		}

		// Land size (Domain provides it in hectares with landUnit)
		if landSize, ok := features["landSize"].(float64); ok && landSize > 0 {
			landUnit := "ha" // default
			if unit, ok := features["landUnit"].(string); ok {
				landUnit = unit
			}
			// Convert to sqm
			var sqm float64
			switch strings.ToLower(landUnit) {
			case "ha", "hectare", "hectares":
				sqm = landSize * 10000
			case "ac", "acre", "acres":
				sqm = landSize * 4046.86
			case "m2", "sqm", "m²":
				sqm = landSize
			default:
				// Assume hectares for large values, sqm for small
				if landSize > 100 {
					sqm = landSize // assume sqm
				} else {
					sqm = landSize * 10000 // assume hectares
				}
			}
			listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
		}

		// Bedrooms/bathrooms
		if beds, ok := features["beds"].(float64); ok && beds > 0 {
			listing.Bedrooms = sql.NullInt64{Int64: int64(beds), Valid: true}
		}
		if baths, ok := features["baths"].(float64); ok && baths > 0 {
			listing.Bathrooms = sql.NullInt64{Int64: int64(baths), Valid: true}
		}
	}

	// Extract images
	var images []string
	if imgList, ok := listingModel["images"].([]interface{}); ok {
		for _, img := range imgList {
			if url, ok := img.(string); ok && url != "" {
				images = append(images, url)
			}
		}
	}
	if len(images) > 0 {
		imgJSON, _ := json.Marshal(images)
		listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
	}

	return listing
}

// findListingsInData recursively searches for listings array in the data
func (s *DomainWebScraper) findListingsInData(data map[string]interface{}) []interface{} {
	// Try common paths
	paths := [][]string{
		{"listingsMap"},
		{"listings"},
		{"componentProps", "listingsMap"},
		{"componentProps", "listings"},
		{"searchResults", "listings"},
		{"searchResults", "results"},
		{"results"},
		{"data", "listings"},
	}

	for _, path := range paths {
		current := data
		found := true
		for i, key := range path {
			if i == len(path)-1 {
				// Last element - could be array or map
				if arr, ok := current[key].([]interface{}); ok {
					return arr
				}
				// If it's a map, extract values
				if m, ok := current[key].(map[string]interface{}); ok {
					var items []interface{}
					for _, v := range m {
						items = append(items, v)
					}
					return items
				}
			} else {
				if next, ok := current[key].(map[string]interface{}); ok {
					current = next
				} else {
					found = false
					break
				}
			}
		}
		if !found {
			continue
		}
	}

	// If not found in known paths, search recursively
	return s.searchForListingsArray(data, 0)
}

// searchForListingsArray recursively searches for arrays that look like listings
func (s *DomainWebScraper) searchForListingsArray(data interface{}, depth int) []interface{} {
	if depth > 10 {
		return nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this map has listing-like keys
		if _, hasID := v["listingId"]; hasID {
			return []interface{}{v}
		}
		if _, hasID := v["id"]; hasID {
			if _, hasAddress := v["address"]; hasAddress {
				return []interface{}{v}
			}
		}
		// Search children
		for key, val := range v {
			// Skip large arrays that aren't listings
			if key == "features" || key == "media" || key == "images" {
				continue
			}
			if result := s.searchForListingsArray(val, depth+1); len(result) > 0 {
				return result
			}
		}
	case []interface{}:
		// Check if this looks like a listings array
		if len(v) > 0 {
			first, ok := v[0].(map[string]interface{})
			if ok {
				if _, hasID := first["listingId"]; hasID {
					return v
				}
				if _, hasID := first["id"]; hasID {
					if _, hasAddress := first["address"]; hasAddress {
						return v
					}
					if _, hasPrice := first["price"]; hasPrice {
						return v
					}
				}
			}
		}
	}

	return nil
}

// parseListingData converts a listing object to a Property
func (s *DomainWebScraper) parseListingData(data interface{}) *models.Property {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	now := time.Now()
	listing := &models.Property{
		Source:    "domain-web",
		State:     "NSW",
		ScrapedAt: now,
		UpdatedAt: now,
	}

	// Extract ID (try various field names)
	if id, ok := m["listingId"].(float64); ok {
		listing.ExternalID = strconv.FormatInt(int64(id), 10)
	} else if id, ok := m["listingId"].(string); ok {
		listing.ExternalID = id
	} else if id, ok := m["id"].(float64); ok {
		listing.ExternalID = strconv.FormatInt(int64(id), 10)
	} else if id, ok := m["id"].(string); ok {
		listing.ExternalID = id
	}

	if listing.ExternalID == "" {
		return nil
	}

	// Extract URL
	if listingURL, ok := m["listingUrl"].(string); ok {
		if !strings.HasPrefix(listingURL, "http") {
			listingURL = s.baseURL + listingURL
		}
		listing.URL = listingURL
	} else if slug, ok := m["listingSlug"].(string); ok {
		listing.URL = fmt.Sprintf("%s/%s", s.baseURL, slug)
	} else {
		listing.URL = fmt.Sprintf("%s/listing/%s", s.baseURL, listing.ExternalID)
	}

	// Extract address
	if addr, ok := m["address"].(map[string]interface{}); ok {
		s.extractAddress(addr, listing)
	} else if addr, ok := m["displayAddress"].(string); ok {
		listing.Address = sql.NullString{String: addr, Valid: true}
	}

	// Extract price
	if price, ok := m["price"].(string); ok {
		listing.PriceText = sql.NullString{String: price, Valid: true}
	} else if priceObj, ok := m["price"].(map[string]interface{}); ok {
		if display, ok := priceObj["displayPrice"].(string); ok {
			listing.PriceText = sql.NullString{String: display, Valid: true}
		}
	}

	// Extract property type
	if propType, ok := m["propertyType"].(string); ok {
		listing.PropertyType = sql.NullString{String: propType, Valid: true}
	} else if propTypes, ok := m["propertyTypes"].([]interface{}); ok && len(propTypes) > 0 {
		if pt, ok := propTypes[0].(string); ok {
			listing.PropertyType = sql.NullString{String: pt, Valid: true}
		}
	}

	// Extract features (bedrooms, bathrooms)
	if beds, ok := m["bedrooms"].(float64); ok && beds > 0 {
		listing.Bedrooms = sql.NullInt64{Int64: int64(beds), Valid: true}
	}
	if baths, ok := m["bathrooms"].(float64); ok && baths > 0 {
		listing.Bathrooms = sql.NullInt64{Int64: int64(baths), Valid: true}
	}

	// Extract land size
	if landSize, ok := m["landAreaSqm"].(float64); ok && landSize > 0 {
		listing.LandSizeSqm = sql.NullFloat64{Float64: landSize, Valid: true}
	} else if landSize, ok := m["landSize"].(float64); ok && landSize > 0 {
		listing.LandSizeSqm = sql.NullFloat64{Float64: landSize, Valid: true}
	} else if landStr, ok := m["landArea"].(string); ok {
		if sqm := parseLandSize(landStr); sqm > 0 {
			listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
		}
	}

	// Extract coordinates
	if lat, ok := m["latitude"].(float64); ok {
		listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
	}
	if lng, ok := m["longitude"].(float64); ok {
		listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
	}
	// Try geoLocation object
	if geo, ok := m["geoLocation"].(map[string]interface{}); ok {
		if lat, ok := geo["latitude"].(float64); ok {
			listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
		}
		if lng, ok := geo["longitude"].(float64); ok {
			listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
		}
	}

	// Extract images
	var images []string
	if mediaList, ok := m["media"].([]interface{}); ok {
		for _, media := range mediaList {
			if mediaObj, ok := media.(map[string]interface{}); ok {
				if url, ok := mediaObj["url"].(string); ok && url != "" {
					images = append(images, url)
				}
			}
		}
	} else if imgList, ok := m["images"].([]interface{}); ok {
		for _, img := range imgList {
			if url, ok := img.(string); ok && url != "" {
				images = append(images, url)
			} else if imgObj, ok := img.(map[string]interface{}); ok {
				if url, ok := imgObj["url"].(string); ok && url != "" {
					images = append(images, url)
				}
			}
		}
	}
	if len(images) > 0 {
		imgJSON, _ := json.Marshal(images)
		listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
	}

	// Extract description/headline
	if headline, ok := m["headline"].(string); ok {
		listing.Description = sql.NullString{String: headline, Valid: true}
	} else if desc, ok := m["description"].(string); ok {
		listing.Description = sql.NullString{String: desc, Valid: true}
	}

	return listing
}

func (s *DomainWebScraper) extractAddress(addr map[string]interface{}, listing *models.Property) {
	// Try displayAddress first
	if display, ok := addr["displayAddress"].(string); ok {
		listing.Address = sql.NullString{String: display, Valid: true}
	} else {
		// Build from components
		var parts []string
		if streetNum, ok := addr["streetNumber"].(string); ok && streetNum != "" {
			parts = append(parts, streetNum)
		}
		if street, ok := addr["street"].(string); ok && street != "" {
			parts = append(parts, street)
		}
		if len(parts) > 0 {
			listing.Address = sql.NullString{String: strings.Join(parts, " "), Valid: true}
		}
	}

	// Extract suburb
	if suburb, ok := addr["suburb"].(string); ok {
		listing.Suburb = sql.NullString{String: suburb, Valid: true}
	}

	// Extract state
	if state, ok := addr["state"].(string); ok {
		listing.State = state
	}

	// Extract postcode
	if postcode, ok := addr["postcode"].(string); ok {
		listing.Postcode = sql.NullString{String: postcode, Valid: true}
	} else if postcode, ok := addr["postcode"].(float64); ok {
		listing.Postcode = sql.NullString{String: strconv.FormatInt(int64(postcode), 10), Valid: true}
	}

	// Extract coordinates from address object if available
	if lat, ok := addr["latitude"].(float64); ok {
		listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
	}
	if lng, ok := addr["longitude"].(float64); ok {
		listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
	}
}

// parseListingCards extracts listings from HTML listing cards
func (s *DomainWebScraper) parseListingCards(html string) []models.Property {
	var listings []models.Property

	// Find listing links with IDs
	// Domain URLs are like: /123-main-street-suburb-nsw-2000-12345678
	listingPattern := regexp.MustCompile(`href="(/[^"]*-(\d{7,10}))"`)
	matches := listingPattern.FindAllStringSubmatch(html, -1)

	seenIDs := make(map[string]bool)
	now := time.Now()

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		path := match[1]
		listingID := match[2]

		// Skip non-listing paths
		if strings.Contains(path, "/suburb-profile") ||
			strings.Contains(path, "/news/") ||
			strings.Contains(path, "/advice/") ||
			strings.Contains(path, "/agent/") {
			continue
		}

		if seenIDs[listingID] {
			continue
		}
		seenIDs[listingID] = true

		listing := &models.Property{
			ExternalID: listingID,
			Source:     "domain-web",
			URL:        s.baseURL + path,
			State:      "NSW",
			ScrapedAt:  now,
			UpdatedAt:  now,
		}

		// Try to extract address from URL
		s.extractAddressFromURL(path, listing)

		listings = append(listings, *listing)
	}

	return listings
}

func (s *DomainWebScraper) extractAddressFromURL(path string, listing *models.Property) {
	// URL format: /123-main-street-suburb-state-postcode-listingid
	// Remove leading slash and listing ID
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "-")

	if len(parts) < 4 {
		return
	}

	// Last part is the listing ID, before that is postcode
	// Try to find the postcode (4 digits)
	for i := len(parts) - 2; i >= 0; i-- {
		if len(parts[i]) == 4 {
			if _, err := strconv.Atoi(parts[i]); err == nil {
				listing.Postcode = sql.NullString{String: parts[i], Valid: true}

				// State is likely before postcode
				if i > 0 {
					state := strings.ToUpper(parts[i-1])
					if state == "NSW" || state == "VIC" || state == "QLD" || state == "SA" || state == "WA" || state == "TAS" || state == "NT" || state == "ACT" {
						listing.State = state
					}
				}

				// Suburb is typically 1-3 words before state
				if i > 1 {
					suburbEnd := i - 1
					if listing.State != "" {
						suburbEnd = i - 2
					}
					if suburbEnd > 0 && suburbEnd < len(parts) {
						// Find where suburb starts (after street address)
						// Usually suburb is 1-2 words
						suburbStart := suburbEnd
						if suburbEnd > 0 {
							suburbStart = suburbEnd - 1
						}
						suburb := strings.Join(parts[suburbStart:suburbEnd+1], " ")
						suburb = strings.Title(suburb)
						listing.Suburb = sql.NullString{String: suburb, Valid: true}
					}
				}

				break
			}
		}
	}
}

// parseInitialState extracts listings from window.__INITIAL_STATE__ or similar patterns
func (s *DomainWebScraper) parseInitialState(html string) []models.Property {
	var listings []models.Property

	// Try various patterns for embedded state
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(\{.*?\});?\s*</script>`),
		regexp.MustCompile(`window\.__data__\s*=\s*(\{.*?\});?\s*</script>`),
		regexp.MustCompile(`window\.pageData\s*=\s*(\{.*?\});?\s*</script>`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(html); len(matches) >= 2 {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
				// Search for listings in the data
				listingsData := s.findListingsInData(data)
				for _, item := range listingsData {
					if listing := s.parseListingData(item); listing != nil {
						listings = append(listings, *listing)
					}
				}
				if len(listings) > 0 {
					log.Printf("Extracted %d listings from embedded state", len(listings))
					return listings
				}
			}
		}
	}

	return listings
}

func (s *DomainWebScraper) hasMorePages(html string) bool {
	// Check for pagination indicators
	return strings.Contains(html, `rel="next"`) ||
		strings.Contains(html, `aria-label="Next page"`) ||
		strings.Contains(html, `data-testid="paginator-next"`) ||
		regexp.MustCompile(`page=\d+[^"]*"[^>]*>Next`).MatchString(html)
}

func (s *DomainWebScraper) fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Handle gzip encoding
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// FetchListingDetails fetches full details for a single listing
func (s *DomainWebScraper) FetchListingDetails(ctx context.Context, listingURL string) (*models.Property, error) {
	body, err := s.fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	return s.parseListingDetails(body, listingURL)
}

func (s *DomainWebScraper) parseListingDetails(html, listingURL string) (*models.Property, error) {
	// Extract listing ID from URL
	idPattern := regexp.MustCompile(`-(\d{7,10})$`)
	matches := idPattern.FindStringSubmatch(listingURL)
	if len(matches) < 2 {
		// Try to find ID in the URL path
		idPattern2 := regexp.MustCompile(`/(\d{7,10})(?:\?|$)`)
		matches = idPattern2.FindStringSubmatch(listingURL)
		if len(matches) < 2 {
			return nil, fmt.Errorf("could not extract listing ID from URL")
		}
	}

	now := time.Now()
	listing := &models.Property{
		ExternalID: matches[1],
		Source:     "domain-web",
		URL:        listingURL,
		State:      "NSW",
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Try to find JSON-LD data first
	jsonLDPattern := regexp.MustCompile(`<script type="application/ld\+json">(\{.*?"@type"\s*:\s*"(?:RealEstateListing|Product|Residence)".*?\})</script>`)
	if matches := jsonLDPattern.FindStringSubmatch(html); len(matches) >= 2 {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			s.extractFromJSONLD(data, listing)
		}
	}

	// Try __NEXT_DATA__ for more details
	nextDataPattern := regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(\{.*?\})</script>`)
	if matches := nextDataPattern.FindStringSubmatch(html); len(matches) >= 2 {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			s.extractDetailsFromNextData(data, listing)
		}
	}

	// Extract additional details from HTML if not found in JSON
	s.extractDetailsFromHTML(html, listing)

	return listing, nil
}

func (s *DomainWebScraper) extractFromJSONLD(data map[string]interface{}, listing *models.Property) {
	if name, ok := data["name"].(string); ok && !listing.Address.Valid {
		listing.Address = sql.NullString{String: name, Valid: true}
	}
	if desc, ok := data["description"].(string); ok && !listing.Description.Valid {
		listing.Description = sql.NullString{String: desc, Valid: true}
	}

	if addr, ok := data["address"].(map[string]interface{}); ok {
		if locality, ok := addr["addressLocality"].(string); ok && !listing.Suburb.Valid {
			listing.Suburb = sql.NullString{String: locality, Valid: true}
		}
		if postcode, ok := addr["postalCode"].(string); ok && !listing.Postcode.Valid {
			listing.Postcode = sql.NullString{String: postcode, Valid: true}
		}
		if region, ok := addr["addressRegion"].(string); ok {
			listing.State = region
		}
	}

	if geo, ok := data["geo"].(map[string]interface{}); ok {
		if lat, ok := geo["latitude"].(float64); ok && !listing.Latitude.Valid {
			listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
		}
		if lng, ok := geo["longitude"].(float64); ok && !listing.Longitude.Valid {
			listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
		}
	}

	// Extract images
	if !listing.Images.Valid {
		var images []string
		if imgList, ok := data["image"].([]interface{}); ok {
			for _, img := range imgList {
				if url, ok := img.(string); ok {
					images = append(images, url)
				}
			}
		} else if imgURL, ok := data["image"].(string); ok {
			images = append(images, imgURL)
		}
		if len(images) > 0 {
			imgJSON, _ := json.Marshal(images)
			listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
		}
	}
}

func (s *DomainWebScraper) extractDetailsFromNextData(data map[string]interface{}, listing *models.Property) {
	// Navigate to find listing details
	props, ok := data["props"].(map[string]interface{})
	if !ok {
		return
	}

	pageProps, ok := props["pageProps"].(map[string]interface{})
	if !ok {
		return
	}

	// Try to find listing object
	var listingData map[string]interface{}
	if ld, ok := pageProps["listing"].(map[string]interface{}); ok {
		listingData = ld
	} else if ld, ok := pageProps["listingDetails"].(map[string]interface{}); ok {
		listingData = ld
	}

	if listingData == nil {
		return
	}

	// Extract details
	if addr, ok := listingData["address"].(map[string]interface{}); ok {
		s.extractAddress(addr, listing)
	}

	if price, ok := listingData["price"].(string); ok && !listing.PriceText.Valid {
		listing.PriceText = sql.NullString{String: price, Valid: true}
	}

	if desc, ok := listingData["description"].(string); ok && !listing.Description.Valid {
		listing.Description = sql.NullString{String: desc, Valid: true}
	}

	if beds, ok := listingData["bedrooms"].(float64); ok && !listing.Bedrooms.Valid && beds > 0 {
		listing.Bedrooms = sql.NullInt64{Int64: int64(beds), Valid: true}
	}
	if baths, ok := listingData["bathrooms"].(float64); ok && !listing.Bathrooms.Valid && baths > 0 {
		listing.Bathrooms = sql.NullInt64{Int64: int64(baths), Valid: true}
	}
	if landSize, ok := listingData["landAreaSqm"].(float64); ok && !listing.LandSizeSqm.Valid && landSize > 0 {
		listing.LandSizeSqm = sql.NullFloat64{Float64: landSize, Valid: true}
	}
}

func (s *DomainWebScraper) extractDetailsFromHTML(html string, listing *models.Property) {
	// Extract price if not found
	if !listing.PriceText.Valid {
		pricePatterns := []*regexp.Regexp{
			regexp.MustCompile(`<[^>]*data-testid="listing-details__summary-title"[^>]*>([^<]+)</`),
			regexp.MustCompile(`<span[^>]*class="[^"]*css-[^"]*"[^>]*>\s*(\$[\d,]+(?:\s*-\s*\$[\d,]+)?)\s*</span>`),
			regexp.MustCompile(`>\s*(\$[\d,]+(?:\s*-\s*\$[\d,]+)?)\s*<`),
		}
		for _, pattern := range pricePatterns {
			if matches := pattern.FindStringSubmatch(html); len(matches) > 1 {
				price := strings.TrimSpace(matches[1])
				if strings.Contains(price, "$") {
					listing.PriceText = sql.NullString{String: price, Valid: true}
					break
				}
			}
		}
	}

	// Extract land size if not found
	if !listing.LandSizeSqm.Valid {
		landPatterns := []*regexp.Regexp{
			regexp.MustCompile(`([\d,]+(?:\.\d+)?)\s*(hectares?|ha)\b`),
			regexp.MustCompile(`([\d,]+(?:\.\d+)?)\s*(acres?)\b`),
			regexp.MustCompile(`Land[:\s]*([\d,]+)\s*(m²|sqm|m2)`),
		}
		for _, pattern := range landPatterns {
			if matches := pattern.FindStringSubmatch(strings.ToLower(html)); len(matches) >= 2 {
				sizeStr := matches[1]
				unit := ""
				if len(matches) >= 3 {
					unit = matches[2]
				}
				if sqm := parseLandSize(sizeStr + " " + unit); sqm > 0 {
					listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
					break
				}
			}
		}
	}

	// Extract bedrooms/bathrooms if not found
	if !listing.Bedrooms.Valid {
		bedPattern := regexp.MustCompile(`(\d+)\s*(?:bed|bedroom)`)
		if matches := bedPattern.FindStringSubmatch(strings.ToLower(html)); len(matches) >= 2 {
			if beds, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				listing.Bedrooms = sql.NullInt64{Int64: beds, Valid: true}
			}
		}
	}
	if !listing.Bathrooms.Valid {
		bathPattern := regexp.MustCompile(`(\d+)\s*(?:bath|bathroom)`)
		if matches := bathPattern.FindStringSubmatch(strings.ToLower(html)); len(matches) >= 2 {
			if baths, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				listing.Bathrooms = sql.NullInt64{Int64: baths, Valid: true}
			}
		}
	}

	// Extract images if not found
	if !listing.Images.Valid {
		var images []string
		imgPattern := regexp.MustCompile(`"(https://[^"]*rimages\.domain\.com\.au[^"]+\.(?:jpg|jpeg|png|webp))"`)
		seenImages := make(map[string]bool)
		for _, match := range imgPattern.FindAllStringSubmatch(html, -1) {
			if len(match) >= 2 {
				imgURL := match[1]
				// Skip thumbnails
				if strings.Contains(imgURL, "thumbnail") || strings.Contains(imgURL, "50x50") {
					continue
				}
				if !seenImages[imgURL] {
					seenImages[imgURL] = true
					images = append(images, imgURL)
				}
			}
		}
		if len(images) > 0 {
			imgJSON, _ := json.Marshal(images)
			listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
		}
	}
}
