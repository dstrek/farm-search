package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Router calculates driving routes and times using Valhalla
type Router struct {
	client  *http.Client
	baseURL string
}

// RouteResult contains the result of a route calculation
type RouteResult struct {
	DurationMins float64 // Drive time in minutes
	DistanceKm   float64 // Distance in kilometers
}

// NewRouter creates a new router using Valhalla
// Pass empty string to use the public Valhalla server
// Pass a URL like "http://localhost:8002" for a local instance
func NewRouter(baseURL string) *Router {
	if baseURL == "" {
		baseURL = "https://valhalla1.openstreetmap.de"
	}
	return &Router{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// valhallaRouteResponse represents the Valhalla route API response
type valhallaRouteResponse struct {
	Trip struct {
		Summary struct {
			Time   float64 `json:"time"`   // Duration in seconds
			Length float64 `json:"length"` // Distance in kilometers
		} `json:"summary"`
		Legs []struct {
			Shape string `json:"shape"` // Encoded polyline
		} `json:"legs"`
	} `json:"trip"`
}

// RouteWithShape contains route result with coordinates for drawing
type RouteWithShape struct {
	DurationMins float64     `json:"duration_mins"`
	DistanceKm   float64     `json:"distance_km"`
	Coordinates  [][]float64 `json:"coordinates"` // [[lng, lat], ...]
}

// GetDriveTime calculates the drive time from a property to Sutherland
func (r *Router) GetDriveTime(ctx context.Context, fromLat, fromLng float64) (*RouteResult, error) {
	return r.GetRoute(ctx, fromLat, fromLng, Sutherland.Lat, Sutherland.Lng)
}

// GetRoute calculates the drive time between two points
func (r *Router) GetRoute(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (*RouteResult, error) {
	// Build compact request JSON (no whitespace - required for URL encoding)
	requestJSON := fmt.Sprintf(`{"locations":[{"lat":%f,"lon":%f},{"lat":%f,"lon":%f}],"costing":"auto","units":"kilometers"}`,
		fromLat, fromLng, toLat, toLng)

	url := fmt.Sprintf("%s/route?json=%s", r.baseURL, requestJSON)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "FarmSearch/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("route request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("route API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result valhallaRouteResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse route response: %w", err)
	}

	// Apply 10% buffer to account for traffic, stops, and real-world conditions
	durationMins := (result.Trip.Summary.Time / 60.0) * 1.1

	return &RouteResult{
		DurationMins: durationMins,
		DistanceKm:   result.Trip.Summary.Length,
	}, nil
}

// GetRouteWithShape calculates the driving route with the route shape for drawing
func (r *Router) GetRouteWithShape(ctx context.Context, fromLat, fromLng, toLat, toLng float64) (*RouteWithShape, error) {
	// Build request JSON with shape format
	requestJSON := fmt.Sprintf(`{"locations":[{"lat":%f,"lon":%f},{"lat":%f,"lon":%f}],"costing":"auto","units":"kilometers","shape_format":"polyline6"}`,
		fromLat, fromLng, toLat, toLng)

	url := fmt.Sprintf("%s/route?json=%s", r.baseURL, requestJSON)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "FarmSearch/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("route request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("route API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result valhallaRouteResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse route response: %w", err)
	}

	// Apply 10% buffer to account for traffic
	durationMins := (result.Trip.Summary.Time / 60.0) * 1.1

	// Decode the polyline shape
	var coordinates [][]float64
	if len(result.Trip.Legs) > 0 && result.Trip.Legs[0].Shape != "" {
		coordinates = decodePolyline6(result.Trip.Legs[0].Shape)
	}

	return &RouteWithShape{
		DurationMins: durationMins,
		DistanceKm:   result.Trip.Summary.Length,
		Coordinates:  coordinates,
	}, nil
}

// decodePolyline6 decodes a polyline encoded with precision 6 (Valhalla default)
// Returns coordinates as [[lng, lat], ...] for GeoJSON
func decodePolyline6(encoded string) [][]float64 {
	var coordinates [][]float64
	precision := 1e6

	index := 0
	lat := 0.0
	lng := 0.0

	for index < len(encoded) {
		// Decode latitude
		shift := 0
		result := 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lat += float64(^(result >> 1))
		} else {
			lat += float64(result >> 1)
		}

		// Decode longitude
		shift = 0
		result = 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lng += float64(^(result >> 1))
		} else {
			lng += float64(result >> 1)
		}

		// GeoJSON uses [lng, lat] order
		coordinates = append(coordinates, []float64{lng / precision, lat / precision})
	}

	return coordinates
}
