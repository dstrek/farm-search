package models

import (
	"database/sql"
	"time"
)

// Property represents a real estate listing
type Property struct {
	ID           int64           `db:"id" json:"id"`
	ExternalID   string          `db:"external_id" json:"external_id"`
	Source       string          `db:"source" json:"source"`
	URL          string          `db:"url" json:"url"`
	Address      sql.NullString  `db:"address" json:"address"`
	Suburb       sql.NullString  `db:"suburb" json:"suburb"`
	State        string          `db:"state" json:"state"`
	Postcode     sql.NullString  `db:"postcode" json:"postcode"`
	Latitude     sql.NullFloat64 `db:"latitude" json:"latitude"`
	Longitude    sql.NullFloat64 `db:"longitude" json:"longitude"`
	PriceMin     sql.NullInt64   `db:"price_min" json:"price_min"`
	PriceMax     sql.NullInt64   `db:"price_max" json:"price_max"`
	PriceText    sql.NullString  `db:"price_text" json:"price_text"`
	PropertyType sql.NullString  `db:"property_type" json:"property_type"`
	Bedrooms     sql.NullInt64   `db:"bedrooms" json:"bedrooms"`
	Bathrooms    sql.NullInt64   `db:"bathrooms" json:"bathrooms"`
	LandSizeSqm  sql.NullFloat64 `db:"land_size_sqm" json:"land_size_sqm"`
	Description  sql.NullString  `db:"description" json:"description"`
	Images       sql.NullString  `db:"images" json:"images"` // JSON array
	ListedAt     sql.NullTime    `db:"listed_at" json:"listed_at"`
	ScrapedAt    time.Time       `db:"scraped_at" json:"scraped_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

// PropertyDistance represents pre-computed distance from a property to a target
type PropertyDistance struct {
	PropertyID    int64           `db:"property_id" json:"property_id"`
	TargetType    string          `db:"target_type" json:"target_type"`
	TargetName    string          `db:"target_name" json:"target_name"`
	DistanceKm    sql.NullFloat64 `db:"distance_km" json:"distance_km"`
	DriveTimeMins sql.NullInt64   `db:"drive_time_mins" json:"drive_time_mins"`
}

// Town represents a reference town for distance calculations
type Town struct {
	ID         int64   `db:"id" json:"id"`
	Name       string  `db:"name" json:"name"`
	State      string  `db:"state" json:"state"`
	Latitude   float64 `db:"latitude" json:"latitude"`
	Longitude  float64 `db:"longitude" json:"longitude"`
	Population int     `db:"population" json:"population"`
}

// School represents a reference school for distance calculations
type School struct {
	ID         int64          `db:"id" json:"id"`
	Name       string         `db:"name" json:"name"`
	SchoolType sql.NullString `db:"school_type" json:"school_type"`
	Suburb     sql.NullString `db:"suburb" json:"suburb"`
	State      string         `db:"state" json:"state"`
	Latitude   float64        `db:"latitude" json:"latitude"`
	Longitude  float64        `db:"longitude" json:"longitude"`
}

// PropertyListItem is a lightweight property for map markers
type PropertyListItem struct {
	ID              int64   `db:"id" json:"id"`
	Latitude        float64 `db:"latitude" json:"lat"`
	Longitude       float64 `db:"longitude" json:"lng"`
	PriceText       string  `db:"price_text" json:"price_text"`
	PropertyType    string  `db:"property_type" json:"property_type"`
	Address         string  `db:"address" json:"address"`
	Suburb          string  `db:"suburb" json:"suburb"`
	Source          string  `db:"source" json:"source"`
	DriveTimeSydney *int    `db:"drive_time_sydney" json:"drive_time_sydney,omitempty"`
}

// PropertySource represents a listing source for a property
type PropertySource struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

// PropertyDetail is the full property info for popup/modal
type PropertyDetail struct {
	ID              int64            `json:"id"`
	ExternalID      string           `json:"external_id"`
	Source          string           `json:"source"`
	URL             string           `json:"url"`
	Sources         []PropertySource `json:"sources,omitempty"` // All sources where this property is listed
	Address         string           `json:"address"`
	Suburb          string           `json:"suburb"`
	State           string           `json:"state"`
	Postcode        string           `json:"postcode"`
	Latitude        float64          `json:"lat"`
	Longitude       float64          `json:"lng"`
	PriceMin        *int64           `json:"price_min,omitempty"`
	PriceMax        *int64           `json:"price_max,omitempty"`
	PriceText       string           `json:"price_text"`
	PropertyType    string           `json:"property_type"`
	Bedrooms        *int64           `json:"bedrooms,omitempty"`
	Bathrooms       *int64           `json:"bathrooms,omitempty"`
	LandSizeSqm     *float64         `json:"land_size_sqm,omitempty"`
	Description     string           `json:"description"`
	Images          []string         `json:"images"`
	ListedAt        *string          `json:"listed_at,omitempty"`
	DriveTimeSydney *int             `json:"drive_time_sydney,omitempty"` // Drive time to Sutherland in minutes
}
