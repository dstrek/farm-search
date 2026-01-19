.PHONY: run build scrape migrate clean help seed isochrones distances

# Default target
help:
	@echo "Farm Search - NSW Property Map"
	@echo ""
	@echo "Usage:"
	@echo "  make run        - Run the web server with live reload (air)"
	@echo "  make build      - Build server, scraper, and tools binaries"
	@echo "  make scrape     - Run the property scraper"
	@echo "  make seed       - Seed database with sample properties"
	@echo "  make isochrones - Generate Sydney drive-time isochrone GeoJSON"
	@echo "  make distances  - Calculate property distances"
	@echo "  make migrate    - Initialize/migrate the database"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make deps       - Download Go dependencies"

# Run the server in development mode with live reload
run:
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	@PATH="$$PATH:$$(go env GOPATH)/bin" air

# Build binaries
build:
	@mkdir -p bin
	go build -o bin/server ./cmd/server
	go build -o bin/scraper ./cmd/scraper
	go build -o bin/tools ./cmd/tools

# Run the scraper
scrape:
	go run ./cmd/scraper

# Seed database with sample data
seed:
	@mkdir -p data
	go run ./cmd/tools seed

# Generate Sydney isochrones
isochrones:
	go run ./cmd/tools isochrones

# Calculate property distances
distances:
	go run ./cmd/tools distances

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
