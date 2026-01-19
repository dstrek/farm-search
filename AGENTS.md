# AGENTS.md

## Project Overview

Farm Search is a web application for discovering rural and farm properties for sale in NSW, Australia. It displays properties on an interactive map with filtering capabilities based on price, property type, land size, and distance from key locations.

## Tech Stack

- **Backend**: Go 1.24+ with Chi router and sqlx for SQLite
- **Frontend**: Vanilla JavaScript with MapLibre GL JS
- **Database**: SQLite
- **Build**: Make, Air (live reload)

## Project Structure

```
farm-search/
├── cmd/
│   ├── server/          # Web server entry point
│   ├── scraper/         # Property scraper CLI
│   └── tools/           # Utility commands (seed, isochrones, distances)
├── internal/
│   ├── api/             # HTTP handlers, routes, middleware
│   ├── db/              # Database connection, queries, schema
│   ├── geo/             # Geographic calculations, isochrones, schools data
│   ├── models/          # Domain types
│   └── scraper/         # REA scraper, geocoder
├── web/
│   ├── static/          # CSS, JS, and data files
│   └── templates/       # HTML templates
├── scripts/             # Shell scripts and SQL seeds
└── data/                # SQLite database (gitignored)
```

## Key Commands

```bash
make setup      # Install deps and seed sample data
make run        # Start server with live reload (air)
make build      # Build production binaries
make scrape     # Run property scraper
make seed       # Seed sample data
```

## Development Guidelines

### Go Code

- Use Chi for routing, sqlx for database access
- Place HTTP handlers in `internal/api/handlers.go`
- Database queries go in `internal/db/properties.go`
- Domain models in `internal/models/`
- Use `sql.Null*` types for nullable database fields

### Frontend

- Vanilla JS only, no frameworks or build steps
- MapLibre GL JS for mapping
- Keep JS modular: `api.js`, `map.js`, `filters.js`, `app.js`
- CSS in single `style.css` file

### Database

- Schema defined in `internal/db/schema.sql`
- Migrations run automatically via `db.New()`
- Use `ON CONFLICT` for upserts

### Testing the API

```bash
curl http://localhost:8080/api/properties
curl http://localhost:8080/api/properties/1
curl http://localhost:8080/api/filters/options
```

## External Services

- **Nominatim**: Geocoding (free, rate-limited to 1 req/sec)
- **Valhalla**: Isochrone generation (public OSM instance)
- **realestate.com.au**: Property listing source (scraping)

## Important Notes

- Respect rate limits when scraping or geocoding
- Sample data uses `source = 'sample'` to distinguish from real listings
- Isochrone GeoJSON files stored in `web/static/data/isochrones/`
- Distance calculations use Haversine formula (straight-line, not driving)
