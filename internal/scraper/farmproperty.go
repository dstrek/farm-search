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

// FarmPropertyScraper handles scraping from farmproperty.com.au
type FarmPropertyScraper struct {
	client    *http.Client
	userAgent string
	baseURL   string
}

// NewFarmPropertyScraper creates a new FarmProperty scraper
func NewFarmPropertyScraper() *FarmPropertyScraper {
	return &FarmPropertyScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		baseURL:   "https://www.farmproperty.com.au",
	}
}

// ScrapeListings scrapes property listings from FarmProperty
func (s *FarmPropertyScraper) ScrapeListings(ctx context.Context, state string, maxPages int) ([]models.Property, error) {
	var allListings []models.Property

	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		log.Printf("Scraping FarmProperty page %d for %s...", page, state)

		listings, hasMore, err := s.scrapePage(ctx, state, page)
		if err != nil {
			log.Printf("Error scraping page %d: %v", page, err)
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

func (s *FarmPropertyScraper) scrapePage(ctx context.Context, state string, page int) ([]models.Property, bool, error) {
	// Build the search URL
	searchURL := fmt.Sprintf("%s/buy/%s", s.baseURL, strings.ToLower(state))
	if page > 1 {
		searchURL += fmt.Sprintf("?pagenumber=%d", page)
	}

	body, err := s.fetch(ctx, searchURL)
	if err != nil {
		return nil, false, err
	}

	// Find property links
	linkPattern := regexp.MustCompile(`href="/property/(\d+)-([^"]+)"`)
	matches := linkPattern.FindAllStringSubmatch(body, -1)

	seenIDs := make(map[string]bool)
	var listings []models.Property

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		listingID := match[1]
		if seenIDs[listingID] {
			continue
		}
		seenIDs[listingID] = true

		// Fetch full listing details
		listingURL := fmt.Sprintf("%s/property/%s-%s", s.baseURL, listingID, match[2])
		listing, err := s.FetchListingDetails(ctx, listingURL, listingID)
		if err != nil {
			log.Printf("Error fetching listing %s: %v", listingID, err)
			continue
		}

		if listing != nil {
			listings = append(listings, *listing)
		}

		// Rate limiting between detail fetches
		time.Sleep(500 * time.Millisecond)
	}

	// Check if there are more pages
	hasMore := strings.Contains(body, `rel="next"`) ||
		strings.Contains(body, fmt.Sprintf("pagenumber=%d", page+1))

	return listings, hasMore, nil
}

// FetchListingDetails fetches full details for a single listing
func (s *FarmPropertyScraper) FetchListingDetails(ctx context.Context, listingURL, listingID string) (*models.Property, error) {
	body, err := s.fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	return s.parseListingDetails(body, listingURL, listingID)
}

func (s *FarmPropertyScraper) parseListingDetails(html, listingURL, listingID string) (*models.Property, error) {
	now := time.Now()
	listing := &models.Property{
		ExternalID: listingID,
		Source:     "farmproperty",
		URL:        listingURL,
		State:      "NSW",
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Extract JSON-LD structured data
	// Note: FarmProperty uses HTML-encoded type: application/ld&#x2B;json
	jsonLDPattern := regexp.MustCompile(`<script type="application/ld(?:\+|&#x2B;)json">\s*(\[[\s\S]*?\])\s*</script>`)
	matches := jsonLDPattern.FindStringSubmatch(html)

	if len(matches) >= 2 {
		// Unescape HTML entities
		jsonStr := strings.ReplaceAll(matches[1], "&#x2B;", "+")
		jsonStr = strings.ReplaceAll(jsonStr, "&amp;", "&")

		var jsonLD []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &jsonLD); err == nil {
			for _, item := range jsonLD {
				schemaType, _ := item["@type"].(string)
				if schemaType == "Residence" {
					// Extract name/title
					if name, ok := item["name"].(string); ok {
						listing.Address = sql.NullString{String: name, Valid: true}
					}

					// Extract description
					if desc, ok := item["description"].(string); ok {
						listing.Description = sql.NullString{String: desc, Valid: true}
					}

					// Extract image
					if img, ok := item["image"].(string); ok {
						images := []string{img}
						imgJSON, _ := json.Marshal(images)
						listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
					}

					// Extract address
					if addr, ok := item["address"].(map[string]interface{}); ok {
						if locality, ok := addr["addressLocality"].(string); ok {
							listing.Suburb = sql.NullString{String: locality, Valid: true}
						}
						if postcode, ok := addr["postalCode"].(string); ok {
							listing.Postcode = sql.NullString{String: postcode, Valid: true}
						}
						if street, ok := addr["streetAddress"].(string); ok {
							if !listing.Address.Valid {
								listing.Address = sql.NullString{String: street, Valid: true}
							}
						}
					}

					// Extract coordinates
					if geo, ok := item["geo"].(map[string]interface{}); ok {
						if lat, ok := geo["latitude"].(string); ok {
							if latF, err := strconv.ParseFloat(lat, 64); err == nil {
								listing.Latitude = sql.NullFloat64{Float64: latF, Valid: true}
							}
						}
						if lng, ok := geo["longitude"].(string); ok {
							if lngF, err := strconv.ParseFloat(lng, 64); err == nil {
								listing.Longitude = sql.NullFloat64{Float64: lngF, Valid: true}
							}
						}
					}
				}
			}
		}
	}

	// Extract price from HTML
	pricePattern := regexp.MustCompile(`<span[^>]*class="[^"]*price[^"]*"[^>]*>([^<]+)</span>`)
	if matches := pricePattern.FindStringSubmatch(html); len(matches) > 1 {
		listing.PriceText = sql.NullString{String: strings.TrimSpace(matches[1]), Valid: true}
	}

	// Try alternative price patterns
	if !listing.PriceText.Valid {
		pricePattern2 := regexp.MustCompile(`>\s*\$[\d,]+(?:\s*-\s*\$[\d,]+)?\s*<`)
		if matches := pricePattern2.FindStringSubmatch(html); len(matches) > 0 {
			price := strings.Trim(matches[0], "<>")
			listing.PriceText = sql.NullString{String: strings.TrimSpace(price), Valid: true}
		}
	}

	// Extract land size
	landPattern := regexp.MustCompile(`([\d,.]+)\s*(hectares?|ha|acres?|ac)\b`)
	if matches := landPattern.FindStringSubmatch(strings.ToLower(html)); len(matches) >= 3 {
		sizeStr := strings.ReplaceAll(matches[1], ",", "")
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			unit := matches[2]
			var sqm float64
			if strings.HasPrefix(unit, "hectare") || unit == "ha" {
				sqm = size * 10000
			} else if strings.HasPrefix(unit, "acre") || unit == "ac" {
				sqm = size * 4046.86
			}
			if sqm > 0 {
				listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
			}
		}
	}

	// Extract property type from page content
	listing.PropertyType = sql.NullString{String: "rural", Valid: true}

	// Determine state from postcode or URL
	if listing.Postcode.Valid {
		listing.State = stateFromPostcode(listing.Postcode.String)
	}

	return listing, nil
}

func (s *FarmPropertyScraper) fetch(ctx context.Context, url string) (string, error) {
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

// stateFromPostcode determines the Australian state from a postcode
func stateFromPostcode(postcode string) string {
	if len(postcode) != 4 {
		return "NSW"
	}

	first := postcode[0]
	switch first {
	case '1', '2':
		return "NSW"
	case '3':
		return "VIC"
	case '4':
		return "QLD"
	case '5':
		return "SA"
	case '6':
		return "WA"
	case '7':
		return "TAS"
	case '0':
		return "NT"
	default:
		return "NSW"
	}
}
