# AGENTS.md

## Project Overview

Farm Search is a web application for discovering rural and farm properties for sale in NSW, Australia. It displays properties on an interactive map with filtering capabilities based on price, property type, land size, and distance from key locations.

## Tech Stack

- **Backend**: Go 1.24+ with Chi router and sqlx for SQLite
- **Frontend**: Vanilla JavaScript with MapLibre GL JS
- **Database**: SQLite (using modernc.org/sqlite pure Go driver)
- **Build**: Make, Air (live reload)
- **Deployment**: systemd service behind Caddy reverse proxy

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
│   └── scraper/         # Property scrapers (FarmProperty, FarmBuy, REA), geocoder, browser
├── web/
│   ├── static/          # CSS, JS, and data files
│   └── templates/       # HTML templates
├── scripts/             # Shell scripts and SQL seeds
└── data/                # SQLite database (gitignored)
```

## Key Commands

```bash
make setup        # Install deps and seed sample data
make run          # Start server with live reload (air)
make build        # Build production binaries
make scrape       # Run property scraper
make seed         # Seed sample data
make deploy       # Build and deploy to production
make setup-server # Initial server setup (run once)

# Cadastral data
go run cmd/tools/main.go cadastral        # Fetch lots for properties missing data
go run cmd/tools/main.go cadastral -all   # Re-fetch lots for all properties
```

## Task Management

Feature development and tasks are tracked in `TODO.md`. Check this file for pending work and update it as features are completed.

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
- **Valhalla**: Isochrone generation (local Docker instance or public OSM server)
- **NSW Spatial Services**: Cadastral lot boundaries via ArcGIS REST API
- **FarmProperty.com.au**: Primary property listing source (no bot protection)
- **FarmBuy.com**: Secondary property listing source (implemented, no bot protection)
- **realestate.com.au**: Requires headless browser (Kasada bot protection)

## Isochrone Generation

Isochrones (drive-time polygons from Sutherland, NSW) are pre-generated and stored as GeoJSON files.

### Generate Isochrones

```bash
# Using public Valhalla (max 90 min)
go run cmd/tools/main.go isochrones

# Using local Valhalla (no time limit, supports up to 180 min)
go run cmd/tools/main.go isochrones -valhalla-url="http://localhost:8002"
```

Current intervals: 15, 30, 45, 60, 75, 90, 105, 120, 135, 150, 165, 180 minutes.

### Local Valhalla Setup (for >90 min isochrones)

```bash
# Install Docker (Ubuntu)
sudo apt-get install -y docker.io
sudo systemctl start docker

# Create data directory
mkdir -p ~/valhalla_tiles

# Start Valhalla with Australia OSM data (first run downloads ~1GB and builds tiles ~30 min)
sudo docker run -dt --name valhalla \
  -p 8002:8002 \
  -v ~/valhalla_tiles:/custom_files \
  -e tile_urls=https://download.geofabrik.de/australia-oceania/australia-latest.osm.pbf \
  ghcr.io/gis-ops/docker-valhalla/valhalla:latest

# Monitor tile build progress
sudo docker logs -f valhalla

# Once ready, update config for longer isochrones (default max is 120 min)
sudo python3 -c "
import json
with open('$HOME/valhalla_tiles/valhalla.json', 'r') as f:
    config = json.load(f)
config['service_limits']['isochrone']['max_time_contour'] = 240
with open('$HOME/valhalla_tiles/valhalla.json', 'w') as f:
    json.dump(config, f, indent=2)
"
sudo docker restart valhalla
```

### Managing Valhalla

```bash
sudo docker stop valhalla   # Stop (tiles preserved)
sudo docker start valhalla  # Start again
sudo docker rm valhalla     # Remove container (tiles persist in ~/valhalla_tiles)
```

## Deployment

**Production URL**: https://farms.dstrek.com

**Server**: 107.191.56.246 (Ubuntu 25.10)

### Initial Setup (run once)
```bash
make setup-server
```

This installs Caddy, creates the systemd service, and configures automatic HTTPS.

### Deploy Updates
```bash
make deploy
```

This builds for Linux, uploads binaries and static files, and restarts the service.

### Service Management
```bash
ssh root@107.191.56.246 'systemctl status farm-search'   # Check status
ssh root@107.191.56.246 'systemctl restart farm-search'  # Restart app
ssh root@107.191.56.246 'journalctl -u farm-search -f'   # View logs
```

### Crash Protection
The systemd service is configured to restart automatically:
- Restarts after 5 seconds on failure
- Up to 10 restarts within 5 minutes before stopping

## Important Notes

- Respect rate limits when scraping or geocoding
- Isochrone GeoJSON files stored in `web/static/data/isochrones/`
- Distance calculations use Haversine formula (straight-line, not driving)
- Local Valhalla tiles are stored in `~/valhalla_tiles/` (persists across container restarts)
