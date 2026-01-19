#!/bin/bash
# Generate Sydney drive-time isochrones using Valhalla API
# Saves GeoJSON files to web/static/data/isochrones/

set -e

OUTPUT_DIR="web/static/data/isochrones"
mkdir -p "$OUTPUT_DIR"

# Sydney CBD coordinates
SYDNEY_LAT="-33.8688"
SYDNEY_LNG="151.2093"

# Drive time intervals in minutes
INTERVALS=(15 30 45 60 75 90 105 120)

# Valhalla public API endpoint
API_URL="https://valhalla1.openstreetmap.de/isochrone"

echo "Generating Sydney isochrones..."

for MINS in "${INTERVALS[@]}"; do
    OUTPUT_FILE="$OUTPUT_DIR/sydney_${MINS}.geojson"
    
    echo "Generating ${MINS} minute isochrone..."
    
    # Build the request JSON
    REQUEST=$(cat <<EOF
{
  "locations": [{"lat": ${SYDNEY_LAT}, "lon": ${SYDNEY_LNG}}],
  "costing": "auto",
  "contours": [{"time": ${MINS}}],
  "polygons": true,
  "denoise": 0.5,
  "generalize": 100
}
EOF
)
    
    # URL encode the JSON
    ENCODED=$(echo "$REQUEST" | jq -sRr @uri)
    
    # Make the request
    curl -s "${API_URL}?json=${ENCODED}" | jq '.' > "$OUTPUT_FILE"
    
    if [ -s "$OUTPUT_FILE" ]; then
        echo "  Saved to $OUTPUT_FILE"
    else
        echo "  Failed to generate ${MINS} minute isochrone"
        rm -f "$OUTPUT_FILE"
    fi
    
    # Rate limiting
    sleep 2
done

echo "Done! Generated isochrones in $OUTPUT_DIR"
