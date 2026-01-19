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
// URL: https://data.cese.nsw.gov.au/data/dataset/nsw-public-schools-master-dataset
func (sd *SchoolData) LoadFromNSWData(ctx context.Context) error {
	// NSW Government open data URL for school locations
	// This is a simplified version - in production you'd want to cache this data
	url := "https://data.cese.nsw.gov.au/data/dataset/027493b2-33ad-3f5b-8ed9-37cdca2b8571/resource/2ac19870-44f6-443d-a0c3-4c867f04c305/download/master_dataset.csv"

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		// If we can't fetch, use embedded sample data
		sd.loadSampleSchools()
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sd.loadSampleSchools()
		return nil
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
		col = strings.ToLower(strings.TrimSpace(col))
		switch {
		case strings.Contains(col, "school_name") || col == "school name":
			nameIdx = i
		case strings.Contains(col, "school_type") || col == "level of schooling":
			typeIdx = i
		case strings.Contains(col, "suburb") || col == "town_suburb":
			suburbIdx = i
		case col == "latitude" || col == "lat":
			latIdx = i
		case col == "longitude" || col == "long" || col == "lng":
			lngIdx = i
		}
	}

	if nameIdx == -1 || latIdx == -1 || lngIdx == -1 {
		sd.loadSampleSchools()
		return nil
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

		sd.Schools = append(sd.Schools, school)
	}

	return nil
}

// loadSampleSchools loads a sample set of NSW schools
func (sd *SchoolData) loadSampleSchools() {
	sd.Schools = []School{
		{Name: "Sydney Boys High School", Type: "Secondary", Suburb: "Surry Hills", Latitude: -33.8847, Longitude: 151.2136},
		{Name: "Sydney Girls High School", Type: "Secondary", Suburb: "Surry Hills", Latitude: -33.8852, Longitude: 151.2139},
		{Name: "Fort Street High School", Type: "Secondary", Suburb: "Petersham", Latitude: -33.8933, Longitude: 151.1547},
		{Name: "North Sydney Boys High School", Type: "Secondary", Suburb: "Crows Nest", Latitude: -33.8267, Longitude: 151.2033},
		{Name: "North Sydney Girls High School", Type: "Secondary", Suburb: "Crows Nest", Latitude: -33.8272, Longitude: 151.2028},
		{Name: "James Ruse Agricultural High School", Type: "Secondary", Suburb: "Carlingford", Latitude: -33.7833, Longitude: 151.0500},
		{Name: "Normanhurst Boys High School", Type: "Secondary", Suburb: "Normanhurst", Latitude: -33.7167, Longitude: 151.0833},
		{Name: "Hornsby Girls High School", Type: "Secondary", Suburb: "Hornsby", Latitude: -33.7050, Longitude: 151.0983},
		{Name: "Gosford High School", Type: "Secondary", Suburb: "Gosford", Latitude: -33.4300, Longitude: 151.3417},
		{Name: "Newcastle High School", Type: "Secondary", Suburb: "Newcastle", Latitude: -32.9283, Longitude: 151.7817},
		{Name: "Wollongong High School", Type: "Secondary", Suburb: "Wollongong", Latitude: -34.4278, Longitude: 150.8931},
		{Name: "Wagga Wagga High School", Type: "Secondary", Suburb: "Wagga Wagga", Latitude: -35.1082, Longitude: 147.3598},
		{Name: "Dubbo College", Type: "Secondary", Suburb: "Dubbo", Latitude: -32.2500, Longitude: 148.6167},
		{Name: "Orange High School", Type: "Secondary", Suburb: "Orange", Latitude: -33.2833, Longitude: 149.1000},
		{Name: "Bathurst High School", Type: "Secondary", Suburb: "Bathurst", Latitude: -33.4167, Longitude: 149.5833},
		{Name: "Armidale High School", Type: "Secondary", Suburb: "Armidale", Latitude: -30.5167, Longitude: 151.6667},
		{Name: "Tamworth High School", Type: "Secondary", Suburb: "Tamworth", Latitude: -31.0833, Longitude: 150.9167},
		{Name: "Coffs Harbour High School", Type: "Secondary", Suburb: "Coffs Harbour", Latitude: -30.3000, Longitude: 153.1333},
		{Name: "Lismore High School", Type: "Secondary", Suburb: "Lismore", Latitude: -28.8167, Longitude: 153.2833},
		{Name: "Grafton High School", Type: "Secondary", Suburb: "Grafton", Latitude: -29.6833, Longitude: 152.9333},
		{Name: "Broken Hill High School", Type: "Secondary", Suburb: "Broken Hill", Latitude: -31.9500, Longitude: 141.4667},
		{Name: "Griffith High School", Type: "Secondary", Suburb: "Griffith", Latitude: -34.2833, Longitude: 146.0333},
		{Name: "Albury High School", Type: "Secondary", Suburb: "Albury", Latitude: -36.0737, Longitude: 146.9135},
		{Name: "Goulburn High School", Type: "Secondary", Suburb: "Goulburn", Latitude: -34.7500, Longitude: 149.7167},
		{Name: "Nowra High School", Type: "Secondary", Suburb: "Nowra", Latitude: -34.8833, Longitude: 150.6000},
	}
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

// String returns school info as a string
func (s School) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Suburb)
}
