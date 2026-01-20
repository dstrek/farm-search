// API client for farm-search backend

const API = {
    baseUrl: '/api',

    // Fetch properties with filters
    async getProperties(filters = {}) {
        const params = new URLSearchParams();

        if (filters.priceMin) params.set('price_min', filters.priceMin);
        if (filters.priceMax) params.set('price_max', filters.priceMax);
        if (filters.types && filters.types.length > 0) {
            params.set('type', filters.types.join(','));
        }
        if (filters.landSizeMin) params.set('land_size_min', filters.landSizeMin);
        if (filters.landSizeMax) params.set('land_size_max', filters.landSizeMax);
        if (filters.distanceSydneyMax) params.set('distance_sydney_max', filters.distanceSydneyMax);
        if (filters.distanceTownMax) params.set('distance_town_max', filters.distanceTownMax);
        if (filters.driveTimeSydneyMax) params.set('drive_time_sydney_max', filters.driveTimeSydneyMax);
        if (filters.driveTimeTownMax) params.set('drive_time_town_max', filters.driveTimeTownMax);
        if (filters.driveTimeSchoolMax) params.set('drive_time_school_max', filters.driveTimeSchoolMax);
        if (filters.bounds) params.set('bounds', filters.bounds);
        if (filters.limit) params.set('limit', filters.limit);

        const response = await fetch(`${this.baseUrl}/properties?${params}`);
        if (!response.ok) {
            throw new Error(`Failed to fetch properties: ${response.statusText}`);
        }
        return response.json();
    },

    // Fetch single property details
    async getProperty(id) {
        const response = await fetch(`${this.baseUrl}/properties/${id}`);
        if (!response.ok) {
            throw new Error(`Failed to fetch property: ${response.statusText}`);
        }
        return response.json();
    },

    // Fetch filter options
    async getFilterOptions() {
        const response = await fetch(`${this.baseUrl}/filters/options`);
        if (!response.ok) {
            throw new Error(`Failed to fetch filter options: ${response.statusText}`);
        }
        return response.json();
    },

    // Trigger scraper
    async triggerScrape() {
        const response = await fetch(`${this.baseUrl}/scrape/trigger`, {
            method: 'POST'
        });
        if (!response.ok) {
            throw new Error(`Failed to trigger scrape: ${response.statusText}`);
        }
        return response.json();
    },

    // Fetch isochrone GeoJSON
    async getIsochrone(city, minutes) {
        const response = await fetch(`/data/isochrones/${city}_${minutes}.geojson`);
        if (!response.ok) {
            return null; // Isochrone may not exist
        }
        return response.json();
    },

    // Fetch property boundaries (cadastral lots) within map bounds
    // Accepts same filters as getProperties to ensure boundaries match visible properties
    async getBoundaries(bounds, zoom, filters = {}) {
        const params = new URLSearchParams();
        params.set('bounds', bounds);
        if (zoom !== undefined) {
            params.set('zoom', zoom);
        }
        
        // Add same filters as properties endpoint
        if (filters.priceMin) params.set('price_min', filters.priceMin);
        if (filters.priceMax) params.set('price_max', filters.priceMax);
        if (filters.types && filters.types.length > 0) {
            params.set('type', filters.types.join(','));
        }
        if (filters.landSizeMin) params.set('land_size_min', filters.landSizeMin);
        if (filters.landSizeMax) params.set('land_size_max', filters.landSizeMax);
        if (filters.distanceSydneyMax) params.set('distance_sydney_max', filters.distanceSydneyMax);
        if (filters.distanceTownMax) params.set('distance_town_max', filters.distanceTownMax);
        if (filters.driveTimeSydneyMax) params.set('drive_time_sydney_max', filters.driveTimeSydneyMax);
        if (filters.driveTimeTownMax) params.set('drive_time_town_max', filters.driveTimeTownMax);
        if (filters.driveTimeSchoolMax) params.set('drive_time_school_max', filters.driveTimeSchoolMax);

        const response = await fetch(`${this.baseUrl}/boundaries?${params}`);
        if (!response.ok) {
            throw new Error(`Failed to fetch boundaries: ${response.statusText}`);
        }
        return response.json();
    },

    // Fetch driving route from property to a destination
    // Can route by town name OR by coordinates
    // Options: { town: 'TownName' } OR { toLat, toLng, name }
    async getRoute(fromLat, fromLng, options) {
        let url;
        if (typeof options === 'string') {
            // Legacy: options is a town name
            url = `${this.baseUrl}/route?from_lat=${fromLat}&from_lng=${fromLng}&town=${encodeURIComponent(options)}`;
        } else if (options.town) {
            // Route by town name
            url = `${this.baseUrl}/route?from_lat=${fromLat}&from_lng=${fromLng}&town=${encodeURIComponent(options.town)}`;
        } else if (options.toLat !== undefined && options.toLng !== undefined) {
            // Route by coordinates
            url = `${this.baseUrl}/route?from_lat=${fromLat}&from_lng=${fromLng}&to_lat=${options.toLat}&to_lng=${options.toLng}`;
            if (options.name) {
                url += `&name=${encodeURIComponent(options.name)}`;
            }
        } else {
            console.error('getRoute: invalid options', options);
            return null;
        }
        
        const response = await fetch(url);
        if (!response.ok) {
            return null; // Route may not be available
        }
        return response.json();
    }
};
