package api

import (
	"encoding/json"
	"farm-search/internal/db"
	"net/http"
	"strconv"
	"strings"

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

// ListProperties handles GET /api/properties
func (h *Handlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := db.PropertyFilter{}

	// Parse price filters
	if v := q.Get("price_min"); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PriceMin = &val
		}
	}
	if v := q.Get("price_max"); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.PriceMax = &val
		}
	}

	// Parse property types
	if v := q.Get("type"); v != "" {
		filter.PropertyTypes = strings.Split(v, ",")
	}

	// Parse land size filters
	if v := q.Get("land_size_min"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.LandSizeMin = &val
		}
	}
	if v := q.Get("land_size_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.LandSizeMax = &val
		}
	}

	// Parse distance filters
	if v := q.Get("distance_sydney_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceSydneyMax = &val
		}
	}
	if v := q.Get("distance_town_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceTownMax = &val
		}
	}
	if v := q.Get("distance_school_max"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			filter.DistanceSchoolMax = &val
		}
	}

	// Parse drive time filters
	if v := q.Get("drive_time_sydney_max"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			filter.DriveTimeSydneyMax = &val
		}
	}
	if v := q.Get("drive_time_town_max"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			filter.DriveTimeTownMax = &val
		}
	}

	// Parse map bounds (sw_lat,sw_lng,ne_lat,ne_lng)
	if v := q.Get("bounds"); v != "" {
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
	if v := q.Get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 && val <= 500 {
			filter.Limit = val
		}
	}
	if v := q.Get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 0 {
			filter.Offset = val
		}
	}

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

// GetBoundaries handles GET /api/boundaries
// Returns cadastral lot boundaries as GeoJSON for properties within map bounds
func (h *Handlers) GetBoundaries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse bounds (required)
	boundsStr := q.Get("bounds")
	if boundsStr == "" {
		http.Error(w, "bounds parameter required (sw_lat,sw_lng,ne_lat,ne_lng)", http.StatusBadRequest)
		return
	}

	parts := strings.Split(boundsStr, ",")
	if len(parts) != 4 {
		http.Error(w, "bounds must have 4 values: sw_lat,sw_lng,ne_lat,ne_lng", http.StatusBadRequest)
		return
	}

	swLat, err1 := strconv.ParseFloat(parts[0], 64)
	swLng, err2 := strconv.ParseFloat(parts[1], 64)
	neLat, err3 := strconv.ParseFloat(parts[2], 64)
	neLng, err4 := strconv.ParseFloat(parts[3], 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		http.Error(w, "invalid bounds values", http.StatusBadRequest)
		return
	}

	// Parse zoom level (optional)
	zoom := 0.0
	if v := q.Get("zoom"); v != "" {
		zoom, _ = strconv.ParseFloat(v, 64)
	}

	// Add buffer to bounds only at high zoom levels (14+) to catch large properties
	// whose centroid is just outside viewport when panning
	if zoom >= 14 {
		latSize := neLat - swLat
		lngSize := neLng - swLng
		buffer := latSize
		if lngSize > buffer {
			buffer = lngSize
		}
		buffer *= 2
		swLat -= buffer
		swLng -= buffer
		neLat += buffer
		neLng += buffer
	}

	lots, err := h.db.GetBoundariesInBounds(swLat, swLng, neLat, neLng)
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
