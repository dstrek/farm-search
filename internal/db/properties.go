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
	DriveTimeSydneyMax *int
	DriveTimeTownMax   *int // Drive time to nearest town in minutes
	DriveTimeSchoolMax *int // Drive time to nearest school in minutes
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
			COALESCE(p.suburb, '') as suburb,
			p.source,
			p.drive_time_sydney
		FROM properties p
		LEFT JOIN property_distances pd_sydney ON p.id = pd_sydney.property_id 
			AND pd_sydney.target_type = 'capital' AND pd_sydney.target_name = 'Sydney'
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
		// Use pre-computed nearest_town_1_km on properties table (much faster)
		query += " AND p.nearest_town_1_km <= ?"
		args = append(args, *f.DistanceTownMax)
	}
	// Drive time filters (use pre-computed columns on properties table)
	if f.DriveTimeSydneyMax != nil {
		query += " AND p.drive_time_sydney <= ?"
		args = append(args, *f.DriveTimeSydneyMax)
	}
	if f.DriveTimeTownMax != nil {
		query += " AND p.nearest_town_1_mins <= ?"
		args = append(args, *f.DriveTimeTownMax)
	}
	if f.DriveTimeSchoolMax != nil {
		query += " AND p.nearest_school_1_mins <= ?"
		args = append(args, *f.DriveTimeSchoolMax)
	}

	// Map bounds filter
	if f.SWLat != nil && f.SWLng != nil && f.NELat != nil && f.NELng != nil {
		query += " AND p.latitude BETWEEN ? AND ? AND p.longitude BETWEEN ? AND ?"
		args = append(args, *f.SWLat, *f.NELat, *f.SWLng, *f.NELng)
	}

	// Apply limit only if specified
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

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
			listed_at,
			drive_time_sydney,
			nearest_town_1, nearest_town_1_km, nearest_town_1_mins,
			nearest_town_2, nearest_town_2_km, nearest_town_2_mins,
			nearest_school_1, nearest_school_1_km, nearest_school_1_mins, nearest_school_1_lat, nearest_school_1_lng,
			nearest_school_2, nearest_school_2_km, nearest_school_2_mins, nearest_school_2_lat, nearest_school_2_lng
		FROM properties WHERE id = ?
	`

	var p struct {
		ID                 int64    `db:"id"`
		ExternalID         string   `db:"external_id"`
		Source             string   `db:"source"`
		URL                string   `db:"url"`
		Address            string   `db:"address"`
		Suburb             string   `db:"suburb"`
		State              string   `db:"state"`
		Postcode           string   `db:"postcode"`
		Latitude           float64  `db:"latitude"`
		Longitude          float64  `db:"longitude"`
		PriceMin           *int64   `db:"price_min"`
		PriceMax           *int64   `db:"price_max"`
		PriceText          string   `db:"price_text"`
		PropertyType       string   `db:"property_type"`
		Bedrooms           *int64   `db:"bedrooms"`
		Bathrooms          *int64   `db:"bathrooms"`
		LandSizeSqm        *float64 `db:"land_size_sqm"`
		Description        string   `db:"description"`
		Images             string   `db:"images"`
		ListedAt           *string  `db:"listed_at"`
		DriveTimeSydney    *int     `db:"drive_time_sydney"`
		NearestTown1       *string  `db:"nearest_town_1"`
		NearestTown1Km     *float64 `db:"nearest_town_1_km"`
		NearestTown1Mins   *int     `db:"nearest_town_1_mins"`
		NearestTown2       *string  `db:"nearest_town_2"`
		NearestTown2Km     *float64 `db:"nearest_town_2_km"`
		NearestTown2Mins   *int     `db:"nearest_town_2_mins"`
		NearestSchool1     *string  `db:"nearest_school_1"`
		NearestSchool1Km   *float64 `db:"nearest_school_1_km"`
		NearestSchool1Mins *int     `db:"nearest_school_1_mins"`
		NearestSchool1Lat  *float64 `db:"nearest_school_1_lat"`
		NearestSchool1Lng  *float64 `db:"nearest_school_1_lng"`
		NearestSchool2     *string  `db:"nearest_school_2"`
		NearestSchool2Km   *float64 `db:"nearest_school_2_km"`
		NearestSchool2Mins *int     `db:"nearest_school_2_mins"`
		NearestSchool2Lat  *float64 `db:"nearest_school_2_lat"`
		NearestSchool2Lng  *float64 `db:"nearest_school_2_lng"`
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
		ID:                 p.ID,
		ExternalID:         p.ExternalID,
		Source:             p.Source,
		URL:                p.URL,
		Sources:            sources,
		Address:            p.Address,
		Suburb:             p.Suburb,
		State:              p.State,
		Postcode:           p.Postcode,
		Latitude:           p.Latitude,
		Longitude:          p.Longitude,
		PriceMin:           p.PriceMin,
		PriceMax:           p.PriceMax,
		PriceText:          p.PriceText,
		PropertyType:       p.PropertyType,
		Bedrooms:           p.Bedrooms,
		Bathrooms:          p.Bathrooms,
		LandSizeSqm:        p.LandSizeSqm,
		Description:        p.Description,
		Images:             images,
		ListedAt:           p.ListedAt,
		DriveTimeSydney:    p.DriveTimeSydney,
		NearestTown1:       p.NearestTown1,
		NearestTown1Km:     p.NearestTown1Km,
		NearestTown1Mins:   p.NearestTown1Mins,
		NearestTown2:       p.NearestTown2,
		NearestTown2Km:     p.NearestTown2Km,
		NearestTown2Mins:   p.NearestTown2Mins,
		NearestSchool1:     p.NearestSchool1,
		NearestSchool1Km:   p.NearestSchool1Km,
		NearestSchool1Mins: p.NearestSchool1Mins,
		NearestSchool1Lat:  p.NearestSchool1Lat,
		NearestSchool1Lng:  p.NearestSchool1Lng,
		NearestSchool2:     p.NearestSchool2,
		NearestSchool2Km:   p.NearestSchool2Km,
		NearestSchool2Mins: p.NearestSchool2Mins,
		NearestSchool2Lat:  p.NearestSchool2Lat,
		NearestSchool2Lng:  p.NearestSchool2Lng,
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

// UpdatePropertyDriveTime updates the drive time to Sydney for a property
func (db *DB) UpdatePropertyDriveTime(propertyID int64, driveTimeMins int) error {
	_, err := db.Exec("UPDATE properties SET drive_time_sydney = ? WHERE id = ?", driveTimeMins, propertyID)
	return err
}

// GetPropertiesWithoutDriveTime returns properties that don't have drive time calculated
func (db *DB) GetPropertiesWithoutDriveTime() ([]models.PropertyListItem, error) {
	query := `
		SELECT 
			id,
			latitude,
			longitude,
			COALESCE(price_text, '') as price_text,
			COALESCE(property_type, '') as property_type,
			COALESCE(address, '') as address,
			COALESCE(suburb, '') as suburb,
			source
		FROM properties 
		WHERE latitude IS NOT NULL 
			AND longitude IS NOT NULL 
			AND drive_time_sydney IS NULL
	`
	var properties []models.PropertyListItem
	err := db.Select(&properties, query)
	return properties, err
}

// SaveCadastralLot inserts or updates a cadastral lot
func (db *DB) SaveCadastralLot(lotIDString, lotNumber, planLabel string, areaSqm, centroidLat, centroidLng float64, geometry string) (int64, error) {
	query := `
		INSERT INTO cadastral_lots (lot_id_string, lot_number, plan_label, area_sqm, geometry, centroid_lat, centroid_lng, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(lot_id_string) DO UPDATE SET
			lot_number = excluded.lot_number,
			plan_label = excluded.plan_label,
			area_sqm = excluded.area_sqm,
			geometry = excluded.geometry,
			centroid_lat = excluded.centroid_lat,
			centroid_lng = excluded.centroid_lng,
			fetched_at = excluded.fetched_at
		RETURNING id
	`

	var id int64
	err := db.Get(&id, query, lotIDString, lotNumber, planLabel, areaSqm, geometry, centroidLat, centroidLng)
	if err != nil {
		return 0, fmt.Errorf("failed to save cadastral lot: %w", err)
	}
	return id, nil
}

// GetCadastralLotByID returns a cadastral lot by its lot_id_string
func (db *DB) GetCadastralLotByID(lotIDString string) (*models.CadastralLot, error) {
	var lot models.CadastralLot
	err := db.Get(&lot, "SELECT * FROM cadastral_lots WHERE lot_id_string = ?", lotIDString)
	if err != nil {
		return nil, err
	}
	return &lot, nil
}

// LinkPropertyToLot creates a link between a property and a cadastral lot
func (db *DB) LinkPropertyToLot(propertyID, lotID int64) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO property_lots (property_id, lot_id)
		VALUES (?, ?)
	`, propertyID, lotID)
	return err
}

// GetPropertyLots returns all cadastral lots linked to a property
func (db *DB) GetPropertyLots(propertyID int64) ([]models.CadastralLot, error) {
	query := `
		SELECT cl.* FROM cadastral_lots cl
		JOIN property_lots pl ON cl.id = pl.lot_id
		WHERE pl.property_id = ?
	`
	var lots []models.CadastralLot
	err := db.Select(&lots, query, propertyID)
	return lots, err
}

// GetPropertiesWithoutLots returns properties that don't have cadastral lots linked
func (db *DB) GetPropertiesWithoutLots() ([]models.PropertyListItem, error) {
	query := `
		SELECT 
			p.id,
			p.latitude,
			p.longitude,
			COALESCE(p.price_text, '') as price_text,
			COALESCE(p.property_type, '') as property_type,
			COALESCE(p.address, '') as address,
			COALESCE(p.suburb, '') as suburb,
			p.source
		FROM properties p
		LEFT JOIN property_lots pl ON p.id = pl.property_id
		WHERE p.latitude IS NOT NULL 
			AND p.longitude IS NOT NULL 
			AND pl.property_id IS NULL
	`
	var properties []models.PropertyListItem
	err := db.Select(&properties, query)
	return properties, err
}

// GetCadastralLotCount returns total number of cadastral lots
func (db *DB) GetCadastralLotCount() (int, error) {
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM cadastral_lots")
	return count, err
}

// PropertyExists checks if a property with the given external_id and source already exists
func (db *DB) PropertyExists(externalID, source string) (bool, error) {
	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM properties WHERE external_id = ? AND source = ?", externalID, source)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// PropertiesExist checks if properties with the given external_ids and source already exist
// Returns a map of external_id -> exists
func (db *DB) PropertiesExist(externalIDs []string, source string) (map[string]bool, error) {
	if len(externalIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(externalIDs))
	args := make([]interface{}, len(externalIDs)+1)
	args[0] = source
	for i, id := range externalIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf(
		"SELECT external_id FROM properties WHERE source = ? AND external_id IN (%s)",
		strings.Join(placeholders, ","),
	)

	var existingIDs []string
	err := db.Select(&existingIDs, query, args...)
	if err != nil {
		return nil, err
	}

	// Build result map
	result := make(map[string]bool)
	for _, id := range existingIDs {
		result[id] = true
	}
	return result, nil
}

// GetBoundariesInBounds returns cadastral lot boundaries for properties matching the filter
// Returns a list of lots with their geometry (GeoJSON) and associated property IDs
// Applies the same filters as ListProperties to ensure boundaries match visible properties
func (db *DB) GetBoundariesInBounds(f PropertyFilter) ([]models.CadastralLot, error) {
	// Build base query with same joins as ListProperties for filtering
	query := `
		SELECT DISTINCT cl.id, cl.lot_id_string, cl.lot_number, cl.plan_label, 
			   cl.area_sqm, cl.geometry, cl.centroid_lat, cl.centroid_lng, cl.fetched_at
		FROM cadastral_lots cl
		JOIN property_lots pl ON cl.id = pl.lot_id
		JOIN properties p ON pl.property_id = p.id
		LEFT JOIN property_distances pd_sydney ON p.id = pd_sydney.property_id 
			AND pd_sydney.target_type = 'capital' AND pd_sydney.target_name = 'Sydney'
		LEFT JOIN property_links plink ON p.id = plink.duplicate_id
		WHERE p.latitude IS NOT NULL AND p.longitude IS NOT NULL
			AND plink.duplicate_id IS NULL
	`

	args := make([]interface{}, 0)

	// Price filters
	if f.PriceMin != nil {
		query += " AND (p.price_max >= ? OR p.price_max IS NULL)"
		args = append(args, *f.PriceMin)
	}
	if f.PriceMax != nil {
		query += " AND (p.price_min <= ? OR p.price_min IS NULL)"
		args = append(args, *f.PriceMax)
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
		query += " AND p.nearest_town_1_km <= ?"
		args = append(args, *f.DistanceTownMax)
	}
	// Drive time filters
	if f.DriveTimeSydneyMax != nil {
		query += " AND p.drive_time_sydney <= ?"
		args = append(args, *f.DriveTimeSydneyMax)
	}
	if f.DriveTimeTownMax != nil {
		query += " AND p.nearest_town_1_mins <= ?"
		args = append(args, *f.DriveTimeTownMax)
	}
	if f.DriveTimeSchoolMax != nil {
		query += " AND p.nearest_school_1_mins <= ?"
		args = append(args, *f.DriveTimeSchoolMax)
	}

	// Map bounds filter - check both property coords and lot centroid
	if f.SWLat != nil && f.SWLng != nil && f.NELat != nil && f.NELng != nil {
		query += ` AND (
			(p.latitude BETWEEN ? AND ? AND p.longitude BETWEEN ? AND ?)
			OR (cl.centroid_lat BETWEEN ? AND ? AND cl.centroid_lng BETWEEN ? AND ?)
		)`
		args = append(args, *f.SWLat, *f.NELat, *f.SWLng, *f.NELng)
		args = append(args, *f.SWLat, *f.NELat, *f.SWLng, *f.NELng)
	}

	query += " LIMIT 500"

	var lots []models.CadastralLot
	err := db.Select(&lots, query, args...)
	return lots, err
}

// REAPropertyForDetails represents a REA property that needs details fetched
type REAPropertyForDetails struct {
	ID         int64  `db:"id"`
	ExternalID string `db:"external_id"`
	URL        string `db:"url"`
	Address    string `db:"address"`
	Suburb     string `db:"suburb"`
}

// GetREAPropertiesWithoutDetails returns REA properties that haven't had their details scraped yet
func (db *DB) GetREAPropertiesWithoutDetails(limit int) ([]REAPropertyForDetails, error) {
	query := `
		SELECT id, external_id, url, COALESCE(address, '') as address, COALESCE(suburb, '') as suburb
		FROM properties 
		WHERE source = 'rea' 
		  AND details_scraped_at IS NULL
		ORDER BY scraped_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var properties []REAPropertyForDetails
	err := db.Select(&properties, query)
	return properties, err
}

// MarkPropertyDetailsScraped marks a property as having had its details successfully scraped
func (db *DB) MarkPropertyDetailsScraped(id int64) error {
	_, err := db.Exec("UPDATE properties SET details_scraped_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// UpdatePropertyFromDetails updates a property with details fetched from the listing page
func (db *DB) UpdatePropertyFromDetails(id int64, description, images string, landSizeSqm *float64, bedrooms, bathrooms *int64, priceMin, priceMax *int64) error {
	_, err := db.Exec(`
		UPDATE properties SET
			description = COALESCE(?, description),
			images = COALESCE(?, images),
			land_size_sqm = COALESCE(?, land_size_sqm),
			bedrooms = COALESCE(?, bedrooms),
			bathrooms = COALESCE(?, bathrooms),
			price_min = COALESCE(?, price_min),
			price_max = COALESCE(?, price_max),
			details_scraped_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, description, images, landSizeSqm, bedrooms, bathrooms, priceMin, priceMax, id)
	return err
}

// UpdatePropertyLandSize updates the land size for a property
func (db *DB) UpdatePropertyLandSize(id int64, landSizeSqm float64) error {
	_, err := db.Exec(`
		UPDATE properties SET
			land_size_sqm = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, landSizeSqm, id)
	return err
}

// GetPropertiesWithSmallLandSize returns properties with cadastral lots where land_size_sqm is NULL or < threshold
func (db *DB) GetPropertiesWithSmallLandSize(thresholdSqm float64) ([]struct {
	ID          int64   `db:"id"`
	Address     string  `db:"address"`
	Suburb      string  `db:"suburb"`
	LandSizeSqm float64 `db:"land_size_sqm"`
}, error) {
	query := `
		SELECT DISTINCT p.id, COALESCE(p.address, '') as address, COALESCE(p.suburb, '') as suburb, 
		       COALESCE(p.land_size_sqm, 0) as land_size_sqm
		FROM properties p
		INNER JOIN property_lots pl ON p.id = pl.property_id
		WHERE p.land_size_sqm IS NULL OR p.land_size_sqm < ?
	`
	var properties []struct {
		ID          int64   `db:"id"`
		Address     string  `db:"address"`
		Suburb      string  `db:"suburb"`
		LandSizeSqm float64 `db:"land_size_sqm"`
	}
	err := db.Select(&properties, query, thresholdSqm)
	return properties, err
}

// GetTotalCadastralAreaForProperty returns the sum of cadastral lot areas for a property
func (db *DB) GetTotalCadastralAreaForProperty(propertyID int64) (float64, error) {
	var totalArea float64
	err := db.Get(&totalArea, `
		SELECT COALESCE(SUM(cl.area_sqm), 0)
		FROM cadastral_lots cl
		INNER JOIN property_lots pl ON cl.id = pl.lot_id
		WHERE pl.property_id = ?
	`, propertyID)
	return totalArea, err
}
