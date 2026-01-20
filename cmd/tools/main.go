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

// Default Valhalla URL - local instance in this container
const defaultValhallaURL = "http://localhost:8002"

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
	case "towns":
		calculateNearestTowns()
	case "towndrivetimes":
		calculateTownDriveTimes()
	case "schools":
		calculateNearestSchools()
	case "schooldrivetimes":
		calculateSchoolDriveTimes()
	case "cadastral":
		fetchCadastralLots()
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
	fmt.Println("  isochrones        Generate Sydney drive-time isochrones")
	fmt.Println("  distances         Calculate property distances to towns, schools, Sydney")
	fmt.Println("  drivetimes        Calculate drive times to Sutherland for all properties")
	fmt.Println("  towns             Calculate nearest towns for all properties")
	fmt.Println("  towndrivetimes    Calculate drive times to nearest towns for all properties")
	fmt.Println("  schools           Calculate nearest schools for all properties")
	fmt.Println("  schooldrivetimes  Calculate drive times to nearest schools for all properties")
	fmt.Println("  cadastral         Fetch cadastral lot boundaries for properties")
	fmt.Println("  seed              Seed database with sample data")
}

func generateIsochrones() {
	outputDir := flag.String("output", "web/static/data/isochrones", "Output directory")
	valhallaURL := flag.String("valhalla-url", defaultValhallaURL, "Valhalla server URL")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	ctx := context.Background()

	log.Printf("Using Valhalla at %s", *valhallaURL)
	gen := geo.NewIsochroneGenerator(*valhallaURL)

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

func calculateTownDriveTimes() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	valhallaURL := flag.String("valhalla-url", defaultValhallaURL, "Valhalla server URL")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Create router
	log.Printf("Using Valhalla at %s", *valhallaURL)
	router := geo.NewRouter(*valhallaURL)

	// Build a map of town name -> coordinates for quick lookup
	townCoords := make(map[string]geo.Location)
	for _, town := range geo.NSWTowns {
		townCoords[town.Name] = town
	}

	// Get properties that need drive times calculated
	var properties []struct {
		ID           int64   `db:"id"`
		Latitude     float64 `db:"latitude"`
		Longitude    float64 `db:"longitude"`
		Suburb       string  `db:"suburb"`
		NearestTown1 string  `db:"nearest_town_1"`
		NearestTown2 string  `db:"nearest_town_2"`
	}

	var query string
	if *all {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb, 
				 nearest_town_1, COALESCE(nearest_town_2, '') as nearest_town_2
				 FROM properties 
				 WHERE latitude IS NOT NULL AND longitude IS NOT NULL 
				 AND nearest_town_1 IS NOT NULL`
	} else {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb,
				 nearest_town_1, COALESCE(nearest_town_2, '') as nearest_town_2
				 FROM properties 
				 WHERE latitude IS NOT NULL AND longitude IS NOT NULL 
				 AND nearest_town_1 IS NOT NULL AND nearest_town_1_mins IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need town drive time calculation")
		return
	}

	log.Printf("Calculating drive times to nearest towns for %d properties...", len(properties))

	success := 0
	failed := 0

	for i, p := range properties {
		var town1Mins, town2Mins *int

		// Get drive time to nearest town 1
		if town1, ok := townCoords[p.NearestTown1]; ok {
			result, err := router.GetRoute(ctx, p.Latitude, p.Longitude, town1.Latitude, town1.Longitude)
			if err != nil {
				log.Printf("[%d/%d] Failed route to %s for property %d: %v",
					i+1, len(properties), p.NearestTown1, p.ID, err)
				failed++
				continue
			}
			mins := int(result.DurationMins + 0.5)
			town1Mins = &mins
		}

		// Get drive time to nearest town 2 (if exists)
		if p.NearestTown2 != "" {
			if town2, ok := townCoords[p.NearestTown2]; ok {
				result, err := router.GetRoute(ctx, p.Latitude, p.Longitude, town2.Latitude, town2.Longitude)
				if err != nil {
					log.Printf("[%d/%d] Failed route to %s for property %d: %v",
						i+1, len(properties), p.NearestTown2, p.ID, err)
					// Continue anyway, we at least have town 1
				} else {
					mins := int(result.DurationMins + 0.5)
					town2Mins = &mins
				}
			}
		}

		// Save to database
		_, err = database.Exec(`
			UPDATE properties 
			SET nearest_town_1_mins = ?, nearest_town_2_mins = ?
			WHERE id = ?`,
			town1Mins, town2Mins, p.ID)

		if err != nil {
			log.Printf("[%d/%d] Failed to save for property %d: %v", i+1, len(properties), p.ID, err)
			failed++
			continue
		}

		town1Str := "N/A"
		if town1Mins != nil {
			town1Str = fmt.Sprintf("%d min", *town1Mins)
		}
		town2Str := "N/A"
		if town2Mins != nil {
			town2Str = fmt.Sprintf("%d min", *town2Mins)
		}

		log.Printf("[%d/%d] Property %d (%s): %s (%s), %s (%s)",
			i+1, len(properties), p.ID, p.Suburb,
			p.NearestTown1, town1Str, p.NearestTown2, town2Str)

		success++
	}

	log.Printf("Done! Success: %d, Failed: %d", success, failed)
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
	valhallaURL := flag.String("valhalla-url", defaultValhallaURL, "Valhalla server URL")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Create router
	log.Printf("Using Valhalla at %s", *valhallaURL)
	router := geo.NewRouter(*valhallaURL)

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
	}

	log.Printf("Done! Success: %d, Failed: %d", success, failed)
}

func calculateNearestTowns() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Get properties
	var properties []struct {
		ID        int64   `db:"id"`
		Latitude  float64 `db:"latitude"`
		Longitude float64 `db:"longitude"`
		Suburb    string  `db:"suburb"`
	}

	var query string
	if *all {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL`
	} else {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL AND nearest_town_1 IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need nearest town calculation")
		return
	}

	log.Printf("Calculating nearest towns for %d properties using %d towns...", len(properties), len(geo.NSWTowns))

	for i, p := range properties {
		// Find two nearest towns
		town1, town2 := geo.FindTwoNearestTowns(p.Latitude, p.Longitude)

		// Save to database
		_, err = database.Exec(`
			UPDATE properties 
			SET nearest_town_1 = ?, nearest_town_1_km = ?, 
			    nearest_town_2 = ?, nearest_town_2_km = ?
			WHERE id = ?`,
			town1.Name, town1.DistanceKm, town2.Name, town2.DistanceKm, p.ID)

		if err != nil {
			log.Printf("[%d/%d] Failed to save for property %d: %v", i+1, len(properties), p.ID, err)
			continue
		}

		log.Printf("[%d/%d] Property %d (%s): %s (%.1f km), %s (%.1f km)",
			i+1, len(properties), p.ID, p.Suburb,
			town1.Name, town1.DistanceKm, town2.Name, town2.DistanceKm)
	}

	log.Println("Done!")
}

func calculateNearestSchools() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
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

	// Get properties
	var properties []struct {
		ID        int64   `db:"id"`
		Latitude  float64 `db:"latitude"`
		Longitude float64 `db:"longitude"`
		Suburb    string  `db:"suburb"`
	}

	var query string
	if *all {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL`
	} else {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb 
				 FROM properties WHERE latitude IS NOT NULL AND longitude IS NOT NULL AND nearest_school_1 IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need nearest school calculation")
		return
	}

	log.Printf("Calculating nearest schools for %d properties...", len(properties))

	for i, p := range properties {
		// Find two nearest schools
		school1, school2 := schoolData.FindTwoNearestSchools(p.Latitude, p.Longitude)

		// Save to database
		_, err = database.Exec(`
			UPDATE properties 
			SET nearest_school_1 = ?, nearest_school_1_km = ?, 
			    nearest_school_2 = ?, nearest_school_2_km = ?
			WHERE id = ?`,
			school1.Name, school1.DistanceKm, school2.Name, school2.DistanceKm, p.ID)

		if err != nil {
			log.Printf("[%d/%d] Failed to save for property %d: %v", i+1, len(properties), p.ID, err)
			continue
		}

		log.Printf("[%d/%d] Property %d (%s): %s (%.1f km), %s (%.1f km)",
			i+1, len(properties), p.ID, p.Suburb,
			school1.Name, school1.DistanceKm, school2.Name, school2.DistanceKm)
	}

	log.Println("Done!")
}

func calculateSchoolDriveTimes() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	valhallaURL := flag.String("valhalla-url", defaultValhallaURL, "Valhalla server URL")
	all := flag.Bool("all", false, "Recalculate all properties, not just missing ones")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Load schools to get coordinates
	schoolData := geo.NewSchoolData()
	if err := schoolData.LoadFromNSWData(ctx); err != nil {
		log.Printf("Warning: Could not load school data: %v", err)
	}
	log.Printf("Loaded %d schools", len(schoolData.Schools))

	// Build a map of school name -> coordinates for quick lookup
	schoolCoords := make(map[string]geo.School)
	for _, school := range schoolData.Schools {
		schoolCoords[school.Name] = school
	}

	// Create router
	log.Printf("Using Valhalla at %s", *valhallaURL)
	router := geo.NewRouter(*valhallaURL)

	// Get properties that need drive times calculated
	var properties []struct {
		ID             int64   `db:"id"`
		Latitude       float64 `db:"latitude"`
		Longitude      float64 `db:"longitude"`
		Suburb         string  `db:"suburb"`
		NearestSchool1 string  `db:"nearest_school_1"`
		NearestSchool2 string  `db:"nearest_school_2"`
	}

	var query string
	if *all {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb, 
				 nearest_school_1, COALESCE(nearest_school_2, '') as nearest_school_2
				 FROM properties 
				 WHERE latitude IS NOT NULL AND longitude IS NOT NULL 
				 AND nearest_school_1 IS NOT NULL`
	} else {
		query = `SELECT id, latitude, longitude, COALESCE(suburb, '') as suburb,
				 nearest_school_1, COALESCE(nearest_school_2, '') as nearest_school_2
				 FROM properties 
				 WHERE latitude IS NOT NULL AND longitude IS NOT NULL 
				 AND nearest_school_1 IS NOT NULL AND nearest_school_1_mins IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need school drive time calculation")
		return
	}

	log.Printf("Calculating drive times to nearest schools for %d properties...", len(properties))

	success := 0
	failed := 0

	for i, p := range properties {
		var school1Mins, school2Mins *int

		// Get drive time to nearest school 1
		if school1, ok := schoolCoords[p.NearestSchool1]; ok {
			result, err := router.GetRoute(ctx, p.Latitude, p.Longitude, school1.Latitude, school1.Longitude)
			if err != nil {
				log.Printf("[%d/%d] Failed route to %s for property %d: %v",
					i+1, len(properties), p.NearestSchool1, p.ID, err)
				failed++
				continue
			}
			mins := int(result.DurationMins + 0.5)
			school1Mins = &mins
		}

		// Get drive time to nearest school 2 (if exists)
		if p.NearestSchool2 != "" {
			if school2, ok := schoolCoords[p.NearestSchool2]; ok {
				result, err := router.GetRoute(ctx, p.Latitude, p.Longitude, school2.Latitude, school2.Longitude)
				if err != nil {
					log.Printf("[%d/%d] Failed route to %s for property %d: %v",
						i+1, len(properties), p.NearestSchool2, p.ID, err)
					// Continue anyway, we at least have school 1
				} else {
					mins := int(result.DurationMins + 0.5)
					school2Mins = &mins
				}
			}
		}

		// Save to database
		_, err = database.Exec(`
			UPDATE properties 
			SET nearest_school_1_mins = ?, nearest_school_2_mins = ?
			WHERE id = ?`,
			school1Mins, school2Mins, p.ID)

		if err != nil {
			log.Printf("[%d/%d] Failed to save for property %d: %v", i+1, len(properties), p.ID, err)
			failed++
			continue
		}

		school1Str := "N/A"
		if school1Mins != nil {
			school1Str = fmt.Sprintf("%d min", *school1Mins)
		}
		school2Str := "N/A"
		if school2Mins != nil {
			school2Str = fmt.Sprintf("%d min", *school2Mins)
		}

		log.Printf("[%d/%d] Property %d (%s): %s (%s), %s (%s)",
			i+1, len(properties), p.ID, p.Suburb,
			p.NearestSchool1, school1Str, p.NearestSchool2, school2Str)

		success++
	}

	log.Printf("Done! Success: %d, Failed: %d", success, failed)
}

func fetchCadastralLots() {
	dbPath := flag.String("db", "data/farm-search.db", "Database path")
	all := flag.Bool("all", false, "Fetch lots for all properties, not just those without lots")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// Create cadastral client
	client := geo.NewCadastralClient()

	// Get properties that need cadastral lots
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
		query = `SELECT p.id, p.latitude, p.longitude, COALESCE(p.address, '') as address, COALESCE(p.suburb, '') as suburb 
				 FROM properties p
				 LEFT JOIN property_lots pl ON p.id = pl.property_id
				 WHERE p.latitude IS NOT NULL AND p.longitude IS NOT NULL 
				 AND pl.property_id IS NULL`
	}

	err = database.Select(&properties, query)
	if err != nil {
		log.Fatalf("Failed to get properties: %v", err)
	}

	if len(properties) == 0 {
		log.Println("No properties need cadastral lot lookup")
		return
	}

	log.Printf("Fetching cadastral lots for %d properties...", len(properties))

	success := 0
	failed := 0
	lotsFound := 0

	for i, p := range properties {
		// Fetch lots at the property's coordinates
		lots, err := client.FetchLotsAtPoint(ctx, p.Longitude, p.Latitude)
		if err != nil {
			log.Printf("[%d/%d] Failed for property %d (%s): %v",
				i+1, len(properties), p.ID, p.Suburb, err)
			failed++
			time.Sleep(500 * time.Millisecond) // Rate limiting
			continue
		}

		if len(lots) == 0 {
			log.Printf("[%d/%d] Property %d (%s): No lots found",
				i+1, len(properties), p.ID, p.Suburb)
			failed++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Save each lot and link to property
		for _, lot := range lots {
			// Calculate centroid
			centroidLat, centroidLng, err := geo.CalculateLotCentroid(lot.Geometry)
			if err != nil {
				log.Printf("  Warning: Could not calculate centroid for lot %s: %v", lot.LotIDString, err)
				centroidLat, centroidLng = p.Latitude, p.Longitude // Use property coords as fallback
			}

			// Convert geometry to JSON string
			geomJSON, err := geo.LotGeometryToJSON(lot.Geometry)
			if err != nil {
				log.Printf("  Warning: Could not serialize geometry for lot %s: %v", lot.LotIDString, err)
				continue
			}

			// Save lot to database
			lotID, err := database.SaveCadastralLot(
				lot.LotIDString,
				lot.LotNumber,
				lot.PlanLabel,
				lot.AreaSqm,
				centroidLat,
				centroidLng,
				geomJSON,
			)
			if err != nil {
				log.Printf("  Warning: Could not save lot %s: %v", lot.LotIDString, err)
				continue
			}

			// Link property to lot
			err = database.LinkPropertyToLot(p.ID, lotID)
			if err != nil {
				log.Printf("  Warning: Could not link property %d to lot %d: %v", p.ID, lotID, err)
				continue
			}

			lotsFound++
		}

		location := p.Suburb
		if p.Address != "" {
			location = p.Address
		}
		log.Printf("[%d/%d] Property %d (%s): Found %d lots",
			i+1, len(properties), p.ID, location, len(lots))

		success++

		// Rate limiting to avoid overloading NSW Spatial Services
		time.Sleep(500 * time.Millisecond)
	}

	totalLots, _ := database.GetCadastralLotCount()
	log.Printf("Done! Properties: %d success, %d failed. Total lots in DB: %d", success, failed, totalLots)
}
