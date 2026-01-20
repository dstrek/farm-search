package scraper

import (
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

// FarmBuyScraper handles scraping from farmbuy.com
type FarmBuyScraper struct {
	client    *http.Client
	userAgent string
	baseURL   string
}

// NewFarmBuyScraper creates a new FarmBuy scraper
func NewFarmBuyScraper() *FarmBuyScraper {
	return &FarmBuyScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		baseURL:   "https://farmbuy.com",
	}
}

// FarmBuy embedded JSON structure
type farmBuyListing struct {
	ID         string `json:"id"`
	ExternalID string `json:"externalId"`
	URL        string `json:"url"`
	PriceText  string `json:"priceText"`
	LandArea   string `json:"landArea"`
	Address    struct {
		Full     string `json:"full"`
		State    string `json:"state"`
		Postcode string `json:"postcode"`
		Suburb   string `json:"suburb"`
	} `json:"address"`
	Types            []string    `json:"types"`
	MainTileImageURL interface{} `json:"mainTileImageURL"` // Can be string or bool
	Meta             struct {
		Bed  string `json:"bed"`
		Bath string `json:"bath"`
		Car  string `json:"car"`
	} `json:"meta"`
}

// ScrapeListings scrapes property listings from FarmBuy
func (s *FarmBuyScraper) ScrapeListings(ctx context.Context, state string, maxPages int) ([]models.Property, error) {
	var allListings []models.Property

	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		log.Printf("Scraping FarmBuy page %d for %s...", page, state)

		listings, hasMore, err := s.scrapePage(ctx, state, page)
		if err != nil {
			log.Printf("Error scraping FarmBuy page %d: %v", page, err)
			break
		}

		allListings = append(allListings, listings...)
		log.Printf("Found %d listings on page %d (total: %d)", len(listings), page, len(allListings))

		if !hasMore || len(listings) == 0 {
			break
		}

		// Rate limiting between pages
		time.Sleep(1 * time.Second)
	}

	return allListings, nil
}

func (s *FarmBuyScraper) scrapePage(ctx context.Context, state string, page int) ([]models.Property, bool, error) {
	// Build the search URL
	// FarmBuy uses query parameters: sort=datedesc for most recent, page=N for pagination
	searchURL := fmt.Sprintf("%s/state/%s?sort=datedesc", s.baseURL, strings.ToLower(state))
	if page > 1 {
		searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
	}

	body, err := s.fetch(ctx, searchURL)
	if err != nil {
		return nil, false, err
	}

	// Extract embedded JSON from property tiles
	// Pattern: <li data-propertyid="123456"...><script type="application/json"> {...} </script>
	jsonPattern := regexp.MustCompile(`<li data-propertyid="(\d+)"[^>]*>.*?<script type="application/json">\s*(\{[^<]+\})\s*</script>`)
	matches := jsonPattern.FindAllStringSubmatch(body, -1)

	var listings []models.Property
	seenIDs := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		propertyID := match[1]
		jsonStr := match[2]

		if seenIDs[propertyID] {
			continue
		}
		seenIDs[propertyID] = true

		var data farmBuyListing
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			log.Printf("Error parsing FarmBuy JSON for %s: %v", propertyID, err)
			continue
		}

		listing := s.convertListing(&data)
		if listing != nil {
			// Fetch detail page to get all images and description
			if data.URL != "" {
				images, description, err := s.fetchDetailPage(ctx, data.URL)
				if err != nil {
					log.Printf("Error fetching detail page for %s: %v", propertyID, err)
				} else {
					if len(images) > 0 {
						imgJSON, _ := json.Marshal(images)
						listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
					}
					if description != "" {
						listing.Description = sql.NullString{String: description, Valid: true}
					}
				}
				// Rate limiting between detail fetches
				time.Sleep(300 * time.Millisecond)
			}
			listings = append(listings, *listing)
		}
	}

	// If no embedded JSON found, fall back to fetching coordinates from map markers
	if len(listings) == 0 {
		log.Printf("No embedded JSON found, trying map markers...")
		listings = s.extractFromMapMarkers(body, state)
	} else {
		// Enrich listings with coordinates from map markers
		s.enrichWithCoordinates(body, listings)
	}

	// Check if there are more pages (pagination uses query params: ?page=N)
	hasMore := strings.Contains(body, fmt.Sprintf("page=%d", page+1)) ||
		strings.Contains(body, `rel="next"`)

	return listings, hasMore, nil
}

func (s *FarmBuyScraper) convertListing(data *farmBuyListing) *models.Property {
	if data.ID == "" {
		return nil
	}

	now := time.Now()
	listing := &models.Property{
		ExternalID: data.ID,
		Source:     "farmbuy",
		URL:        data.URL,
		State:      data.Address.State,
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Address
	if data.Address.Full != "" {
		listing.Address = sql.NullString{String: data.Address.Full, Valid: true}
	}
	if data.Address.Suburb != "" {
		listing.Suburb = sql.NullString{String: data.Address.Suburb, Valid: true}
	}
	if data.Address.Postcode != "" {
		listing.Postcode = sql.NullString{String: data.Address.Postcode, Valid: true}
	}
	if data.Address.State == "" {
		listing.State = "NSW"
	}

	// Price
	if data.PriceText != "" {
		listing.PriceText = sql.NullString{String: data.PriceText, Valid: true}
	}

	// Property type
	if len(data.Types) > 0 {
		listing.PropertyType = sql.NullString{String: strings.ToLower(data.Types[0]), Valid: true}
	} else {
		listing.PropertyType = sql.NullString{String: "rural", Valid: true}
	}

	// Land size
	if data.LandArea != "" {
		sqm := parseLandSizeString(data.LandArea)
		if sqm > 0 {
			listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
		}
	}

	// Bedrooms/Bathrooms
	if data.Meta.Bed != "" {
		if beds, err := strconv.ParseFloat(data.Meta.Bed, 64); err == nil && beds > 0 {
			listing.Bedrooms = sql.NullInt64{Int64: int64(beds), Valid: true}
		}
	}
	if data.Meta.Bath != "" {
		if baths, err := strconv.ParseFloat(data.Meta.Bath, 64); err == nil && baths > 0 {
			listing.Bathrooms = sql.NullInt64{Int64: int64(baths), Valid: true}
		}
	}

	// Image (can be string or bool)
	if imgURL, ok := data.MainTileImageURL.(string); ok && imgURL != "" {
		images := []string{imgURL}
		imgJSON, _ := json.Marshal(images)
		listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
	}

	return listing
}

// enrichWithCoordinates extracts coordinates from map markers and adds them to listings
func (s *FarmBuyScraper) enrichWithCoordinates(body string, listings []models.Property) {
	// Map markers format: <figure class="marker" data-lat="-32.872" data-lng="149.392">
	//   <figcaption><div class="propertyMapTile" data-property-id="363892">...

	// Split by figure tags and process each one
	coords := make(map[string][2]float64)

	// Find figure start positions
	figureStarts := regexp.MustCompile(`<figure[^>]*class="marker"[^>]*data-lat="([^"]+)"[^>]*data-lng="([^"]+)"`)
	figureMatches := figureStarts.FindAllStringSubmatchIndex(body, -1)

	propIDPattern := regexp.MustCompile(`data-property-id="(\d+)"`)

	for _, match := range figureMatches {
		if len(match) < 6 {
			continue
		}

		lat, _ := strconv.ParseFloat(body[match[2]:match[3]], 64)
		lng, _ := strconv.ParseFloat(body[match[4]:match[5]], 64)

		if lat == 0 || lng == 0 {
			continue
		}

		// Look for property-id in the next ~2000 characters (within this figure element)
		searchEnd := match[1] + 2000
		if searchEnd > len(body) {
			searchEnd = len(body)
		}
		segment := body[match[1]:searchEnd]

		if idMatch := propIDPattern.FindStringSubmatch(segment); len(idMatch) > 1 {
			coords[idMatch[1]] = [2]float64{lat, lng}
		}
	}

	log.Printf("Found coordinates for %d properties from map markers", len(coords))

	// Enrich listings
	enriched := 0
	for i := range listings {
		if c, ok := coords[listings[i].ExternalID]; ok {
			listings[i].Latitude = sql.NullFloat64{Float64: c[0], Valid: true}
			listings[i].Longitude = sql.NullFloat64{Float64: c[1], Valid: true}
			enriched++
		}
	}
	log.Printf("Enriched %d/%d listings with coordinates", enriched, len(listings))
}

// extractFromMapMarkers extracts basic listing info from map markers when JSON is not available
func (s *FarmBuyScraper) extractFromMapMarkers(body, state string) []models.Property {
	var listings []models.Property

	// Find map markers with property info
	markerPattern := regexp.MustCompile(`<figure class="marker"[^>]*data-lat="([^"]+)"[^>]*data-lng="([^"]+)"[^>]*>.*?data-property-id="(\d+)".*?href="([^"]+)".*?<span class="suburb">([^<]*)</span>.*?<span class="streetAddress">([^<]*)</span>`)
	matches := markerPattern.FindAllStringSubmatch(body, -1)

	seenIDs := make(map[string]bool)
	now := time.Now()

	for _, match := range matches {
		if len(match) < 7 {
			continue
		}

		lat, _ := strconv.ParseFloat(match[1], 64)
		lng, _ := strconv.ParseFloat(match[2], 64)
		id := match[3]
		url := match[4]
		suburb := match[5]
		street := match[6]

		if seenIDs[id] {
			continue
		}
		seenIDs[id] = true

		listing := models.Property{
			ExternalID:   id,
			Source:       "farmbuy",
			URL:          url,
			State:        strings.ToUpper(state),
			ScrapedAt:    now,
			UpdatedAt:    now,
			PropertyType: sql.NullString{String: "rural", Valid: true},
		}

		if lat != 0 && lng != 0 {
			listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
			listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
		}
		if suburb != "" {
			listing.Suburb = sql.NullString{String: suburb, Valid: true}
		}
		if street != "" {
			listing.Address = sql.NullString{String: street, Valid: true}
		}

		listings = append(listings, listing)
	}

	return listings
}

// parseLandSizeString converts land size strings like "280ha" or "691.90ac" to square meters
func parseLandSizeString(sizeStr string) float64 {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	sizeStr = strings.ReplaceAll(sizeStr, ",", "")

	// Extract numeric value and unit
	numPattern := regexp.MustCompile(`([\d.]+)\s*(ha|hectares?|ac|acres?)`)
	matches := numPattern.FindStringSubmatch(sizeStr)
	if len(matches) < 3 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := matches[2]
	switch {
	case strings.HasPrefix(unit, "ha") || strings.HasPrefix(unit, "hectare"):
		return value * 10000
	case strings.HasPrefix(unit, "ac") || strings.HasPrefix(unit, "acre"):
		return value * 4046.86
	default:
		return 0
	}
}

// fetchDetailPage fetches images and description from a property detail page
func (s *FarmBuyScraper) fetchDetailPage(ctx context.Context, url string) ([]string, string, error) {
	body, err := s.fetch(ctx, url)
	if err != nil {
		return nil, "", err
	}

	// Extract full-size images from farmbuycdn (1920_ prefix for full size)
	// Pattern: farmbuycdn.clodflare.pushcreative.com.au/PROPERTYID/1920_*
	imgPattern := regexp.MustCompile(`https://farmbuycdn\.clodflare\.pushcreative\.com\.au/\d+/1920_[^"]+\.(jpg|jpeg|png|webp)`)
	matches := imgPattern.FindAllString(body, -1)

	// Deduplicate images
	seen := make(map[string]bool)
	var images []string
	for _, img := range matches {
		if !seen[img] {
			seen[img] = true
			images = append(images, img)
		}
	}

	// Extract description from <div id="propertyprofile" class="user-content">
	description := s.extractDescription(body)

	return images, description, nil
}

// extractDescription extracts the property description from HTML
func (s *FarmBuyScraper) extractDescription(body string) string {
	// Find the propertyprofile div content
	// Pattern: <div id="propertyprofile" class="user-content">...</div>
	startMarker := `<div id="propertyprofile" class="user-content">`
	startIdx := strings.Index(body, startMarker)
	if startIdx == -1 {
		return ""
	}

	// Move past the opening tag
	startIdx += len(startMarker)

	// Find the closing tag - look for the next major section marker
	// The description ends before <h4> sections or </read-more> or the closing </div>
	content := body[startIdx:]

	// Find where the main description ends (before additional sections like Annual Rainfall)
	endMarkers := []string{"<h4>", "</read-more>", "</div>"}
	endIdx := len(content)
	for _, marker := range endMarkers {
		if idx := strings.Index(content, marker); idx != -1 && idx < endIdx {
			endIdx = idx
		}
	}
	content = content[:endIdx]

	// Extract text from <p> tags and clean up HTML
	var descParts []string
	pPattern := regexp.MustCompile(`<p>([^<]+)`)
	pMatches := pPattern.FindAllStringSubmatch(content, -1)
	for _, match := range pMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				// Decode HTML entities
				text = strings.ReplaceAll(text, "&amp;", "&")
				text = strings.ReplaceAll(text, "&lt;", "<")
				text = strings.ReplaceAll(text, "&gt;", ">")
				text = strings.ReplaceAll(text, "&quot;", "\"")
				text = strings.ReplaceAll(text, "&#39;", "'")
				text = strings.ReplaceAll(text, "&nbsp;", " ")
				descParts = append(descParts, text)
			}
		}
	}

	return strings.Join(descParts, "\n\n")
}

func (s *FarmBuyScraper) fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
