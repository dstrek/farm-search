# Farm Search - Application Specification

## Overview

Farm Search is a web application for discovering and filtering rural, farm, and acreage properties for sale in New South Wales, Australia. The application displays properties on an interactive map with a sidebar for applying filters. It includes drive-time isochrone overlays to visualize how far you can travel from Sydney within a given time.

## Goals

1. Display NSW rural/farm properties on an interactive map
2. Allow filtering by price, property type, land size, and distance from key locations
3. Show drive-time isochrones from Sydney (15-minute increments up to 90 minutes)
4. Aggregate listings from multiple real estate sources (FarmProperty, FarmBuy, REA)
5. Display property boundary data from NSW cadastral sources

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
    ├── farmproperty.go # farmproperty.com.au scraper (primary)
    ├── farmbuy.go      # farmbuy.com scraper
    ├── rea.go          # realestate.com.au scraper
    ├── browser.go      # Headless Chrome browser for bot-protected sites
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

### property_links

Tracks duplicate properties across sources.

| Column | Type | Description |
|--------|------|-------------|
| canonical_id | INTEGER | FK to canonical property |
| duplicate_id | INTEGER | FK to duplicate property (PK) |
| match_type | TEXT | 'coords' or 'address' |
| created_at | DATETIME | When link was created |

### cadastral_lots

Stores cadastral lot boundaries from NSW Spatial Services.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| lot_id_string | TEXT | Unique lot identifier (e.g., "2//DP875844") |
| lot_number | TEXT | Lot number (e.g., "2") |
| plan_label | TEXT | Plan label (e.g., "DP875844") |
| area_sqm | REAL | Area in square meters |
| geometry | TEXT | GeoJSON Polygon geometry |
| centroid_lat | REAL | Centroid latitude |
| centroid_lng | REAL | Centroid longitude |
| fetched_at | DATETIME | When data was fetched |

### property_lots

Links properties to cadastral lots (many-to-many).

| Column | Type | Description |
|--------|------|-------------|
| property_id | INTEGER | FK to properties |
| lot_id | INTEGER | FK to cadastral_lots |

**Primary Key**: (property_id, lot_id)

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

### GET /api/boundaries

Get cadastral lot boundaries within map bounds.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| bounds | string | Map viewport: "sw_lat,sw_lng,ne_lat,ne_lng" (required) |
| zoom | float | Current zoom level (optional, enables buffer at zoom >= 14) |

**Response:**
```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[lng, lat], ...]]
      },
      "properties": {
        "lot_id": "2//DP875844",
        "lot_number": "2",
        "plan_label": "DP875844",
        "area_sqm": 513241.86
      }
    }
  ]
}
```

## Frontend Features

### Map Display

- **Library**: MapLibre GL JS
- **Base Tiles**: OpenStreetMap (streets) or Mapbox (satellite)
- **Default Center**: NSW (150.086, -34.048)
- **Default Zoom**: 7.72 (shows regional NSW)
- **Markers**: Colored circles for each property (color by source: orange=FarmProperty, green=FarmBuy, red=REA)
- **Property Sidebar**: Clicking a marker opens a right sidebar (380px) with full property details
- **Isochrone Layer**: Semi-transparent polygon overlay showing drive time from Sutherland
- **Boundary Layer**: Property cadastral boundaries (visible at zoom 12+)

**Viewport Persistence**: Map center and zoom level are saved to localStorage (`farm-search-viewport`) on every move (debounced 500ms) and restored on page load.

### Filter Sidebar

Width: 260px (collapsible on mobile)

| Filter | Control | Behavior |
|--------|---------|----------|
| Max Price | Range slider | Custom price steps ($100k-$10M) |
| Min Land Size | Range slider | 10-100 HA in 10 HA increments |
| Drive to Sutherland | Range slider | 15-255 min in 15-min increments |
| Drive to nearest town | Range slider | 5-60 min in 5-min increments |
| Distance from school | Range slider | 0-50 km |
| Map Style | Button group | Streets / Satellite toggle |
| Drive time area | Dropdown | Isochrone overlay (1-3 hours) |

**Persistence**: Filter state is saved to localStorage (`farm-search-filters`) and restored on page load. Schema versioning ensures invalid saved data is cleared automatically.

### Property Details Sidebar

Right sidebar (380px) that opens when clicking a map marker:
- Address and suburb
- Price
- Property type, beds, baths, land size
- Drive time to Sutherland
- Nearest towns with drive times
- Image gallery (horizontal scroll, up to 5 images)
- Description
- Link to original listing (shows multiple sources if property listed on multiple sites)
- Close via X button or Escape key

## Data Sources

### Property Listings

| Source | URL | Status |
|--------|-----|--------|
| FarmProperty | farmproperty.com.au | Implemented (primary, no bot protection) |
| FarmBuy | farmbuy.com | Implemented (no bot protection) |
| realestate.com.au | realestate.com.au/buy/property-rural-in-nsw | Implemented (requires headless browser) |
| Domain | domain.com.au | Future |

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
- `sydney_15.geojson` through `sydney_90.geojson`
- 15-minute increments (15, 30, 45, 60, 75, 90)
- Stored in `web/static/data/isochrones/`

### Geographic Reference Data

| Data | Source | Format |
|------|--------|--------|
| NSW Towns | Embedded in code | Go slice of Location structs |
| NSW Schools | NSW Education Data Hub | CSV (cached locally) |
| Cadastral | NSW Spatial Services | ArcGIS REST API |

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
- Automated daily scraping via cron

### Phase 3: Cadastral Data (Implemented)
- NSW DCDB property boundary integration via ArcGIS REST API
- Display lot boundaries when zoomed in (zoom 12+)
- Lot/DP number stored with properties

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
