# TODO

## Completed

### Phase 1: Project Scaffolding
- [x] Initialize Go module with chi, sqlx, sqlite3 dependencies
- [x] Create folder structure (cmd/, internal/, web/, scripts/, data/)
- [x] Create SQLite database schema (properties, property_distances, towns, schools)
- [x] Set up Chi router with logging and CORS middleware
- [x] Create index.html with MapLibre map centered on NSW
- [x] Create CSS styles for sidebar and map layout
- [x] Create JS modules (api.js, map.js, filters.js, app.js)
- [x] Create server main.go entry point
- [x] Create Makefile with build commands

### Phase 2: REA Scraper
- [x] Create scraper CLI in cmd/scraper/
- [x] Implement REA listing page parser (HTML + embedded JSON)
- [x] Extract property details from listing URLs
- [x] Add Nominatim geocoder for addresses without coordinates
- [x] Create scraper orchestration with rate limiting

### Phase 3: API & Frontend
- [x] Implement GET /api/properties with filter query params
- [x] Implement GET /api/properties/:id for full details
- [x] Implement GET /api/filters/options for dynamic filter values
- [x] Implement POST /api/scrape/trigger placeholder
- [x] Create filter sidebar UI (price, type, land size, distances)
- [x] Create property markers on map
- [x] Create property popup on marker click
- [x] Create property detail modal

### Phase 4: Isochrones
- [x] Create Valhalla isochrone API client
- [x] Add isochrone generation script (scripts/generate-isochrones.sh)
- [x] Add isochrone layer to MapLibre map
- [x] Wire up drive time filter to show/hide isochrone layer

### Phase 5: Distance Calculations
- [x] Implement Haversine distance formula
- [x] Add NSW towns reference data (50+ towns, pop > 5000)
- [x] Add NSW schools reference data
- [x] Create distance calculation tool in cmd/tools/
- [x] Add SavePropertyDistance and GetPropertyDistances DB methods

### Infrastructure
- [x] Create sample data seed (15 NSW properties)
- [x] Create seed SQL script
- [x] Set up Air for live reload
- [x] Create .air.toml configuration
- [x] Create .gitignore
- [x] Write AGENTS.md
- [x] Write SPEC.md

### Deployment
- [x] Create server setup script (Caddy, systemd service)
- [x] Create deploy script (build, upload, restart)
- [x] Add Makefile deploy targets
- [x] Switch to pure Go SQLite driver (modernc.org/sqlite)
- [x] Deploy to production (farms.dstrek.com)
- [x] Configure automatic HTTPS via Let's Encrypt
- [x] Configure crash protection (systemd restart policy)

### Scraper Enhancements
- [x] Test scraper against live REA website (blocked by Kasada bot protection)
- [x] Add browser automation support (chromedp) for bot-protected sites
- [x] Add FarmProperty.com.au scraper (no bot protection, works great)
- [x] Add FarmBuy.com scraper (extracts embedded JSON + map coordinates)
- [x] Scrape real property data from FarmProperty.com.au and FarmBuy.com
- [x] Add .air.toml configuration for live reload
- [x] Add cross-source deduplication (detect same property on multiple sites)
- [x] Show multiple source links when property listed on multiple sites
- [x] Generate Sydney isochrone GeoJSON files (15-90 min via Valhalla API)
- [x] Calculate distances for all scraped properties (Sydney, nearest town, nearest school)
- [x] Add map marker clustering using MapLibre's native GeoJSON clustering
- [x] Add loading indicators for API calls (overlay spinner)
- [x] Improve REA scraper with advanced anti-detection techniques:
  - Comprehensive stealth mode flags for Chrome
  - Human-like behavior simulation (scrolling, random delays, mouse movement)
  - Cookie injection from JSON file (export from browser to bypass Kasada)
  - Multiple JSON extraction patterns (ArgonautExchange, Next.js, recursive search)
  - Enhanced HTML parsing fallback
  - Better detail page scraping

---

## Backlog

### High Priority
- [x] Pre-compute drive time to Sutherland for each property and show in popup
- [x] Replace "distance from Sydney" filter with "drive time to Sutherland" filter
- [x] Store a list of towns (211 NSW towns in internal/geo/distance.go)
- [x] Show distance to 2 nearest towns on each property (in modal)
- [x] Change "distance from town" filter to "drive time to nearest town" filter
- [x] Research NSW Planning Portal for spatial data to get property boundaries
- [x] Fetch and store cadastral lot boundaries from NSW Spatial Services
- [x] Render property boundaries on map after a certain zoom level

### UI Improvements
- [x] Reduce left sidebar width (320px → 260px)
- [x] Replace popup+modal with right property details sidebar
  - Added `<aside id="property-sidebar">` element
  - CSS: Right sidebar (380px), fixed position
  - Click marker opens sidebar directly with full details
  - Close via X button or Escape key
  - Removed popup and modal code
- [x] Property image gallery/carousel
  - Large primary image with 4:3 aspect ratio
  - Thumbnail strip for navigation
  - Prev/next buttons with keyboard support (arrow keys)
  - Shows all images (no limit)
  - Image counter showing position (e.g., "3 / 12")
- [x] Show travel route to nearest town on map
  - When property sidebar opens, draw route line to nearest town
  - Added GET /api/route endpoint that proxies Valhalla routing API
  - Added GeoJSON source/layer for route polyline
  - Styled as dashed rose-colored line
  - Route clears when property deselected or sidebar closed
- [x] Persist map viewport to localStorage
- [x] Fullscreen image modal
  - Click on main carousel image to open fullscreen modal
  - Modal carousel with prev/next navigation
  - Close via X button, Escape key, or clicking outside
  - Keyboard navigation (arrow keys)
- [x] Drive time to nearest primary school
  - Added schema columns: nearest_school_1/2, nearest_school_1/2_km, nearest_school_1/2_mins
  - Added `schools` and `schooldrivetimes` tool commands (make targets added)
  - Loads ~1600 NSW primary schools from data.nsw.gov.au
  - Replaced distance filter with drive time filter (5-60 min)
  - Display nearest schools in property sidebar (abbreviated "Public School" to "PS")
  - Note: Run `schooldrivetimes` with Valhalla to populate drive times
- [x] Make nearest towns/schools clickable to show route on map
  - Removed automatic route display when property sidebar opens
  - Click on a town name in property details to show route to that town
  - Click on a school name in property details to show route to that school
  - Added school coordinates to schema for routing support
  - Clickable items highlight when active (blue for towns, sky blue for schools)

### Medium Priority
- [ ] Add Domain.com.au scraper (may have bot protection)
- [ ] Implement scheduled daily scraping (cron)
- [ ] Improve mobile responsive layout

### Low Priority
- [ ] Add error handling UI (toast notifications)
- [ ] Add property image lazy loading
- [x] Cache filter options in localStorage (with schema versioning for auto-reset)

---

## Future Ideas

### Enhanced Filters
- [ ] Filter by water features (dam, creek, river frontage)
- [ ] Filter by zoning (rural, residential, mixed)
- [ ] Filter by listing age (new this week, etc.)

### Cadastral Integration
- [ ] Show Lot/DP number in property details
- [ ] Calculate actual land area from cadastral data

### User Features
- [ ] Save favorite properties (localStorage)
- [ ] Email alerts for new listings matching filters

### Data Enrichment
- [ ] Soil type data overlay
- [ ] Bushfire risk zones
- [ ] Flood zones
- [ ] Mobile coverage map
- [ ] Nearest hospital distance
- [ ] Climate/rainfall data

### Performance
- [ ] Add API response compression

### DevOps
- [x] Create production deployment scripts
- [x] Set up systemd service with crash protection
- [x] Configure Caddy reverse proxy with auto-HTTPS

---

## Notes

### Scraping Observations
- **REA (realestate.com.au)**: Uses Kasada bot protection, bypassed via ScrapingBee
  - Direct HTTP requests and headless browsers are blocked by Kasada fingerprinting
  - **ScrapingBee integration**: Use `-scrapingbee <API_KEY>` or set `SCRAPINGBEE_API_KEY` env var
  - Uses stealth proxy mode (75 credits/request) for reliable bypass
  - Map view URL (`/map-N`) returns ~200 listings per page WITH coordinates (no geocoding needed)
  - List view URL (`/list-N`) returns only 25 listings without coordinates
  - **Early pagination stop**: Stops scraping when a page has no new properties (saves credits since results are sorted newest-first)
- **FarmProperty.com.au**: No bot protection, works with simple HTTP requests, has JSON-LD structured data with coordinates (PRIMARY SOURCE)
- **FarmBuy.com**: No bot protection, has embedded JSON in listing tiles + coordinates in map markers (SECONDARY SOURCE)
- REA embeds JSON data in `window.ArgonautExchange` on listing pages (when accessible)
- Listing URLs contain property type, suburb, postcode, and listing ID
- Some listings don't have coordinates; need geocoding fallback
- Rate limiting is essential to avoid IP blocks

### Isochrone Generation
- Valhalla public API at valhalla1.openstreetmap.de works well
- 2-hour isochrone covers most of Greater Sydney + Blue Mountains
- Consider pre-generating for other major cities (Newcastle, Wollongong)

### Distance Calculations
- Haversine gives straight-line distance, not driving distance
- For accurate drive times, would need OSRM routing API
- Pre-computing distances is essential for filter performance

### Map Performance
- MapLibre handles 100-200 markers smoothly
- Beyond 500 markers, need clustering or server-side filtering
- Isochrone polygons can be large; simplify geometry if needed

### Cross-Source Deduplication
- Properties within ~100m (0.001 degrees) are detected as duplicates
- `property_links` table tracks canonical vs duplicate properties
- Only canonical properties shown on map; duplicates hidden
- Property detail modal shows all source links when listed on multiple sites
