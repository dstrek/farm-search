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
	} `json:"trip"`
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
