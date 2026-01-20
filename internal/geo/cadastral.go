package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// NSW Spatial Services ArcGIS REST API
	nswSpatialBaseURL = "https://portal.spatial.nsw.gov.au/server/rest/services/NSW_Land_Parcel_Property_Theme/MapServer/8/query"
)

// CadastralClient fetches cadastral lot data from NSW Spatial Services
type CadastralClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewCadastralClient creates a new cadastral API client
func NewCadastralClient() *CadastralClient {
	return &CadastralClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    nswSpatialBaseURL,
	}
}

// LotFeature represents a cadastral lot from the API response
type LotFeature struct {
	LotIDString string       `json:"lot_id_string"`
	LotNumber   string       `json:"lot_number"`
	PlanLabel   string       `json:"plan_label"`
	AreaSqm     float64      `json:"area_sqm"`
	Geometry    *LotGeometry `json:"geometry"`
}

// LotGeometry represents a GeoJSON geometry for cadastral lots
type LotGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// cadastralFeature is used for parsing the NSW Spatial API response
type cadastralFeature struct {
	Type       string                 `json:"type"`
	Geometry   *LotGeometry           `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

// cadastralFeatureCollection is used for parsing the NSW Spatial API response
type cadastralFeatureCollection struct {
	Type     string             `json:"type"`
	Features []cadastralFeature `json:"features"`
}

// FetchLotsInBounds fetches cadastral lots within the given bounding box
// Returns lots as GeoJSON features
func (c *CadastralClient) FetchLotsInBounds(ctx context.Context, minLng, minLat, maxLng, maxLat float64) ([]LotFeature, error) {
	// Build query parameters
	params := url.Values{}
	params.Set("where", "1=1")
	params.Set("outFields", "lotnumber,planlabel,lotidstring,shape_Area")
	params.Set("geometry", fmt.Sprintf("%f,%f,%f,%f", minLng, minLat, maxLng, maxLat))
	params.Set("geometryType", "esriGeometryEnvelope")
	params.Set("inSR", "4326")
	params.Set("outSR", "4326")
	params.Set("f", "geojson")
	params.Set("resultRecordCount", "2000") // Max allowed by API

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching lots: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var fc cadastralFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	lots := make([]LotFeature, 0, len(fc.Features))
	for _, f := range fc.Features {
		lot := LotFeature{
			Geometry: f.Geometry,
		}

		// Extract properties
		if v, ok := f.Properties["lotidstring"].(string); ok {
			lot.LotIDString = v
		}
		if v, ok := f.Properties["lotnumber"].(string); ok {
			lot.LotNumber = v
		}
		if v, ok := f.Properties["planlabel"].(string); ok {
			lot.PlanLabel = v
		}
		if v, ok := f.Properties["shape_Area"].(float64); ok {
			lot.AreaSqm = v
		}

		// Skip lots without valid identifiers
		if lot.LotIDString == "" {
			continue
		}

		lots = append(lots, lot)
	}

	return lots, nil
}

// FetchLotsAtPoint fetches cadastral lots that contain or are near the given point.
// First tries an exact point intersection, then falls back to a small bounding box
// search (~200m radius) to handle cases where property coordinates are approximate
// (e.g., geocoded to a road rather than the property itself).
func (c *CadastralClient) FetchLotsAtPoint(ctx context.Context, lng, lat float64) ([]LotFeature, error) {
	// First try exact point intersection
	lots, err := c.fetchLotsWithGeometry(ctx, fmt.Sprintf("%f,%f", lng, lat), "esriGeometryPoint", 10)
	if err != nil {
		return nil, err
	}

	// If point query found lots, return them
	if len(lots) > 0 {
		return lots, nil
	}

	// Fall back to bounding box search (~500m radius)
	// At Australian latitudes, 0.005 degrees ≈ 500m
	buffer := 0.005
	minLng := lng - buffer
	maxLng := lng + buffer
	minLat := lat - buffer
	maxLat := lat + buffer

	lots, err = c.fetchLotsWithGeometry(ctx,
		fmt.Sprintf("%f,%f,%f,%f", minLng, minLat, maxLng, maxLat),
		"esriGeometryEnvelope", 20)
	if err != nil {
		return nil, err
	}

	// Filter to lots whose centroid is within ~200m of the search point
	if len(lots) > 0 {
		filtered := make([]LotFeature, 0)
		for _, lot := range lots {
			if lot.Geometry == nil {
				continue
			}
			centLat, centLng, err := CalculateLotCentroid(lot.Geometry)
			if err != nil {
				continue
			}
			// Simple distance check (not exact but good enough for filtering)
			if abs(centLat-lat) < buffer && abs(centLng-lng) < buffer {
				filtered = append(filtered, lot)
			}
		}
		return filtered, nil
	}

	return lots, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// fetchLotsWithGeometry performs the actual API query with the given geometry
func (c *CadastralClient) fetchLotsWithGeometry(ctx context.Context, geometry, geometryType string, maxResults int) ([]LotFeature, error) {
	params := url.Values{}
	params.Set("where", "1=1")
	params.Set("outFields", "lotnumber,planlabel,lotidstring,shape_Area")
	params.Set("geometry", geometry)
	params.Set("geometryType", geometryType)
	params.Set("inSR", "4326")
	params.Set("outSR", "4326")
	params.Set("spatialRel", "esriSpatialRelIntersects")
	params.Set("f", "geojson")
	params.Set("resultRecordCount", fmt.Sprintf("%d", maxResults))

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching lots: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var fc cadastralFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	lots := make([]LotFeature, 0, len(fc.Features))
	for _, f := range fc.Features {
		lot := LotFeature{
			Geometry: f.Geometry,
		}

		// Extract properties
		if v, ok := f.Properties["lotidstring"].(string); ok {
			lot.LotIDString = v
		}
		if v, ok := f.Properties["lotnumber"].(string); ok {
			lot.LotNumber = v
		}
		if v, ok := f.Properties["planlabel"].(string); ok {
			lot.PlanLabel = v
		}
		if v, ok := f.Properties["shape_Area"].(float64); ok {
			lot.AreaSqm = v
		}

		// Skip lots without valid identifiers
		if lot.LotIDString == "" {
			continue
		}

		lots = append(lots, lot)
	}

	return lots, nil
}

// CalculateLotCentroid calculates the centroid of a lot polygon or multipolygon geometry
func CalculateLotCentroid(geom *LotGeometry) (lat, lng float64, err error) {
	if geom == nil {
		return 0, 0, fmt.Errorf("nil geometry")
	}

	var sumLng, sumLat float64
	var count int

	switch geom.Type {
	case "Polygon":
		// Polygon has [[[lng, lat], ...]] structure
		var coords [][][]float64
		if err := json.Unmarshal(geom.Coordinates, &coords); err != nil {
			return 0, 0, fmt.Errorf("parsing polygon coordinates: %w", err)
		}
		if len(coords) == 0 || len(coords[0]) == 0 {
			return 0, 0, fmt.Errorf("empty polygon")
		}
		ring := coords[0]
		for _, pt := range ring {
			if len(pt) >= 2 {
				sumLng += pt[0]
				sumLat += pt[1]
				count++
			}
		}

	case "MultiPolygon":
		// MultiPolygon has [[[[lng, lat], ...]]] structure
		var coords [][][][]float64
		if err := json.Unmarshal(geom.Coordinates, &coords); err != nil {
			return 0, 0, fmt.Errorf("parsing multipolygon coordinates: %w", err)
		}
		if len(coords) == 0 {
			return 0, 0, fmt.Errorf("empty multipolygon")
		}
		// Use first polygon's exterior ring for centroid
		if len(coords[0]) > 0 {
			ring := coords[0][0]
			for _, pt := range ring {
				if len(pt) >= 2 {
					sumLng += pt[0]
					sumLat += pt[1]
					count++
				}
			}
		}

	default:
		return 0, 0, fmt.Errorf("unsupported geometry type: %s", geom.Type)
	}

	if count == 0 {
		return 0, 0, fmt.Errorf("no valid coordinates found")
	}

	return sumLat / float64(count), sumLng / float64(count), nil
}

// LotGeometryToJSON converts a LotGeometry to its JSON string representation
func LotGeometryToJSON(geom *LotGeometry) (string, error) {
	if geom == nil {
		return "", fmt.Errorf("nil geometry")
	}
	data, err := json.Marshal(geom)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
