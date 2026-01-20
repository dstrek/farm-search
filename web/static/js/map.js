// Map module - handles MapLibre GL JS map

const PropertyMap = {
    map: null,

    properties: [],  // Store properties for click lookups
    propertiesById: new Map(),  // Quick lookup by ID
    onViewDetailsCallback: null,
    ready: false,     // Track if map is fully initialized
    readyCallbacks: [], // Callbacks to run when ready
    currentFilters: {},  // Current filter state for boundary loading
    isochroneLayerId: 'isochrone-layer',
    isochroneSourceId: 'isochrone-source',
    propertiesSourceId: 'properties-source',
    propertiesLayerId: 'properties-layer',
    boundariesSourceId: 'boundaries-source',
    boundariesLayerId: 'boundaries-layer',
    routeSourceId: 'route-source',
    routeLayerId: 'route-layer',
    currentBaseLayer: 'streets',  // 'streets' or 'satellite'
    boundariesMinZoom: 12,  // Minimum zoom level to show boundaries
    boundariesLoading: false,  // Prevent concurrent boundary requests

    // Viewport persistence
    VIEWPORT_STORAGE_KEY: 'farm-search-viewport',
    viewportSaveTimeout: null,

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
        // Try to restore saved viewport, fall back to NSW defaults
        const savedViewport = this.loadViewport();
        const initialCenter = savedViewport ? savedViewport.center : this.NSW_BOUNDS.center;
        const initialZoom = savedViewport ? savedViewport.zoom : this.NSW_BOUNDS.zoom;

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
            center: initialCenter,
            zoom: initialZoom,
            minZoom: this.NSW_BOUNDS.minZoom,
            maxZoom: this.NSW_BOUNDS.maxZoom
        });

        // Add navigation controls
        this.map.addControl(new maplibregl.NavigationControl(), 'top-right');

        // Add scale
        this.map.addControl(new maplibregl.ScaleControl(), 'bottom-right');

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

            // Boundaries source (cadastral lot polygons)
            this.map.addSource(this.boundariesSourceId, {
                type: 'geojson',
                data: { type: 'FeatureCollection', features: [] }
            });

            // Boundaries fill layer (semi-transparent)
            this.map.addLayer({
                id: this.boundariesLayerId,
                type: 'fill',
                source: this.boundariesSourceId,
                minzoom: this.boundariesMinZoom,
                paint: {
                    'fill-color': '#22c55e',
                    'fill-opacity': 0.15
                }
            });

            // Boundaries outline layer
            this.map.addLayer({
                id: this.boundariesLayerId + '-outline',
                type: 'line',
                source: this.boundariesSourceId,
                minzoom: this.boundariesMinZoom,
                paint: {
                    'line-color': '#16a34a',
                    'line-width': 2,
                    'line-opacity': 0.8
                }
            });

            // Route source (for drawing route to nearest town)
            this.map.addSource(this.routeSourceId, {
                type: 'geojson',
                data: { type: 'FeatureCollection', features: [] }
            });

            // Route line layer - white outline (rendered first, below)
            this.map.addLayer({
                id: this.routeLayerId + '-outline',
                type: 'line',
                source: this.routeSourceId,
                paint: {
                    'line-color': '#ffffff',
                    'line-width': 6,
                    'line-opacity': 1
                }
            });

            // Route line layer - blue fill (rendered on top)
            this.map.addLayer({
                id: this.routeLayerId,
                type: 'line',
                source: this.routeSourceId,
                paint: {
                    'line-color': '#2563eb',  // Blue-600
                    'line-width': 3,
                    'line-opacity': 1
                }
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

            // Change cursor on hover for properties
            this.map.on('mouseenter', this.propertiesLayerId, () => {
                this.map.getCanvas().style.cursor = 'pointer';
            });
            this.map.on('mouseleave', this.propertiesLayerId, () => {
                this.map.getCanvas().style.cursor = '';
            });

            // Load boundaries when zoomed in and map moves
            this.map.on('moveend', () => {
                this.loadBoundariesIfNeeded();
                this.saveViewport();
            });
            this.map.on('zoomend', () => {
                this.loadBoundariesIfNeeded();
            });

            // Click handler for property markers - opens sidebar directly
            this.map.on('click', this.propertiesLayerId, (e) => {
                if (e.features && e.features.length > 0) {
                    const feature = e.features[0];
                    const propertyId = feature.properties.id;
                    if (this.onViewDetailsCallback) {
                        this.onViewDetailsCallback(propertyId);
                    }
                }
            });

            // Mark as ready and run callbacks
            this.ready = true;
            this.readyCallbacks.forEach(cb => cb());
            this.readyCallbacks = [];

            // Load boundaries if already zoomed in (e.g., restored viewport)
            this.loadBoundariesIfNeeded();
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
    },

    // Set current filters (called by app when filters change)
    setFilters(filters) {
        this.currentFilters = filters || {};
        // Reload boundaries with new filters if map is ready
        if (this.ready) {
            this.loadBoundariesIfNeeded();
        }
    },

    // Load property boundaries when zoomed in enough
    async loadBoundariesIfNeeded() {
        if (!this.map || !this.ready) return;
        const zoom = this.map.getZoom();
        
        // Clear boundaries if zoomed out
        if (zoom < this.boundariesMinZoom) {
            const source = this.map.getSource(this.boundariesSourceId);
            if (source) {
                source.setData({ type: 'FeatureCollection', features: [] });
            }
            return;
        }

        // Prevent concurrent requests
        if (this.boundariesLoading) return;
        this.boundariesLoading = true;

        try {
            const bounds = this.getBoundsString();
            const geojson = await API.getBoundaries(bounds, zoom, this.currentFilters);
            
            const source = this.map.getSource(this.boundariesSourceId);
            if (source && geojson) {
                source.setData(geojson);
            }
        } catch (err) {
            console.warn('Failed to load boundaries:', err);
        } finally {
            this.boundariesLoading = false;
        }
    },

    // ==================== Route Display ====================

    // Show route from property to a destination
    // Options: town name (string) OR { town: 'name' } OR { toLat, toLng, name }
    async showRoute(fromLat, fromLng, options) {
        if (!options) {
            this.clearRoute();
            return;
        }

        try {
            const geojson = await API.getRoute(fromLat, fromLng, options);
            if (geojson) {
                this.onReady(() => {
                    const source = this.map.getSource(this.routeSourceId);
                    if (source) {
                        source.setData(geojson);
                    }
                });
            }
        } catch (err) {
            console.warn('Failed to load route:', err);
        }
    },

    // Clear the route from map
    clearRoute() {
        this.onReady(() => {
            const source = this.map.getSource(this.routeSourceId);
            if (source) {
                source.setData({ type: 'FeatureCollection', features: [] });
            }
        });
    },

    // ==================== Viewport Persistence ====================

    // Save current viewport to localStorage (debounced)
    saveViewport() {
        // Debounce: clear any pending save
        if (this.viewportSaveTimeout) {
            clearTimeout(this.viewportSaveTimeout);
        }

        this.viewportSaveTimeout = setTimeout(() => {
            try {
                const center = this.map.getCenter();
                const data = {
                    lng: center.lng,
                    lat: center.lat,
                    zoom: this.map.getZoom(),
                    savedAt: new Date().toISOString()
                };
                localStorage.setItem(this.VIEWPORT_STORAGE_KEY, JSON.stringify(data));
            } catch (err) {
                console.warn('[Map] Failed to save viewport:', err);
            }
        }, 500);  // 500ms debounce
    },

    // Load saved viewport from localStorage
    // Returns { center: [lng, lat], zoom } or null if invalid/missing
    loadViewport() {
        try {
            const raw = localStorage.getItem(this.VIEWPORT_STORAGE_KEY);
            if (!raw) return null;

            const data = JSON.parse(raw);

            // Validate structure and values
            if (!this.validateViewport(data)) {
                console.log('[Map] Saved viewport invalid, using defaults');
                localStorage.removeItem(this.VIEWPORT_STORAGE_KEY);
                return null;
            }

            console.log('[Map] Restored viewport from', data.savedAt);
            return {
                center: [data.lng, data.lat],
                zoom: data.zoom
            };
        } catch (err) {
            console.warn('[Map] Failed to load viewport:', err);
            return null;
        }
    },

    // Validate viewport data
    validateViewport(data) {
        if (!data || typeof data !== 'object') return false;

        // Check required fields exist and are numbers
        if (typeof data.lng !== 'number' || isNaN(data.lng)) return false;
        if (typeof data.lat !== 'number' || isNaN(data.lat)) return false;
        if (typeof data.zoom !== 'number' || isNaN(data.zoom)) return false;

        // Validate longitude (-180 to 180)
        if (data.lng < -180 || data.lng > 180) return false;

        // Validate latitude (-90 to 90)
        if (data.lat < -90 || data.lat > 90) return false;

        // Validate zoom (reasonable range for our app)
        if (data.zoom < this.NSW_BOUNDS.minZoom || data.zoom > this.NSW_BOUNDS.maxZoom) return false;

        return true;
    },

    // Clear saved viewport
    clearViewport() {
        try {
            localStorage.removeItem(this.VIEWPORT_STORAGE_KEY);
            console.log('[Map] Cleared saved viewport');
        } catch (err) {
            console.warn('[Map] Failed to clear viewport:', err);
        }
    }
};
