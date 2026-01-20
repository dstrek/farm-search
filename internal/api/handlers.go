package api

import (
	"context"
	"encoding/json"
	"farm-search/internal/db"
	"farm-search/internal/geo"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handlers contains HTTP handlers and their dependencies
type Handlers struct {
	db *db.DB
}

// NewHandlers creates a new Handlers instance
func NewHandlers(database *db.DB) *Handlers {
	return &Handlers{db: database}
}

// parsePropertyFilter extracts filter parameters from query string
func parsePropertyFilter(q map[string][]string) db.PropertyFilter {
	filter := db.PropertyFilter{}

	get := func(key string) string {
		if vals, ok := q[key]; ok && len(vals) > 0 {
			return vals[0]
		}
		return ""
	}

	// Parse price filters
	if v := get("price_min"); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PriceMin = &val
		}
	}
	if v := get("price_max"); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PriceMax = &val
		}
	}

	// Parse property types
	if v := get("type"); v != "" {
		filter.PropertyTypes = strings.Split(v, ",")
	}

	// Parse land size filters
	if v := get("land_size_min"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.LandSizeMin = &val
		}
	}
	if v := get("land_size_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.LandSizeMax = &val
		}
	}

	// Parse distance filters
	if v := get("distance_sydney_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceSydneyMax = &val
		}
	}
	if v := get("distance_town_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceTownMax = &val
		}
	}
	if v := get("distance_school_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceSchoolMax = &val
		}
	}

	// Parse drive time filters
	if v := get("drive_time_sydney_max"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			filter.DriveTimeSydneyMax = &val
		}
	}
	if v := get("drive_time_town_max"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			filter.DriveTimeTownMax = &val
		}
	}

	// Parse map bounds (sw_lat,sw_lng,ne_lat,ne_lng)
	if v := get("bounds"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) == 4 {
			swLat, _ := strconv.ParseFloat(parts[0], 64)
			swLng, _ := strconv.ParseFloat(parts[1], 64)
			neLat, _ := strconv.ParseFloat(parts[2], 64)
			neLng, _ := strconv.ParseFloat(parts[3], 64)
			filter.SWLat = &swLat
			filter.SWLng = &swLng
			filter.NELat = &neLat
			filter.NELng = &neLng
		}
	}

	// Parse pagination
	if v := get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 && val <= 500 {
			filter.Limit = val
		}
	}
	if v := get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 0 {
			filter.Offset = val
		}
	}

	return filter
}

// ListProperties handles GET /api/properties
func (h *Handlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	filter := parsePropertyFilter(r.URL.Query())

	properties, err := h.db.ListProperties(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"properties": properties,
		"count":      len(properties),
	})
}

// GetProperty handles GET /api/properties/{id}
func (h *Handlers) GetProperty(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid property ID", http.StatusBadRequest)
		return
	}

	property, err := h.db.GetProperty(id)
	if err != nil {
		http.Error(w, "property not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(property)
}

// GetFilterOptions handles GET /api/filters/options
func (h *Handlers) GetFilterOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.db.GetFilterOptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// TriggerScrape handles POST /api/scrape/trigger
func (h *Handlers) TriggerScrape(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement scraper trigger
	// For now, return a placeholder response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "queued",
		"message": "Scrape job has been queued",
	})
}

// GetRoute handles GET /api/route
// Returns a driving route from a property to a town as GeoJSON LineString
func (h *Handlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse coordinates (required)
	fromLat, err1 := strconv.ParseFloat(q.Get("from_lat"), 64)
	fromLng, err2 := strconv.ParseFloat(q.Get("from_lng"), 64)
	townName := q.Get("town")

	if err1 != nil || err2 != nil {
		http.Error(w, "from_lat and from_lng required", http.StatusBadRequest)
		return
	}
	if townName == "" {
		http.Error(w, "town parameter required", http.StatusBadRequest)
		return
	}

	// Look up town coordinates
	var toLat, toLng float64
	found := false
	for _, town := range geo.NSWTowns {
		if strings.EqualFold(town.Name, townName) {
			toLat = town.Latitude
			toLng = town.Longitude
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "town not found", http.StatusNotFound)
		return
	}

	// Get route from Valhalla
	router := geo.NewRouter("")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	route, err := router.GetRouteWithShape(ctx, fromLat, fromLng, toLat, toLng)
	if err != nil {
		http.Error(w, "failed to get route: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build GeoJSON response
	geojson := map[string]interface{}{
		"type": "Feature",
		"geometry": map[string]interface{}{
			"type":        "LineString",
			"coordinates": route.Coordinates,
		},
		"properties": map[string]interface{}{
			"duration_mins": route.DurationMins,
			"distance_km":   route.DistanceKm,
			"town":          townName,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(geojson)
}

// GetBoundaries handles GET /api/boundaries
// Returns cadastral lot boundaries as GeoJSON for properties matching filters
func (h *Handlers) GetBoundaries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse bounds (required)
	boundsStr := q.Get("bounds")
	if boundsStr == "" {
		http.Error(w, "bounds parameter required (sw_lat,sw_lng,ne_lat,ne_lng)", http.StatusBadRequest)
		return
	}

	// Parse all filters (same as properties endpoint)
	filter := parsePropertyFilter(q)

	// Parse zoom level and add buffer at high zoom
	zoom := 0.0
	if v := q.Get("zoom"); v != "" {
		zoom, _ = strconv.ParseFloat(v, 64)
	}

	// Add buffer to bounds only at high zoom levels (14+) to catch large properties
	// whose centroid is just outside viewport when panning
	if zoom >= 14 && filter.SWLat != nil && filter.NELat != nil {
		latSize := *filter.NELat - *filter.SWLat
		lngSize := *filter.NELng - *filter.SWLng
		buffer := latSize
		if lngSize > buffer {
			buffer = lngSize
		}
		buffer *= 2
		swLat := *filter.SWLat - buffer
		swLng := *filter.SWLng - buffer
		neLat := *filter.NELat + buffer
		neLng := *filter.NELng + buffer
		filter.SWLat = &swLat
		filter.SWLng = &swLng
		filter.NELat = &neLat
		filter.NELng = &neLng
	}

	lots, err := h.db.GetBoundariesInBounds(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build GeoJSON FeatureCollection
	features := make([]map[string]interface{}, 0, len(lots))
	for _, lot := range lots {
		// Parse stored geometry JSON
		var geometry interface{}
		if err := json.Unmarshal([]byte(lot.Geometry), &geometry); err != nil {
			continue // Skip lots with invalid geometry
		}

		feature := map[string]interface{}{
			"type":     "Feature",
			"geometry": geometry,
			"properties": map[string]interface{}{
				"lot_id":     lot.LotIDString,
				"lot_number": lot.LotNumber,
				"plan_label": lot.PlanLabel,
				"area_sqm":   lot.AreaSqm,
			},
		}
		features = append(features, feature)
	}

	geojson := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(geojson)
}
