package scraper

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"farm-search/internal/models"
)

// DomainScraper handles fetching listings from Domain.com.au via their official API
type DomainScraper struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

// NewDomainScraper creates a new Domain API scraper
func NewDomainScraper(apiKey string) *DomainScraper {
	return &DomainScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:  apiKey,
		baseURL: "https://api.domain.com.au",
	}
}

// DomainSearchRequest represents the request body for residential search
type DomainSearchRequest struct {
	ListingType          string           `json:"listingType"`
	PropertyTypes        []string         `json:"propertyTypes,omitempty"`
	MinBedrooms          *int             `json:"minBedrooms,omitempty"`
	MaxBedrooms          *int             `json:"maxBedrooms,omitempty"`
	MinBathrooms         *int             `json:"minBathrooms,omitempty"`
	MaxBathrooms         *int             `json:"maxBathrooms,omitempty"`
	MinCarspaces         *int             `json:"minCarspaces,omitempty"`
	MinPrice             *int             `json:"minPrice,omitempty"`
	MaxPrice             *int             `json:"maxPrice,omitempty"`
	MinLandArea          *int             `json:"minLandArea,omitempty"`
	MaxLandArea          *int             `json:"maxLandArea,omitempty"`
	Locations            []DomainLocation `json:"locations,omitempty"`
	ExcludePriceWithheld *bool            `json:"excludePriceWithheld,omitempty"`
	ExcludeUnderContract *bool            `json:"excludeUnderContract,omitempty"`
	PageNumber           int              `json:"pageNumber,omitempty"`
	PageSize             int              `json:"pageSize,omitempty"`
	Sort                 *DomainSort      `json:"sort,omitempty"`
	ListingAttributes    []string         `json:"listingAttributes,omitempty"`
}

// DomainLocation represents a location filter
type DomainLocation struct {
	State                     string `json:"state,omitempty"`
	Region                    string `json:"region,omitempty"`
	Area                      string `json:"area,omitempty"`
	Suburb                    string `json:"suburb,omitempty"`
	PostCode                  string `json:"postCode,omitempty"`
	IncludeSurroundingSuburbs bool   `json:"includeSurroundingSuburbs,omitempty"`
}

// DomainSort represents sort options
type DomainSort struct {
	SortKey   string `json:"sortKey,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// DomainSearchResult represents a single search result item
type DomainSearchResult struct {
	Type    string         `json:"type"` // "PropertyListing" or "Project"
	Listing *DomainListing `json:"listing,omitempty"`
	Project *DomainProject `json:"project,omitempty"`
}

// DomainListing represents a property listing from the API
type DomainListing struct {
	ListingType        string                 `json:"listingType"`
	ID                 int64                  `json:"id"`
	Advertiser         *DomainAdvertiser      `json:"advertiser,omitempty"`
	PriceDetails       *DomainPriceDetails    `json:"priceDetails,omitempty"`
	Media              []DomainMedia          `json:"media,omitempty"`
	PropertyDetails    *DomainPropertyDetails `json:"propertyDetails,omitempty"`
	Headline           string                 `json:"headline,omitempty"`
	SummaryDescription string                 `json:"summaryDescription,omitempty"`
	HasFloorplan       bool                   `json:"hasFloorplan,omitempty"`
	HasVideo           bool                   `json:"hasVideo,omitempty"`
	Labels             []string               `json:"labels,omitempty"`
	AuctionSchedule    *DomainAuction         `json:"auctionSchedule,omitempty"`
	DateListed         string                 `json:"dateListed,omitempty"`
	DateUpdated        string                 `json:"dateUpdated,omitempty"`
	ListingSlug        string                 `json:"listingSlug,omitempty"`
}

// DomainProject represents a development project listing
type DomainProject struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name,omitempty"`
	State         string          `json:"state,omitempty"`
	Media         []DomainMedia   `json:"media,omitempty"`
	ProjectSlug   string          `json:"projectSlug,omitempty"`
	ChildListings []DomainListing `json:"childListings,omitempty"`
}

// DomainAdvertiser represents the listing advertiser
type DomainAdvertiser struct {
	Type               string          `json:"type"`
	ID                 int             `json:"id"`
	Name               string          `json:"name,omitempty"`
	LogoURL            string          `json:"logoUrl,omitempty"`
	PreferredColourHex string          `json:"preferredColourHex,omitempty"`
	BannerURL          string          `json:"bannerUrl,omitempty"`
	Contacts           []DomainContact `json:"contacts,omitempty"`
}

// DomainContact represents an agent contact
type DomainContact struct {
	Name     string `json:"name,omitempty"`
	PhotoURL string `json:"photoUrl,omitempty"`
}

// DomainPriceDetails represents price information
type DomainPriceDetails struct {
	DisplayPrice string `json:"displayPrice,omitempty"`
	PriceFrom    int64  `json:"priceFrom,omitempty"`
	PriceTo      int64  `json:"priceTo,omitempty"`
}

// DomainMedia represents a media item (image/video)
type DomainMedia struct {
	Category string `json:"category"` // "Image", "Video", etc.
	URL      string `json:"url"`
}

// DomainPropertyDetails represents property details
type DomainPropertyDetails struct {
	State              string   `json:"state,omitempty"`
	Features           []string `json:"features,omitempty"`
	PropertyType       string   `json:"propertyType,omitempty"`
	AllPropertyTypes   []string `json:"allPropertyTypes,omitempty"`
	Bathrooms          *float64 `json:"bathrooms,omitempty"`
	Bedrooms           *float64 `json:"bedrooms,omitempty"`
	Carspaces          *int     `json:"carspaces,omitempty"`
	UnitNumber         string   `json:"unitNumber,omitempty"`
	StreetNumber       string   `json:"streetNumber,omitempty"`
	Street             string   `json:"street,omitempty"`
	Area               string   `json:"area,omitempty"`
	Region             string   `json:"region,omitempty"`
	Suburb             string   `json:"suburb,omitempty"`
	Postcode           string   `json:"postcode,omitempty"`
	DisplayableAddress string   `json:"displayableAddress,omitempty"`
	Latitude           *float64 `json:"latitude,omitempty"`
	Longitude          *float64 `json:"longitude,omitempty"`
	LandArea           *float64 `json:"landArea,omitempty"`     // in sqm
	BuildingArea       *float64 `json:"buildingArea,omitempty"` // in sqm
}

// DomainAuction represents auction information
type DomainAuction struct {
	Time            string `json:"time,omitempty"`
	AuctionLocation string `json:"auctionLocation,omitempty"`
}

// ScrapeListings fetches property listings from Domain API
func (s *DomainScraper) ScrapeListings(ctx context.Context, state string, maxPages int) ([]models.Property, error) {
	return s.ScrapeListingsWithExistsCheck(ctx, state, maxPages, nil)
}

// ScrapeListingsWithExistsCheck fetches property listings with optional duplicate detection
func (s *DomainScraper) ScrapeListingsWithExistsCheck(ctx context.Context, state string, maxPages int, existsChecker ExistsChecker) ([]models.Property, error) {
	var allListings []models.Property
	pageSize := 100 // Max allowed by API

	// Build search request for rural/farm properties in the specified state
	// The API limits results to 1000 total, so we use filters to target our desired properties
	searchReq := DomainSearchRequest{
		ListingType: "Sale",
		PropertyTypes: []string{
			"AcreageSemiRural",
			"Farm",
			"Rural",
		},
		Locations: []DomainLocation{
			{
				State: strings.ToUpper(state),
			},
		},
		MinLandArea: intPtr(40000),   // 4 hectares minimum (40,000 sqm)
		MaxPrice:    intPtr(2000000), // Max $2M to match other scrapers
		PageSize:    pageSize,
		Sort: &DomainSort{
			SortKey:   "DateListed",
			Direction: "Descending",
		},
	}

	for page := 1; maxPages <= 0 || page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		searchReq.PageNumber = page

		log.Printf("Fetching Domain API page %d for %s...", page, state)

		results, totalCount, err := s.searchListings(ctx, searchReq)
		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			break
		}

		// Convert API results to our Property model
		var pageListings []models.Property
		for _, result := range results {
			if result.Type == "PropertyListing" && result.Listing != nil {
				if listing := s.convertListing(result.Listing); listing != nil {
					pageListings = append(pageListings, *listing)
				}
			} else if result.Type == "Project" && result.Project != nil {
				// Handle project listings (multiple apartments in one project)
				for _, childListing := range result.Project.ChildListings {
					if listing := s.convertListing(&childListing); listing != nil {
						pageListings = append(pageListings, *listing)
					}
				}
			}
		}

		// Check for duplicates if checker is provided
		if existsChecker != nil && len(pageListings) > 0 {
			externalIDs := make([]string, len(pageListings))
			for i, l := range pageListings {
				externalIDs[i] = l.ExternalID
			}

			existsMap, err := existsChecker(externalIDs)
			if err != nil {
				log.Printf("Warning: failed to check existing properties: %v", err)
			} else {
				newCount := 0
				for _, l := range pageListings {
					if !existsMap[l.ExternalID] {
						newCount++
					}
				}

				log.Printf("Page %d: %d listings, %d new, %d already scraped", page, len(pageListings), newCount, len(pageListings)-newCount)

				// If no new properties, stop pagination (results sorted by newest first)
				if newCount == 0 {
					log.Printf("No new properties found on page %d, stopping pagination", page)
					break
				}
			}
		}

		allListings = append(allListings, pageListings...)
		log.Printf("Page %d: found %d listings (total: %d, API reports %d total)", page, len(pageListings), len(allListings), totalCount)

		// Check if we've fetched all available results
		if len(results) < pageSize || len(allListings) >= totalCount {
			log.Printf("Reached end of results (total available: %d)", totalCount)
			break
		}

		// Rate limiting between pages
		time.Sleep(500 * time.Millisecond)
	}

	return allListings, nil
}

// searchListings makes a search request to the Domain API
func (s *DomainScraper) searchListings(ctx context.Context, req DomainSearchRequest) ([]DomainSearchResult, int, error) {
	url := s.baseURL + "/v1/listings/residential/_search"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", s.apiKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Get total count from header
	totalCount := 0
	if tc := resp.Header.Get("X-Total-Count"); tc != "" {
		totalCount, _ = strconv.Atoi(tc)
	}

	var results []DomainSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return results, totalCount, nil
}

// convertListing converts a Domain API listing to our Property model
func (s *DomainScraper) convertListing(listing *DomainListing) *models.Property {
	if listing == nil {
		return nil
	}

	now := time.Now()
	prop := &models.Property{
		ExternalID: strconv.FormatInt(listing.ID, 10),
		Source:     "domain",
		State:      "NSW", // Default, will be overridden if available
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Build URL from listing slug
	if listing.ListingSlug != "" {
		prop.URL = fmt.Sprintf("https://www.domain.com.au/%s", listing.ListingSlug)
	} else {
		prop.URL = fmt.Sprintf("https://www.domain.com.au/listing/%d", listing.ID)
	}

	// Extract property details
	if details := listing.PropertyDetails; details != nil {
		if details.DisplayableAddress != "" {
			prop.Address = sql.NullString{String: details.DisplayableAddress, Valid: true}
		} else {
			// Build address from components
			addr := ""
			if details.StreetNumber != "" {
				addr = details.StreetNumber
			}
			if details.Street != "" {
				if addr != "" {
					addr += " "
				}
				addr += details.Street
			}
			if addr != "" {
				prop.Address = sql.NullString{String: addr, Valid: true}
			}
		}

		if details.Suburb != "" {
			prop.Suburb = sql.NullString{String: details.Suburb, Valid: true}
		}
		if details.State != "" {
			prop.State = details.State
		}
		if details.Postcode != "" {
			prop.Postcode = sql.NullString{String: details.Postcode, Valid: true}
		}
		if details.Latitude != nil && details.Longitude != nil {
			prop.Latitude = sql.NullFloat64{Float64: *details.Latitude, Valid: true}
			prop.Longitude = sql.NullFloat64{Float64: *details.Longitude, Valid: true}
		}
		if details.PropertyType != "" {
			prop.PropertyType = sql.NullString{String: details.PropertyType, Valid: true}
		}
		if details.Bedrooms != nil {
			prop.Bedrooms = sql.NullInt64{Int64: int64(*details.Bedrooms), Valid: true}
		}
		if details.Bathrooms != nil {
			prop.Bathrooms = sql.NullInt64{Int64: int64(*details.Bathrooms), Valid: true}
		}
		if details.LandArea != nil && *details.LandArea > 0 {
			prop.LandSizeSqm = sql.NullFloat64{Float64: *details.LandArea, Valid: true}
		}
	}

	// Extract price
	if price := listing.PriceDetails; price != nil {
		if price.DisplayPrice != "" {
			prop.PriceText = sql.NullString{String: price.DisplayPrice, Valid: true}
		}
		if price.PriceFrom > 0 {
			prop.PriceMin = sql.NullInt64{Int64: price.PriceFrom, Valid: true}
		}
		if price.PriceTo > 0 {
			prop.PriceMax = sql.NullInt64{Int64: price.PriceTo, Valid: true}
		}
	}

	// Extract description from headline and summary
	if listing.Headline != "" {
		prop.Description = sql.NullString{String: listing.Headline, Valid: true}
	}
	if listing.SummaryDescription != "" {
		// Clean HTML tags from description
		desc := strings.ReplaceAll(listing.SummaryDescription, "<br />", "\n")
		desc = strings.ReplaceAll(desc, "<br>", "\n")
		desc = strings.ReplaceAll(desc, "<b>", "")
		desc = strings.ReplaceAll(desc, "</b>", "")
		desc = strings.TrimSpace(desc)
		if prop.Description.Valid {
			prop.Description.String += "\n\n" + desc
		} else {
			prop.Description = sql.NullString{String: desc, Valid: true}
		}
	}

	// Extract images
	var images []string
	for _, media := range listing.Media {
		if media.Category == "Image" && media.URL != "" {
			images = append(images, media.URL)
		}
	}
	if len(images) > 0 {
		imgJSON, _ := json.Marshal(images)
		prop.Images = sql.NullString{String: string(imgJSON), Valid: true}
	}

	// Extract listing date
	if listing.DateListed != "" {
		// Parse date - format varies, try common formats
		formats := []string{
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, listing.DateListed); err == nil {
				prop.ListedAt = sql.NullTime{Time: t, Valid: true}
				break
			}
		}
	}

	return prop
}

// FetchListingDetails fetches full details for a single listing by ID
func (s *DomainScraper) FetchListingDetails(ctx context.Context, listingID int64) (*models.Property, error) {
	url := fmt.Sprintf("%s/v1/listings/%d", s.baseURL, listingID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", s.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("listing not found")
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var listing DomainListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return s.convertListing(&listing), nil
}

// intPtr is a helper to create a pointer to an int
func intPtr(i int) *int {
	return &i
}
