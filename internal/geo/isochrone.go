package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IsochroneGenerator generates driving time isochrones using OSRM
type IsochroneGenerator struct {
	client  *http.Client
	baseURL string
}

// GeoJSON types for isochrone data
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// NewIsochroneGenerator creates a new isochrone generator
// Uses the public OSRM demo server or a self-hosted instance
func NewIsochroneGenerator(baseURL string) *IsochroneGenerator {
	if baseURL == "" {
		// Use Valhalla public API for isochrones (OSRM doesn't have native isochrone support)
		// Alternative: use openrouteservice.org
		baseURL = "https://valhalla1.openstreetmap.de"
	}
	return &IsochroneGenerator{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: baseURL,
	}
}

// GenerateIsochrone generates a driving time isochrone polygon
// lat, lng: center point coordinates
// minutes: driving time in minutes
func (g *IsochroneGenerator) GenerateIsochrone(ctx context.Context, lat, lng float64, minutes int) (*GeoJSONFeatureCollection, error) {
	// Valhalla isochrone API format
	url := fmt.Sprintf(
		"%s/isochrone?json={\"locations\":[{\"lat\":%f,\"lon\":%f}],\"costing\":\"auto\",\"contours\":[{\"time\":%d}],\"polygons\":true}",
		g.baseURL, lat, lng, minutes,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "FarmSearch/1.0")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("isochrone request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("isochrone API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result GeoJSONFeatureCollection
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse isochrone response: %w", err)
	}

	// Add properties
	for i := range result.Features {
		if result.Features[i].Properties == nil {
			result.Features[i].Properties = make(map[string]interface{})
		}
		result.Features[i].Properties["minutes"] = minutes
	}

	return &result, nil
}

// Sydney CBD coordinates
var SydneyCBD = struct {
	Lat float64
	Lng float64
}{
	Lat: -33.8688,
	Lng: 151.2093,
}

// GenerateSydneyIsochrones generates isochrones for Sydney at specified intervals
func (g *IsochroneGenerator) GenerateSydneyIsochrones(ctx context.Context, intervals []int) (map[int]*GeoJSONFeatureCollection, error) {
	results := make(map[int]*GeoJSONFeatureCollection)

	for _, mins := range intervals {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		iso, err := g.GenerateIsochrone(ctx, SydneyCBD.Lat, SydneyCBD.Lng, mins)
		if err != nil {
			return results, fmt.Errorf("failed to generate %d min isochrone: %w", mins, err)
		}

		results[mins] = iso

		// Rate limiting
		time.Sleep(1 * time.Second)
	}

	return results, nil
}
