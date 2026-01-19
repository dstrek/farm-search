package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IsochroneGenerator generates driving time isochrones using Valhalla
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

// NewIsochroneGenerator creates a new isochrone generator using Valhalla
// Pass empty string to use the public Valhalla server (limited to 90 min)
// Pass a URL like "http://localhost:8002" for a local instance (no limit)
func NewIsochroneGenerator(baseURL string) *IsochroneGenerator {
	if baseURL == "" {
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
	// Apply 10% buffer to make isochrones more conservative
	// (accounts for traffic, stops, slower rural roads)
	adjustedMinutes := int(float64(minutes) * 0.9)

	url := fmt.Sprintf(
		"%s/isochrone?json={\"locations\":[{\"lat\":%f,\"lon\":%f}],\"costing\":\"auto\",\"contours\":[{\"time\":%d}],\"polygons\":true}",
		g.baseURL, lat, lng, adjustedMinutes,
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

// Sutherland, NSW coordinates (origin point for drive time calculations)
var Sutherland = struct {
	Lat float64
	Lng float64
}{
	Lat: -34.0309,
	Lng: 151.0579,
}

// GenerateSutherlandIsochrones generates isochrones for Sutherland at specified intervals
func (g *IsochroneGenerator) GenerateSutherlandIsochrones(ctx context.Context, intervals []int) (map[int]*GeoJSONFeatureCollection, error) {
	results := make(map[int]*GeoJSONFeatureCollection)

	for _, mins := range intervals {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		iso, err := g.GenerateIsochrone(ctx, Sutherland.Lat, Sutherland.Lng, mins)
		if err != nil {
			return results, fmt.Errorf("failed to generate %d min isochrone: %w", mins, err)
		}

		results[mins] = iso

		// Rate limiting
		time.Sleep(1 * time.Second)
	}

	return results, nil
}
