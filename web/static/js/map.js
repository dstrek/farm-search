// Map module - handles MapLibre GL JS map

const PropertyMap = {
    map: null,
    markers: [],
    popup: null,
    properties: [],  // Store properties for click lookups
    onMarkerClickCallback: null,
    ready: false,     // Track if map is fully initialized
    readyCallbacks: [], // Callbacks to run when ready
    isochroneLayerId: 'isochrone-layer',
    isochroneSourceId: 'isochrone-source',

    // Marker colours per source
    sourceColors: {
        'farmproperty': '#f97316',  // Orange
        'farmbuy': '#22c55e',       // Green
        'rea': '#ef4444',           // Red
        'domain': '#8b5cf6',        // Purple
        'default': '#2563eb'        // Blue
    },

    getSourceColor(source) {
        return this.sourceColors[source] || this.sourceColors.default;
    },

    // NSW bounds (approximately)
    NSW_BOUNDS: {
        center: [150.086, -34.048],
        zoom: 7.72,
        minZoom: 4,
        maxZoom: 18
    },

    // Sydney coordinates for reference
    SYDNEY: { lat: -33.8688, lng: 151.2093 },

    // Initialize the map
    init(containerId) {
        this.map = new maplibregl.Map({
            container: containerId,
            style: {
                version: 8,
                sources: {
                    osm: {
                        type: 'raster',
                        tiles: [
                            'https://tile.openstreetmap.org/{z}/{x}/{y}.png'
                        ],
                        tileSize: 256,
                        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
                    }
                },
                layers: [
                    {
                        id: 'osm-tiles',
                        type: 'raster',
                        source: 'osm',
                        minzoom: 0,
                        maxzoom: 19
                    }
                ]
            },
            center: this.NSW_BOUNDS.center,
            zoom: this.NSW_BOUNDS.zoom,
            minZoom: this.NSW_BOUNDS.minZoom,
            maxZoom: this.NSW_BOUNDS.maxZoom
        });

        // Add navigation controls
        this.map.addControl(new maplibregl.NavigationControl(), 'top-right');

        // Add scale
        this.map.addControl(new maplibregl.ScaleControl(), 'bottom-right');

        // Initialize popup
        this.popup = new maplibregl.Popup({
            closeButton: true,
            closeOnClick: false,
            maxWidth: '300px'
        });

        // Setup sources and layers when map loads
        this.map.on('load', () => {
            // Isochrone source
            this.map.addSource(this.isochroneSourceId, {
                type: 'geojson',
                data: { type: 'FeatureCollection', features: [] }
            });

            this.map.addLayer({
                id: this.isochroneLayerId,
                type: 'fill',
                source: this.isochroneSourceId,
                paint: {
                    'fill-color': '#2563eb',
                    'fill-opacity': 0.2,
                    'fill-outline-color': '#1d4ed8'
                }
            });

            // Mark as ready and run callbacks
            this.ready = true;
            this.readyCallbacks.forEach(cb => cb());
            this.readyCallbacks = [];
        });

        return this.map;
    },

    // Register callback to run when map is ready
    onReady(callback) {
        if (this.ready) {
            callback();
        } else {
            this.readyCallbacks.push(callback);
        }
    },

    // Clear all markers
    clearMarkers() {
        this.markers.forEach(marker => marker.remove());
        this.markers = [];
    },

    // Add property markers to the map
    addPropertyMarkers(properties, onMarkerClick) {
        this.clearMarkers();
        this.properties = properties;

        properties.forEach(property => {
            if (!property.lat || !property.lng) return;

            // Get colour based on source
            const color = this.getSourceColor(property.source);

            // Create marker element
            const el = document.createElement('div');
            el.className = 'property-marker';
            el.style.cssText = `
                width: 24px;
                height: 24px;
                background: ${color};
                border: 2px solid white;
                border-radius: 50%;
                cursor: pointer;
                box-shadow: 0 2px 4px rgba(0,0,0,0.2);
            `;

            const marker = new maplibregl.Marker({ element: el })
                .setLngLat([property.lng, property.lat])
                .addTo(this.map);

            // Add click handler
            el.addEventListener('click', (e) => {
                e.stopPropagation();
                this.showPropertyPopup(property, onMarkerClick);
            });

            this.markers.push(marker);
        });
    },

    // Show popup for a property
    showPropertyPopup(property, onViewDetails) {
        const html = `
            <div class="popup-content">
                <h3>${property.address || property.suburb || 'Property'}</h3>
                <div class="price">${property.price_text || 'Contact Agent'}</div>
                <div class="details">
                    ${property.property_type ? `<span>${property.property_type}</span>` : ''}
                    ${property.suburb ? `<span>${property.suburb}</span>` : ''}
                </div>
                <button class="view-btn" data-id="${property.id}">View Details</button>
            </div>
        `;

        this.popup
            .setLngLat([property.lng, property.lat])
            .setHTML(html)
            .addTo(this.map);

        // Add click handler for view details button
        setTimeout(() => {
            const btn = document.querySelector('.popup-content .view-btn');
            if (btn) {
                btn.addEventListener('click', () => {
                    onViewDetails(property.id);
                });
            }
        }, 0);
    },

    // Update isochrone layer
    async setIsochrone(city, minutes) {
        if (!minutes) {
            // Clear isochrone
            this.map.getSource(this.isochroneSourceId)?.setData({
                type: 'FeatureCollection',
                features: []
            });
            return;
        }

        try {
            const geojson = await API.getIsochrone(city, minutes);
            if (geojson) {
                this.map.getSource(this.isochroneSourceId)?.setData(geojson);
            }
        } catch (err) {
            console.warn('Failed to load isochrone:', err);
        }
    },

    // Get current map bounds as filter string
    getBoundsString() {
        const bounds = this.map.getBounds();
        const sw = bounds.getSouthWest();
        const ne = bounds.getNorthEast();
        return `${sw.lat},${sw.lng},${ne.lat},${ne.lng}`;
    },

    // Fit map to show all properties
    fitToMarkers() {
        if (this.properties.length === 0) return;

        const bounds = new maplibregl.LngLatBounds();
        this.properties.forEach(p => {
            if (p.lat && p.lng) {
                bounds.extend([p.lng, p.lat]);
            }
        });

        this.map.fitBounds(bounds, { padding: 50 });
    }
};
