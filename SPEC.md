# Farm Search - Application Specification

## Overview

Farm Search is a web application for discovering and filtering rural, farm, and acreage properties for sale in New South Wales, Australia. The application displays properties on an interactive map with a sidebar for applying filters. It includes drive-time isochrone overlays to visualize how far you can travel from Sydney within a given time.

## Goals

1. Display NSW rural/farm properties on an interactive map
2. Allow filtering by price, property type, land size, and distance from key locations
3. Show drive-time isochrones from Sydney (15-minute increments up to 2 hours)
4. Aggregate listings from multiple real estate sources (REA, Domain, FarmBuy)
5. Display property boundary data from NSW cadastral sources (future)

## Tech Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Backend | Go | Fast, simple deployment, good concurrency |
| Router | Chi | Lightweight, idiomatic Go router |
| Database | SQLite + sqlx | Simple, no external dependencies, good for moderate scale |
| Frontend | Vanilla JS/CSS | No build step, simple maintenance |
| Map | MapLibre GL JS | Open source, vector tiles, good performance |
| Base Map | OpenStreetMap | Free, no API key required |

## Architecture

### Backend Components

```
cmd/
├── server/main.go      # HTTP server, serves API + static files
├── scraper/main.go     # CLI tool for scraping property listings
└── tools/main.go       # Utility CLI (seed, isochrones, distances)

internal/
├── api/
│   ├── routes.go       # Chi router configuration
│   ├── handlers.go     # HTTP request handlers
│   └── middleware.go   # Logging, CORS middleware
├── db/
│   ├── db.go           # Database connection, migrations
│   ├── properties.go   # Property CRUD operations
│   └── schema.sql      # Table definitions
├── models/
│   └── property.go     # Domain types (Property, Town, School, etc.)
├── geo/
│   ├── distance.go     # Haversine distance calculations
│   ├── isochrone.go    # Valhalla isochrone API client
│   └── schools.go      # NSW schools data loader
└── scraper/
    ├── scraper.go      # Scraper orchestration
    ├── rea.go          # realestate.com.au scraper
    └── geocoder.go     # Nominatim geocoding client
```

### Frontend Components

```
web/
├── templates/
│   └── index.html      # Main HTML page
└── static/
    ├── css/
    │   └── style.css   # All styles
    ├── js/
    │   ├── api.js      # REST API client
    │   ├── map.js      # MapLibre map setup and markers
    │   ├── filters.js  # Filter sidebar logic
    │   └── app.js      # Main application entry point
    └── data/
        └── isochrones/ # Pre-generated GeoJSON files
```

## Database Schema

### properties

Primary table for property listings.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| external_id | TEXT | Unique ID from source (e.g., REA listing ID) |
| source | TEXT | Origin: 'rea', 'domain', 'farmbuy', 'sample' |
| url | TEXT | Link to original listing |
| address | TEXT | Street address |
| suburb | TEXT | Suburb/town name |
| state | TEXT | State (default 'NSW') |
| postcode | TEXT | 4-digit postcode |
| latitude | REAL | GPS latitude |
| longitude | REAL | GPS longitude |
| price_min | INTEGER | Minimum price in cents |
| price_max | INTEGER | Maximum price in cents |
| price_text | TEXT | Display price ("$500k - $600k", "Contact Agent") |
| property_type | TEXT | 'house', 'land', 'farm', 'rural', 'acreage-semi-rural' |
| bedrooms | INTEGER | Number of bedrooms |
| bathrooms | INTEGER | Number of bathrooms |
| land_size_sqm | REAL | Land size in square meters |
| description | TEXT | Property description |
| images | TEXT | JSON array of image URLs |
| listed_at | DATETIME | When listing was first seen |
| scraped_at | DATETIME | When listing was last scraped |
| updated_at | DATETIME | When record was last updated |

**Indexes**: coords, price range, property type, source

### property_distances

Pre-computed distances for efficient filtering.

| Column | Type | Description |
|--------|------|-------------|
| property_id | INTEGER | FK to properties |
| target_type | TEXT | 'capital', 'town', 'school' |
| target_name | TEXT | e.g., 'Sydney', 'Bathurst', 'Dubbo High School' |
| distance_km | REAL | Straight-line distance in km |
| drive_time_mins | INTEGER | Driving time (optional) |

**Primary Key**: (property_id, target_type, target_name)

### towns

Reference table for NSW towns (population > 5,000).

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Town name |
| state | TEXT | State |
| latitude | REAL | GPS latitude |
| longitude | REAL | GPS longitude |
| population | INTEGER | Population count |

### schools

Reference table for NSW schools.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | School name |
| school_type | TEXT | 'Primary', 'Secondary', 'Combined' |
| suburb | TEXT | Suburb |
| state | TEXT | State |
| latitude | REAL | GPS latitude |
| longitude | REAL | GPS longitude |

## API Endpoints

### GET /api/properties

List properties with optional filters.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| price_min | int | Minimum price |
| price_max | int | Maximum price |
| type | string | Comma-separated property types |
| land_size_min | float | Minimum land size (sqm) |
| land_size_max | float | Maximum land size (sqm) |
| distance_sydney_max | float | Max distance from Sydney (km) |
| distance_town_max | float | Max distance from nearest town (km) |
| distance_school_max | float | Max distance from nearest school (km) |
| drive_time_sydney_max | int | Max drive time from Sydney (minutes) |
| bounds | string | Map viewport: "sw_lat,sw_lng,ne_lat,ne_lng" |
| limit | int | Max results (default 100, max 500) |
| offset | int | Pagination offset |

**Response:**
```json
{
  "properties": [
    {
      "id": 1,
      "lat": -33.8688,
      "lng": 151.2093,
      "price_text": "$500,000",
      "property_type": "rural",
      "address": "123 Example Rd",
      "suburb": "Somewhere"
    }
  ],
  "count": 1
}
```

### GET /api/properties/:id

Get full property details.

**Response:**
```json
{
  "id": 1,
  "external_id": "12345",
  "source": "rea",
  "url": "https://realestate.com.au/...",
  "address": "123 Example Rd",
  "suburb": "Somewhere",
  "state": "NSW",
  "postcode": "2000",
  "lat": -33.8688,
  "lng": 151.2093,
  "price_min": 500000,
  "price_max": 550000,
  "price_text": "$500,000 - $550,000",
  "property_type": "rural",
  "bedrooms": 3,
  "bathrooms": 2,
  "land_size_sqm": 40000,
  "description": "Beautiful property...",
  "images": ["https://..."]
}
```

### GET /api/filters/options

Get available filter values.

**Response:**
```json
{
  "property_types": ["farm", "rural", "acreage-semi-rural"],
  "price_min": 100000,
  "price_max": 5000000,
  "land_size_min": 1000,
  "land_size_max": 10000000
}
```

### POST /api/scrape/trigger

Manually trigger a scrape job.

**Response:**
```json
{
  "status": "queued",
  "message": "Scrape job has been queued"
}
```

## Frontend Features

### Map Display

- **Library**: MapLibre GL JS
- **Tiles**: OpenStreetMap raster tiles
- **Center**: NSW (-32.5, 147.0)
- **Zoom**: 5.5 (shows all of NSW)
- **Markers**: Blue circles for each property
- **Popups**: Property summary on marker click
- **Isochrone Layer**: Semi-transparent polygon overlay

### Filter Sidebar

| Filter | Control | Behavior |
|--------|---------|----------|
| Price Range | Two number inputs | Min/max price |
| Property Type | Checkboxes | Multiple selection |
| Land Size | Two number inputs | Min/max sqm |
| Distance from Sydney | Range slider | 0-500km |
| Distance from Town | Range slider | 0-100km |
| Distance from School | Range slider | 0-50km |
| Drive Time from Sydney | Dropdown | 15-120 min (15-min increments) |

### Property Modal

Displays full property details when "View Details" is clicked:
- Address and suburb
- Price
- Property type, beds, baths, land size
- Image gallery (horizontal scroll)
- Description
- Link to original listing

## Data Sources

### Property Listings

| Source | URL | Priority |
|--------|-----|----------|
| realestate.com.au | realestate.com.au/buy/property-rural-in-nsw | Primary |
| Domain | domain.com.au | Future |
| FarmBuy | farmbuy.com | Future |

**Scraping Approach:**
1. Search listing pages by property type and region
2. Extract listing IDs and basic info from search results
3. Optionally fetch full listing pages for additional details
4. Geocode addresses without coordinates using Nominatim
5. Store in SQLite with upsert logic

**Rate Limiting:**
- 2 second delay between page requests
- 1 second delay between geocoding requests
- Respect robots.txt

### Isochrones

| Source | API | Usage |
|--------|-----|-------|
| Valhalla | valhalla1.openstreetmap.de | Driving time polygons |

**Pre-generated Files:**
- `sydney_15.geojson` through `sydney_120.geojson`
- 15-minute increments
- Stored in `web/static/data/isochrones/`

### Geographic Reference Data

| Data | Source | Format |
|------|--------|--------|
| NSW Towns | Embedded in code | Go slice of Location structs |
| NSW Schools | NSW Education Data Hub | CSV (cached locally) |
| Cadastral (future) | NSW Spatial Services | WFS/WMS |

## Configuration

### Environment Variables (Future)

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| DB_PATH | data/farm-search.db | SQLite database path |
| SCRAPE_DELAY | 2s | Delay between scrape requests |

### Build Commands

```makefile
make setup      # Install deps, create DB, seed sample data
make run        # Start dev server with live reload
make build      # Build production binaries
make scrape     # Run property scraper
make seed       # Seed sample data
make isochrones # Generate isochrone GeoJSON files
make distances  # Pre-compute property distances
make clean      # Remove build artifacts
```

## Future Enhancements

### Phase 2: Additional Data Sources
- Domain.com.au scraper
- FarmBuy.com scraper
- Automated daily scraping via cron

### Phase 3: Cadastral Data
- NSW DCDB property boundary integration
- Display lot boundaries on property selection
- Lot/DP number display

### Phase 4: Enhanced Filters
- School type filter (primary/secondary)
- Water features (dam, river frontage)
- Road access type
- Zoning information

### Phase 5: User Features
- Save favorite properties
- Email alerts for new listings
- Property comparison view

## Performance Considerations

1. **Database**: SQLite handles ~100k properties comfortably; consider PostgreSQL for larger scale
2. **Map Rendering**: Limit to 500 markers; implement clustering for larger datasets
3. **Isochrones**: Pre-generate and cache; don't compute on-demand
4. **Distances**: Pre-compute and store in property_distances table
5. **Images**: Use original listing image URLs; don't proxy or cache

## Security Notes

1. No authentication required for MVP
2. Scraper respects rate limits to avoid IP blocking
3. No user data stored
4. CORS enabled for development; restrict in production
5. Don't expose database file publicly
