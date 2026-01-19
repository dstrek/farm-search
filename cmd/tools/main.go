package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"farm-search/internal/db"
	"farm-search/internal/geo"
)

func main() {
	// Sub-commands
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	os.Args = os.Args[1:] // Shift args for flag parsing

	switch cmd {
	case "isochrones":
		generateIsochrones()
	case "distances":
		calculateDistances()
	case "drivetimes":
		calculateDriveTimes()
	case "seed":
		seedSampleData()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: tools <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  isochrones   Generate Sydney drive-time isochrones")
	fmt.Println("  distances    Calculate property distances to towns, schools, Sydney")
	fmt.Println("  drivetimes   Calculate drive times to Sutherland for all properties")
	fmt.Println("  seed         Seed database with sample data")
}

func generateIsochrones() {
	outputDir := flag.String("output", "web/static/data/isochrones", "Output directory")
	valhallaURL := flag.String("valhalla-url", "", "Valhalla server URL (e.g., http://localhost:8002 for local, no time limit)")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	ctx := context.Background()

	var gen *geo.IsochroneGenerator
	if *valhallaURL != "" {
		log.Printf("Using Valhalla at %s (no time limit)", *valhallaURL)
		gen = geo.NewIsochroneGenerator(*valhallaURL)
	} else {
		log.Println("Using public Valhalla API (max 90 min)")
		gen = geo.NewIsochroneGenerator("")
	}

	intervals := []int{15, 30, 45, 60, 75, 90, 105, 120, 135, 150, 165, 180}

	log.Println("Generating Sutherland isochrones...")

	for _, mins := range intervals {
		log.Printf("Generating %d minute isochrone...", mins)

		iso, err := gen.GenerateIsochrone(ctx, geo.Sutherland.Lat, geo.Sutherland.Lng, mins)
		if err != nil {
			log.Printf("Failed to generate %d min isochrone: %v", mins, err)
			continue
		}

		filename := filepath.Join(*outputDir, fmt.Sprintf("sutherland_%d.geojson", mins))
		data, _ := json.MarshalIndent(iso, "", "  ")

		if err := os.WriteFile(filename, data, 0644); err != nil {
			log.Printf("Failed to write %s: %v", filename, err)
			continue
		}

		log.Printf("Saved %s", filename)

		// Rate limiting
		time.Sleep(1 * time.Second)
	}

	log.Println("Done!")
}

func calculateDistances() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Load schools
	schoolData := geo.NewSchoolData()
	if err := schoolData.LoadFromNSWData(ctx); err != nil {
		log.Printf("Warning: Could not load school data: %v", err)
	}
	log.Printf("Loaded %d schools", len(schoolData.Schools))

	// Get all properties
	properties, err := database.ListProperties(db.PropertyFilter{Limit: 10000})
	if err != nil {
		log.Fatalf("Failed to list properties: %v", err)
	}

	log.Printf("Calculating distances for %d properties...", len(properties))

	for _, p := range properties {
		// Distance to Sydney
		distSydney := geo.DistanceToSydney(p.Latitude, p.Longitude)

		// Nearest town
		town, distTown := geo.FindNearestTown(p.Latitude, p.Longitude)

		// Nearest school
		school, distSchool := schoolData.FindNearestSchool(p.Latitude, p.Longitude)

		// Save distances
		err := database.SavePropertyDistance(p.ID, "capital", "Sydney", distSydney)
		if err != nil {
			log.Printf("Failed to save Sydney distance for property %d: %v", p.ID, err)
		}

		err = database.SavePropertyDistance(p.ID, "town", town.Name, distTown)
		if err != nil {
			log.Printf("Failed to save town distance for property %d: %v", p.ID, err)
		}

		err = database.SavePropertyDistance(p.ID, "school", school.Name, distSchool)
		if err != nil {
			log.Printf("Failed to save school distance for property %d: %v", p.ID, err)
		}
	}

	log.Println("Distance calculations complete!")
}

func seedSampleData() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Read and execute the seed SQL file
	seedFile := "scripts/seed-sample-data.sql"
	content, err := os.ReadFile(seedFile)
	if err != nil {
		log.Fatalf("Failed to read seed file: %v", err)
	}

	_, err = database.Exec(string(content))
	if err != nil {
		log.Fatalf("Failed to execute seed SQL: %v", err)
	}

	count, _ := database.GetPropertyCount()
	log.Printf("Database seeded successfully! Total properties: %d", count)
}

func calculateDriveTimes() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	valhallaURL := flag.String("valhalla-url", "", "Valhalla server URL (e.g., http://localhost:8002 for local)")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Create router
	isLocal := *valhallaURL != ""
	var router *geo.Router
	if isLocal {
		log.Printf("Using local Valhalla at %s (no rate limiting)", *valhallaURL)
		router = geo.NewRouter(*valhallaURL)
	} else {
		log.Println("Using public Valhalla API (rate limited)")
		router = geo.NewRouter("")
	}

	// Get properties
	var properties []struct {
		ID        int64   `db:"id"`
		Latitude  float64 `db:"latitude"`
		Longitude float64 `db:"longitude"`
		Address   string  `db:"address"`
		Suburb    string  `db:"suburb"`
	}

	var query string
	if *all {
		query = `SELECT id, latitude, longitude, COALESCE(address, '') as address, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL`
	} else {
		query = `SELECT id, latitude, longitude, COALESCE(address, '') as address, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL AND drive_time_sydney IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need drive time calculation")
		return
	}

	log.Printf("Calculating drive times for %d properties to Sutherland...", len(properties))
	log.Printf("Sutherland coordinates: %.4f, %.4f", geo.Sutherland.Lat, geo.Sutherland.Lng)

	success := 0
	failed := 0

	for i, p := range properties {
		// Get drive time
		result, err := router.GetDriveTime(ctx, p.Latitude, p.Longitude)
		if err != nil {
			log.Printf("[%d/%d] Failed for property %d (%s, %s): %v",
				i+1, len(properties), p.ID, p.Address, p.Suburb, err)
			failed++
			if !isLocal {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		// Round to nearest minute
		driveTimeMins := int(result.DurationMins + 0.5)

		// Save to database immediately
		err = database.UpdatePropertyDriveTime(p.ID, driveTimeMins)
		if err != nil {
			log.Printf("[%d/%d] Failed to save drive time for property %d: %v",
				i+1, len(properties), p.ID, err)
			failed++
			continue
		}

		location := p.Suburb
		if p.Address != "" {
			location = p.Address
		}
		log.Printf("[%d/%d] Property %d (%s): %d mins (%.1f km)",
			i+1, len(properties), p.ID, location, driveTimeMins, result.DistanceKm)

		success++

		// Rate limiting only for public API
		if !isLocal {
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("Done! Success: %d, Failed: %d", success, failed)
}
