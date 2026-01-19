package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	fmt.Println("  isochrones  Generate Sydney drive-time isochrones")
	fmt.Println("  distances   Calculate property distances to towns, schools, Sydney")
	fmt.Println("  seed        Seed database with sample data")
}

func generateIsochrones() {
	outputDir := flag.String("output", "web/static/data/isochrones", "Output directory")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	ctx := context.Background()
	gen := geo.NewIsochroneGenerator("")

	intervals := []int{15, 30, 45, 60, 75, 90, 105, 120}

	log.Println("Generating Sydney isochrones...")

	for _, mins := range intervals {
		log.Printf("Generating %d minute isochrone...", mins)

		iso, err := gen.GenerateIsochrone(ctx, geo.SydneyCBD.Lat, geo.SydneyCBD.Lng, mins)
		if err != nil {
			log.Printf("Failed to generate %d min isochrone: %v", mins, err)
			continue
		}

		filename := filepath.Join(*outputDir, fmt.Sprintf("sydney_%d.geojson", mins))
		data, _ := json.MarshalIndent(iso, "", "  ")

		if err := os.WriteFile(filename, data, 0644); err != nil {
			log.Printf("Failed to write %s: %v", filename, err)
			continue
		}

		log.Printf("Saved %s", filename)
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
