.PHONY: run build scrape migrate clean help seed isochrones distances drivetimes towns towndrivetimes cadastral deploy setup-server

# Default target
help:
	@echo "Farm Search - NSW Property Map"
	@echo ""
	@echo "Usage:"
	@echo "  make run           - Run the web server with live reload (air)"
	@echo "  make build         - Build server, scraper, and tools binaries"
	@echo "  make scrape        - Run the property scraper (ARGS=\"-source=farmproperty -pages=1\")"
	@echo "  make seed          - Seed database with sample properties"
	@echo "  make isochrones    - Generate Sutherland drive-time isochrone GeoJSON"
	@echo "  make distances     - Calculate property distances (straight-line)"
	@echo "  make drivetimes    - Calculate drive times to Sutherland"
	@echo "  make towns         - Calculate nearest towns for properties"
	@echo "  make towndrivetimes - Calculate drive times to nearest towns"
	@echo "  make cadastral     - Fetch cadastral lot boundaries"
	@echo "  make migrate       - Initialize/migrate the database"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make deps          - Download Go dependencies"

# Run the server in development mode with live reload
run:
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	@# Kill any orphaned process on port 8080
	@PID=$$(lsof -ti:8080 2>/dev/null); if [ -n "$$PID" ]; then echo "Killing orphaned process on port 8080 (PID: $$PID)"; kill -9 $$PID 2>/dev/null || true; fi
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi && PATH="$$PATH:$$(go env GOPATH)/bin" air

# Build binaries
build:
	@mkdir -p bin
	go build -o bin/server ./cmd/server
	go build -o bin/scraper ./cmd/scraper
	go build -o bin/tools ./cmd/tools

# Run the scraper
# Usage: make scrape ARGS="-source=farmproperty -pages=1"
#    or: go run ./cmd/scraper -source=farmproperty -pages=1
scrape:
	go run ./cmd/scraper $(ARGS)

# Seed database with sample data
seed:
	@mkdir -p data
	go run ./cmd/tools seed

# Generate Sydney isochrones
isochrones:
	go run ./cmd/tools isochrones

# Calculate property distances (straight-line)
distances:
	go run ./cmd/tools distances

# Calculate drive times to Sutherland
drivetimes:
	go run ./cmd/tools drivetimes

# Calculate nearest towns for properties
towns:
	go run ./cmd/tools towns

# Calculate drive times to nearest towns
towndrivetimes:
	go run ./cmd/tools towndrivetimes

# Fetch cadastral lot boundaries
cadastral:
	go run ./cmd/tools cadastral

# Initialize database (creates tables via seed which calls db.New)
migrate: seed
	@echo "Database initialized at data/farm-search.db"

# Download dependencies
deps:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f data/farm-search.db

# Run without live reload (simple mode)
run-simple:
	go run ./cmd/server

# Full setup: deps + seed (seed creates DB)
setup: deps seed
	@echo "Setup complete! Run 'make run' to start the server."

# Deploy to production server
deploy:
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi && ./scripts/deploy.sh

# Initial server setup (run once on fresh server)
setup-server:
	@ssh root@107.191.56.246 'bash -s' < scripts/setup-server.sh
