package db

import (
	"encoding/json"
	"farm-search/internal/models"
	"fmt"
	"strings"
)

// PropertyFilter contains all filter parameters for property queries
type PropertyFilter struct {
	PriceMin           *int64
	PriceMax           *int64
	PropertyTypes      []string
	LandSizeMin        *float64
	LandSizeMax        *float64
	DistanceSydneyMax  *float64
	DistanceTownMax    *float64
	DistanceSchoolMax  *float64
	DriveTimeSydneyMax *int
	// Map bounds
	SWLat *float64
	SWLng *float64
	NELat *float64
	NELng *float64
	// Pagination
	Limit  int
	Offset int
}

// ListProperties returns properties matching the given filters
// Excludes duplicate properties (only shows canonical ones)
func (db *DB) ListProperties(f PropertyFilter) ([]models.PropertyListItem, error) {
	query := `
		SELECT DISTINCT
			p.id,
			p.latitude,
			p.longitude,
			COALESCE(p.price_text, '') as price_text,
			COALESCE(p.property_type, '') as property_type,
			COALESCE(p.address, '') as address,
			COALESCE(p.suburb, '') as suburb
		FROM properties p
		LEFT JOIN property_distances pd_sydney ON p.id = pd_sydney.property_id 
			AND pd_sydney.target_type = 'capital' AND pd_sydney.target_name = 'Sydney'
		LEFT JOIN property_distances pd_town ON p.id = pd_town.property_id 
			AND pd_town.target_type = 'town'
		LEFT JOIN property_distances pd_school ON p.id = pd_school.property_id 
			AND pd_school.target_type = 'school'
		LEFT JOIN property_links pl ON p.id = pl.duplicate_id
		WHERE p.latitude IS NOT NULL AND p.longitude IS NOT NULL
			AND pl.duplicate_id IS NULL  -- Exclude properties that are duplicates
	`

	args := make([]interface{}, 0)
	argIndex := 1

	// Price filters
	if f.PriceMin != nil {
		query += fmt.Sprintf(" AND (p.price_max >= ?%d OR p.price_max IS NULL)", argIndex)
		args = append(args, *f.PriceMin)
		argIndex++
	}
	if f.PriceMax != nil {
		query += fmt.Sprintf(" AND (p.price_min <= ?%d OR p.price_min IS NULL)", argIndex)
		args = append(args, *f.PriceMax)
		argIndex++
	}

	// Property type filter
	if len(f.PropertyTypes) > 0 {
		placeholders := make([]string, len(f.PropertyTypes))
		for i, pt := range f.PropertyTypes {
			placeholders[i] = "?"
			args = append(args, pt)
		}
		query += fmt.Sprintf(" AND p.property_type IN (%s)", strings.Join(placeholders, ","))
	}

	// Land size filters
	if f.LandSizeMin != nil {
		query += " AND p.land_size_sqm >= ?"
		args = append(args, *f.LandSizeMin)
	}
	if f.LandSizeMax != nil {
		query += " AND p.land_size_sqm <= ?"
		args = append(args, *f.LandSizeMax)
	}

	// Distance filters
	if f.DistanceSydneyMax != nil {
		query += " AND pd_sydney.distance_km <= ?"
		args = append(args, *f.DistanceSydneyMax)
	}
	if f.DistanceTownMax != nil {
		query += " AND pd_town.distance_km <= ?"
		args = append(args, *f.DistanceTownMax)
	}
	if f.DistanceSchoolMax != nil {
		query += " AND pd_school.distance_km <= ?"
		args = append(args, *f.DistanceSchoolMax)
	}

	// Drive time filter
	if f.DriveTimeSydneyMax != nil {
		query += " AND pd_sydney.drive_time_mins <= ?"
		args = append(args, *f.DriveTimeSydneyMax)
	}

	// Map bounds filter
	if f.SWLat != nil && f.SWLng != nil && f.NELat != nil && f.NELng != nil {
		query += " AND p.latitude BETWEEN ? AND ? AND p.longitude BETWEEN ? AND ?"
		args = append(args, *f.SWLat, *f.NELat, *f.SWLng, *f.NELng)
	}

	// Apply limit
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", f.Offset)
	}

	// Use regular ? placeholders for SQLite
	query = strings.ReplaceAll(query, "?1", "?")
	query = strings.ReplaceAll(query, "?2", "?")
	query = strings.ReplaceAll(query, "?3", "?")
	query = strings.ReplaceAll(query, "?4", "?")

	var properties []models.PropertyListItem
	err := db.Select(&properties, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list properties: %w", err)
	}

	return properties, nil
}

// GetProperty returns a single property by ID with full details
func (db *DB) GetProperty(id int64) (*models.PropertyDetail, error) {
	query := `
		SELECT 
			id, external_id, source, url,
			COALESCE(address, '') as address,
			COALESCE(suburb, '') as suburb,
			state,
			COALESCE(postcode, '') as postcode,
			latitude, longitude,
			price_min, price_max,
			COALESCE(price_text, '') as price_text,
			COALESCE(property_type, '') as property_type,
			bedrooms, bathrooms, land_size_sqm,
			COALESCE(description, '') as description,
			COALESCE(images, '[]') as images,
			listed_at
		FROM properties WHERE id = ?
	`

	var p struct {
		ID           int64    `db:"id"`
		ExternalID   string   `db:"external_id"`
		Source       string   `db:"source"`
		URL          string   `db:"url"`
		Address      string   `db:"address"`
		Suburb       string   `db:"suburb"`
		State        string   `db:"state"`
		Postcode     string   `db:"postcode"`
		Latitude     float64  `db:"latitude"`
		Longitude    float64  `db:"longitude"`
		PriceMin     *int64   `db:"price_min"`
		PriceMax     *int64   `db:"price_max"`
		PriceText    string   `db:"price_text"`
		PropertyType string   `db:"property_type"`
		Bedrooms     *int64   `db:"bedrooms"`
		Bathrooms    *int64   `db:"bathrooms"`
		LandSizeSqm  *float64 `db:"land_size_sqm"`
		Description  string   `db:"description"`
		Images       string   `db:"images"`
		ListedAt     *string  `db:"listed_at"`
	}

	err := db.Get(&p, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get property: %w", err)
	}

	var images []string
	json.Unmarshal([]byte(p.Images), &images)

	// Get all sources for this property
	sources, _ := db.GetPropertySources(id)

	return &models.PropertyDetail{
		ID:           p.ID,
		ExternalID:   p.ExternalID,
		Source:       p.Source,
		URL:          p.URL,
		Sources:      sources,
		Address:      p.Address,
		Suburb:       p.Suburb,
		State:        p.State,
		Postcode:     p.Postcode,
		Latitude:     p.Latitude,
		Longitude:    p.Longitude,
		PriceMin:     p.PriceMin,
		PriceMax:     p.PriceMax,
		PriceText:    p.PriceText,
		PropertyType: p.PropertyType,
		Bedrooms:     p.Bedrooms,
		Bathrooms:    p.Bathrooms,
		LandSizeSqm:  p.LandSizeSqm,
		Description:  p.Description,
		Images:       images,
		ListedAt:     p.ListedAt,
	}, nil
}

// GetFilterOptions returns available values for filter dropdowns
func (db *DB) GetFilterOptions() (map[string]interface{}, error) {
	options := make(map[string]interface{})

	// Get distinct property types
	var types []string
	err := db.Select(&types, "SELECT DISTINCT property_type FROM properties WHERE property_type IS NOT NULL ORDER BY property_type")
	if err != nil {
		return nil, err
	}
	options["property_types"] = types

	// Get price range
	var priceRange struct {
		Min *int64 `db:"min_price"`
		Max *int64 `db:"max_price"`
	}
	err = db.Get(&priceRange, "SELECT MIN(price_min) as min_price, MAX(price_max) as max_price FROM properties")
	if err != nil {
		return nil, err
	}
	options["price_min"] = priceRange.Min
	options["price_max"] = priceRange.Max

	// Get land size range
	var landRange struct {
		Min *float64 `db:"min_size"`
		Max *float64 `db:"max_size"`
	}
	err = db.Get(&landRange, "SELECT MIN(land_size_sqm) as min_size, MAX(land_size_sqm) as max_size FROM properties")
	if err != nil {
		return nil, err
	}
	options["land_size_min"] = landRange.Min
	options["land_size_max"] = landRange.Max

	return options, nil
}

// UpsertProperty inserts or updates a property based on external_id
func (db *DB) UpsertProperty(p *models.Property) error {
	query := `
		INSERT INTO properties (
			external_id, source, url, address, suburb, state, postcode,
			latitude, longitude, price_min, price_max, price_text,
			property_type, bedrooms, bathrooms, land_size_sqm,
			description, images, listed_at, scraped_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?
		)
		ON CONFLICT(external_id, source) DO UPDATE SET
			url = excluded.url,
			address = COALESCE(excluded.address, properties.address),
			suburb = COALESCE(excluded.suburb, properties.suburb),
			postcode = COALESCE(excluded.postcode, properties.postcode),
			latitude = COALESCE(excluded.latitude, properties.latitude),
			longitude = COALESCE(excluded.longitude, properties.longitude),
			price_min = COALESCE(excluded.price_min, properties.price_min),
			price_max = COALESCE(excluded.price_max, properties.price_max),
			price_text = COALESCE(excluded.price_text, properties.price_text),
			property_type = COALESCE(excluded.property_type, properties.property_type),
			bedrooms = COALESCE(excluded.bedrooms, properties.bedrooms),
			bathrooms = COALESCE(excluded.bathrooms, properties.bathrooms),
			land_size_sqm = COALESCE(excluded.land_size_sqm, properties.land_size_sqm),
			description = COALESCE(excluded.description, properties.description),
			images = COALESCE(excluded.images, properties.images),
			scraped_at = excluded.scraped_at,
			updated_at = excluded.updated_at
	`

	_, err := db.Exec(query,
		p.ExternalID, p.Source, p.URL,
		p.Address, p.Suburb, p.State, p.Postcode,
		p.Latitude, p.Longitude,
		p.PriceMin, p.PriceMax, p.PriceText,
		p.PropertyType, p.Bedrooms, p.Bathrooms, p.LandSizeSqm,
		p.Description, p.Images, p.ListedAt,
		p.ScrapedAt, p.UpdatedAt,
	)

	return err
}

// GetPropertyCount returns total number of properties
func (db *DB) GetPropertyCount() (int, error) {
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM properties")
	return count, err
}

// SavePropertyDistance saves or updates a property distance calculation
func (db *DB) SavePropertyDistance(propertyID int64, targetType, targetName string, distanceKm float64) error {
	query := `
		INSERT INTO property_distances (property_id, target_type, target_name, distance_km)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(property_id, target_type, target_name) DO UPDATE SET
			distance_km = excluded.distance_km
	`
	_, err := db.Exec(query, propertyID, targetType, targetName, distanceKm)
	return err
}

// FindDuplicateProperties finds properties that appear to be the same based on coordinates
// Properties within ~100m of each other are considered potential duplicates
func (db *DB) FindDuplicateProperties() error {
	// Find properties with nearly identical coordinates (within ~0.001 degrees ≈ 100m)
	query := `
		INSERT OR IGNORE INTO property_links (canonical_id, duplicate_id, match_type)
		SELECT 
			p1.id as canonical_id,
			p2.id as duplicate_id,
			'coords' as match_type
		FROM properties p1
		JOIN properties p2 ON p1.id < p2.id
			AND p1.source != p2.source
			AND ABS(p1.latitude - p2.latitude) < 0.001
			AND ABS(p1.longitude - p2.longitude) < 0.001
			AND p1.latitude IS NOT NULL
			AND p2.latitude IS NOT NULL
		WHERE NOT EXISTS (
			SELECT 1 FROM property_links pl 
			WHERE pl.duplicate_id = p2.id
		)
	`
	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to find duplicates: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		fmt.Printf("Linked %d duplicate properties\n", rows)
	}

	return nil
}

// GetPropertySources returns all sources where a property is listed
func (db *DB) GetPropertySources(propertyID int64) ([]models.PropertySource, error) {
	// Get the property itself
	var sources []models.PropertySource

	// First, add the main property's source
	var mainSource models.PropertySource
	err := db.Get(&mainSource, "SELECT source, url FROM properties WHERE id = ?", propertyID)
	if err != nil {
		return nil, err
	}
	sources = append(sources, mainSource)

	// Check if this property is a canonical (has duplicates linked to it)
	var linkedSources []models.PropertySource
	err = db.Select(&linkedSources, `
		SELECT p.source, p.url 
		FROM properties p
		JOIN property_links pl ON p.id = pl.duplicate_id
		WHERE pl.canonical_id = ?
	`, propertyID)
	if err == nil {
		sources = append(sources, linkedSources...)
	}

	// Check if this property is a duplicate (linked to a canonical)
	var canonicalID int64
	err = db.Get(&canonicalID, "SELECT canonical_id FROM property_links WHERE duplicate_id = ?", propertyID)
	if err == nil {
		// Get the canonical property's source
		var canonicalSource models.PropertySource
		db.Get(&canonicalSource, "SELECT source, url FROM properties WHERE id = ?", canonicalID)
		sources = append(sources, canonicalSource)

		// Get other duplicates of the same canonical
		var otherSources []models.PropertySource
		db.Select(&otherSources, `
			SELECT p.source, p.url 
			FROM properties p
			JOIN property_links pl ON p.id = pl.duplicate_id
			WHERE pl.canonical_id = ? AND p.id != ?
		`, canonicalID, propertyID)
		sources = append(sources, otherSources...)
	}

	return sources, nil
}

// GetPropertyDistances returns all distance calculations for a property
func (db *DB) GetPropertyDistances(propertyID int64) ([]models.PropertyDistance, error) {
	query := `SELECT property_id, target_type, target_name, distance_km, drive_time_mins 
			  FROM property_distances WHERE property_id = ?`
	var distances []models.PropertyDistance
	err := db.Select(&distances, query, propertyID)
	return distances, err
}
