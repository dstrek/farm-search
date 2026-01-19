// Map module - handles MapLibre GL JS map

const PropertyMap = {
    map: null,
    popup: null,
    properties: [],  // Store properties for click lookups
    propertiesById: new Map(),  // Quick lookup by ID
    onViewDetailsCallback: null,
    ready: false,     // Track if map is fully initialized
    readyCallbacks: [], // Callbacks to run when ready
    isochroneLayerId: 'isochrone-layer',
    isochroneSourceId: 'isochrone-source',
    propertiesSourceId: 'properties-source',
    propertiesLayerId: 'properties-layer',
    currentBaseLayer: 'streets',  // 'streets' or 'satellite'

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
                    },
                    satellite: {
                        type: 'raster',
                        tiles: [
                            `https://api.mapbox.com/v4/mapbox.satellite/{z}/{x}/{y}@2x.jpg90?access_token=${window.MAPBOX_TOKEN}`
                        ],
                        tileSize: 512,
                        attribution: '&copy; <a href="https://www.mapbox.com/">Mapbox</a> &copy; <a href="https://www.maxar.com/">Maxar</a>'
                    }
                },
                layers: [
                    {
                        id: 'osm-tiles',
                        type: 'raster',
                        source: 'osm',
                        minzoom: 0,
                        maxzoom: 19
                    },
                    {
                        id: 'satellite-tiles',
                        type: 'raster',
                        source: 'satellite',
                        minzoom: 0,
                        maxzoom: 19,
                        layout: {
                            visibility: 'none'
                        }
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
                    'fill-opacity': 0.1
                }
            });

            // Add darker border for isochrone
            this.map.addLayer({
                id: this.isochroneLayerId + '-border',
                type: 'line',
                source: this.isochroneSourceId,
                paint: {
                    'line-color': '#1e40af',
                    'line-width': 2,
                    'line-opacity': 0.8
                }
            });

            // Properties source (GeoJSON for GPU-rendered markers)
            this.map.addSource(this.propertiesSourceId, {
                type: 'geojson',
                data: { type: 'FeatureCollection', features: [] }
            });

            // Properties circle layer - much faster than DOM markers
            this.map.addLayer({
                id: this.propertiesLayerId,
                type: 'circle',
                source: this.propertiesSourceId,
                paint: {
                    'circle-radius': 8,
                    'circle-color': ['get', 'color'],
                    'circle-stroke-color': '#ffffff',
                    'circle-stroke-width': 2
                }
            });

            // Change cursor on hover
            this.map.on('mouseenter', this.propertiesLayerId, () => {
                this.map.getCanvas().style.cursor = 'pointer';
            });
            this.map.on('mouseleave', this.propertiesLayerId, () => {
                this.map.getCanvas().style.cursor = '';
            });

            // Click handler for property markers
            this.map.on('click', this.propertiesLayerId, (e) => {
                if (e.features && e.features.length > 0) {
                    const feature = e.features[0];
                    const propertyId = feature.properties.id;
                    const property = this.propertiesById.get(propertyId);
                    if (property) {
                        this.showPropertyPopup(property);
                    }
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

    // Add property markers to the map using GeoJSON source
    addPropertyMarkers(properties, onViewDetails) {
        this.properties = properties;
        this.propertiesById.clear();
        this.onViewDetailsCallback = onViewDetails;

        // Build GeoJSON features
        const features = [];
        properties.forEach(property => {
            if (!property.lat || !property.lng) return;

            this.propertiesById.set(property.id, property);

            features.push({
                type: 'Feature',
                geometry: {
                    type: 'Point',
                    coordinates: [property.lng, property.lat]
                },
                properties: {
                    id: property.id,
                    color: this.getSourceColor(property.source)
                }
            });
        });

        const geojson = {
            type: 'FeatureCollection',
            features: features
        };

        // Wait for map to be ready before updating source
        this.onReady(() => {
            const source = this.map.getSource(this.propertiesSourceId);
            if (source) {
                source.setData(geojson);
            }
        });
    },

    // Show popup for a property
    showPropertyPopup(property) {
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
            if (btn && this.onViewDetailsCallback) {
                btn.addEventListener('click', () => {
                    this.onViewDetailsCallback(property.id);
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
    },

    // Switch between base layers (streets or satellite)
    setBaseLayer(layer) {
        if (layer === this.currentBaseLayer) return;
        
        this.onReady(() => {
            if (layer === 'satellite') {
                this.map.setLayoutProperty('osm-tiles', 'visibility', 'none');
                this.map.setLayoutProperty('satellite-tiles', 'visibility', 'visible');
            } else {
                this.map.setLayoutProperty('satellite-tiles', 'visibility', 'none');
                this.map.setLayoutProperty('osm-tiles', 'visibility', 'visible');
            }
            this.currentBaseLayer = layer;
        });
    }
};
