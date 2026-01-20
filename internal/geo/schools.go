package geo

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// School represents a NSW school
type School struct {
	Name      string
	Type      string // Primary, Secondary, Combined
	Suburb    string
	Latitude  float64
	Longitude float64
}

// SchoolData holds NSW schools data
type SchoolData struct {
	Schools []School
}

// NewSchoolData creates a new school data store
func NewSchoolData() *SchoolData {
	return &SchoolData{
		Schools: make([]School, 0),
	}
}

// LoadFromNSWData loads schools from NSW Education data
// URL: https://data.nsw.gov.au/data/dataset/nsw-education-nsw-public-schools-master-dataset
func (sd *SchoolData) LoadFromNSWData(ctx context.Context) error {
	// NSW Government open data URL for school locations
	url := "https://data.nsw.gov.au/data/dataset/78c10ea3-8d04-4c9c-b255-bbf8547e37e7/resource/3e6d5f6a-055c-440d-a690-fc0537c31095/download/master_dataset.csv"

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch school data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch school data: status %d", resp.StatusCode)
	}

	return sd.parseCSV(resp.Body)
}

func (sd *SchoolData) parseCSV(r io.Reader) error {
	reader := csv.NewReader(r)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return err
	}

	// Find column indices
	nameIdx := -1
	typeIdx := -1
	suburbIdx := -1
	latIdx := -1
	lngIdx := -1

	for i, col := range header {
		colLower := strings.ToLower(strings.TrimSpace(col))
		switch colLower {
		case "school_name":
			nameIdx = i
		case "level_of_schooling":
			typeIdx = i
		case "town_suburb":
			suburbIdx = i
		case "latitude":
			latIdx = i
		case "longitude":
			lngIdx = i
		}
	}

	if nameIdx == -1 || latIdx == -1 || lngIdx == -1 {
		return fmt.Errorf("required columns not found in CSV (need school_name, latitude, longitude)")
	}

	// Read data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(record[latIdx]), 64)
		if err != nil {
			continue
		}
		lng, err := strconv.ParseFloat(strings.TrimSpace(record[lngIdx]), 64)
		if err != nil {
			continue
		}

		// Validate coordinates are in NSW
		if lat > -27 || lat < -38 || lng < 140 || lng > 154 {
			continue
		}

		school := School{
			Name:      record[nameIdx],
			Latitude:  lat,
			Longitude: lng,
		}

		if typeIdx >= 0 && typeIdx < len(record) {
			school.Type = record[typeIdx]
		}
		if suburbIdx >= 0 && suburbIdx < len(record) {
			school.Suburb = record[suburbIdx]
		}

		// Only include primary schools (Primary or Infants)
		schoolType := strings.ToLower(school.Type)
		if !strings.Contains(schoolType, "primary") && !strings.Contains(schoolType, "infants") {
			continue
		}

		sd.Schools = append(sd.Schools, school)
	}

	return nil
}

// FindNearestSchool finds the nearest school to a given location
func (sd *SchoolData) FindNearestSchool(lat, lng float64) (School, float64) {
	var nearest School
	minDist := math.MaxFloat64

	for _, school := range sd.Schools {
		dist := Haversine(lat, lng, school.Latitude, school.Longitude)
		if dist < minDist {
			minDist = dist
			nearest = school
		}
	}

	return nearest, minDist
}

// NearestSchoolResult holds school and distance info
type NearestSchoolResult struct {
	Name       string
	Type       string
	Suburb     string
	Latitude   float64
	Longitude  float64
	DistanceKm float64
}

// FindTwoNearestSchools finds the two nearest schools to a given location
func (sd *SchoolData) FindTwoNearestSchools(lat, lng float64) (NearestSchoolResult, NearestSchoolResult) {
	var first, second NearestSchoolResult
	first.DistanceKm = math.MaxFloat64
	second.DistanceKm = math.MaxFloat64

	for _, school := range sd.Schools {
		dist := Haversine(lat, lng, school.Latitude, school.Longitude)
		if dist < first.DistanceKm {
			// Current first becomes second
			second = first
			// New first
			first = NearestSchoolResult{
				Name:       school.Name,
				Type:       school.Type,
				Suburb:     school.Suburb,
				Latitude:   school.Latitude,
				Longitude:  school.Longitude,
				DistanceKm: dist,
			}
		} else if dist < second.DistanceKm {
			second = NearestSchoolResult{
				Name:       school.Name,
				Type:       school.Type,
				Suburb:     school.Suburb,
				Latitude:   school.Latitude,
				Longitude:  school.Longitude,
				DistanceKm: dist,
			}
		}
	}

	return first, second
}

// String returns school info as a string
func (s School) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Suburb)
}
