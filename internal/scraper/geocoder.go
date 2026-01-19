package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Geocoder handles address geocoding using Nominatim
type Geocoder struct {
	client    *http.Client
	userAgent string
	baseURL   string
}

// NominatimResult represents a geocoding result from Nominatim
type NominatimResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	Importance  float64 `json:"importance"`
}

// NewGeocoder creates a new Nominatim geocoder
func NewGeocoder() *Geocoder {
	return &Geocoder{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		userAgent: "FarmSearch/1.0 (property search application)",
		baseURL:   "https://nominatim.openstreetmap.org",
	}
}

// Geocode converts an address to coordinates
func (g *Geocoder) Geocode(ctx context.Context, address string) (lat, lng float64, err error) {
	// Build the request URL
	params := url.Values{}
	params.Set("q", address)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("countrycodes", "au")

	reqURL := fmt.Sprintf("%s/search?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Nominatim requires a valid User-Agent
	req.Header.Set("User-Agent", g.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read response: %w", err)
	}

	var results []NominatimResult
	if err := json.Unmarshal(body, &results); err != nil {
		return 0, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no results found for address: %s", address)
	}

	// Parse coordinates
	result := results[0]
	if _, err := fmt.Sscanf(result.Lat, "%f", &lat); err != nil {
		return 0, 0, fmt.Errorf("failed to parse latitude: %w", err)
	}
	if _, err := fmt.Sscanf(result.Lon, "%f", &lng); err != nil {
		return 0, 0, fmt.Errorf("failed to parse longitude: %w", err)
	}

	return lat, lng, nil
}

// ReverseGeocode converts coordinates to an address
func (g *Geocoder) ReverseGeocode(ctx context.Context, lat, lng float64) (string, error) {
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))
	params.Set("format", "json")

	reqURL := fmt.Sprintf("%s/reverse?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", g.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result NominatimResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.DisplayName, nil
}
