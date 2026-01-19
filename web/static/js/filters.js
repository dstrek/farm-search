// Filters module - handles sidebar filter UI

const Filters = {
    // Storage configuration
    // Bump this version when filter structure changes to auto-reset invalid saved data
    STORAGE_KEY: 'farm-search-filters',
    STORAGE_VERSION: 4,

    // Define expected filter schema for validation
    // Each key maps to: { type, min, max } for range validation
    filterSchema: {
        'price-max': { type: 'number', min: 0, max: 36 },
        'land-size-min': { type: 'number', min: 0, max: 10 },
        'drive-time-sydney': { type: 'number', min: 15, max: 255 },
        'drive-time-town': { type: 'number', min: 5, max: 60 },
        'distance-school': { type: 'number', min: 0, max: 50 },
        'isochrone-overlay': { type: 'string', allowed: ['', '60', '90', '120', '150', '180'] }
    },

    // Price steps: $0, $100k-$2M in $100k increments, then $2.5M-$10M in $500k increments
    // Index 0 = Any, 1-20 = $100k-$2M, 21-36 = $2.5M-$10M
    priceSteps: [
        0,       // 0: Any
        100000, 200000, 300000, 400000, 500000,      // 1-5
        600000, 700000, 800000, 900000, 1000000,     // 6-10
        1100000, 1200000, 1300000, 1400000, 1500000, // 11-15
        1600000, 1700000, 1800000, 1900000, 2000000, // 16-20
        2500000, 3000000, 3500000, 4000000, 4500000, // 21-25
        5000000, 5500000, 6000000, 6500000, 7000000, // 26-30
        7500000, 8000000, 8500000, 9000000, 9500000, // 31-35
        10000000 // 36
    ],

    formatPrice(value) {
        if (value === 0) return 'Any';
        if (value >= 1000000) {
            return `$${(value / 1000000).toFixed(value % 1000000 === 0 ? 0 : 1)}M`;
        }
        return `$${value / 1000}k`;
    },

    // Get current filter values from UI
    getValues() {
        const filters = {};

        // Price range (max only)
        const priceMaxIdx = parseInt(document.getElementById('price-max').value, 10);
        if (priceMaxIdx < this.priceSteps.length - 1) filters.priceMax = this.priceSteps[priceMaxIdx];

        // Land size (slider: 0-9 = 10-100 HA, 10 = Any)
        const landSizeIdx = parseInt(document.getElementById('land-size-min').value, 10);
        if (landSizeIdx < 10) {
            // Convert HA to sqm (1 HA = 10000 sqm)
            filters.landSizeMin = (landSizeIdx + 1) * 10 * 10000;
        }

        // Drive time to Sutherland (in minutes)
        const driveTime = document.getElementById('drive-time-sydney');
        if (parseInt(driveTime.value, 10) < parseInt(driveTime.max, 10)) {
            filters.driveTimeSydneyMax = parseInt(driveTime.value, 10);
        }

        // Drive time to nearest town (in minutes)
        const driveTimeTown = document.getElementById('drive-time-town');
        if (parseInt(driveTimeTown.value, 10) < parseInt(driveTimeTown.max, 10)) {
            filters.driveTimeTownMax = parseInt(driveTimeTown.value, 10);
        }

        // Distance from school
        const distanceSchool = document.getElementById('distance-school');
        if (parseInt(distanceSchool.value, 10) < parseInt(distanceSchool.max, 10)) {
            filters.distanceSchoolMax = parseInt(distanceSchool.value, 10);
        }

        return filters;
    },

    // Clear all filters
    clear() {
        const priceMax = document.getElementById('price-max');
        priceMax.value = this.priceSteps.length - 1;
        this.updateRangeDisplay('price-max', 'Any');

        const landSize = document.getElementById('land-size-min');
        landSize.value = 10;
        this.updateRangeDisplay('land-size-min', 'Any');

        const driveTime = document.getElementById('drive-time-sydney');
        driveTime.value = driveTime.max;
        this.updateRangeDisplay('drive-time-sydney', 'Any');

        const driveTimeTown = document.getElementById('drive-time-town');
        driveTimeTown.value = driveTimeTown.max;
        this.updateRangeDisplay('drive-time-town', 'Any');

        const distanceSchool = document.getElementById('distance-school');
        distanceSchool.value = distanceSchool.max;
        this.updateRangeDisplay('distance-school', 'Any');

        document.getElementById('isochrone-overlay').value = '';
        if (typeof PropertyMap !== 'undefined') {
            PropertyMap.setIsochrone('sutherland', '');
        }
    },

    // Update range slider display value
    updateRangeDisplay(inputId, value) {
        const displayEl = document.getElementById(`${inputId}-value`);
        if (displayEl) {
            displayEl.textContent = value;
        }
    },

    // Debounce helper for text inputs
    debounce(fn, delay) {
        let timeoutId;
        return (...args) => {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => fn(...args), delay);
        };
    },

    // Initialize filter event listeners
    init(onApply, onClear) {
        // Store callback for reuse
        this.onApply = onApply;

        // Wrap onApply to also save filters after each change
        const onApplyAndSave = () => {
            onApply();
            this.save();
        };

        // Debounced version for text inputs
        const debouncedApply = this.debounce(onApplyAndSave, 300);

        // Load saved filters before setting up listeners
        const hadSavedFilters = this.load();

        // Clear button - also clears saved filters
        document.getElementById('clear-filters').addEventListener('click', () => {
            this.clear();
            this.clearSaved();
            onClear();
        });

        // Price slider (max only)
        this.initPriceSlider('price-max', onApplyAndSave);

        // Land size slider
        this.initLandSizeSlider('land-size-min', onApplyAndSave);

        // Drive time sliders
        this.initDriveTimeSlider('drive-time-sydney', onApplyAndSave);
        this.initDriveTimeSliderTown('drive-time-town', onApplyAndSave);

        // Range sliders - apply on change (mouseup/touchend)
        this.initRangeSlider('distance-school', 50, 'km', onApplyAndSave);

        // Isochrone overlay dropdown - updates map display only (not filtering)
        document.getElementById('isochrone-overlay').addEventListener('change', (e) => {
            const minutes = e.target.value;
            if (typeof PropertyMap !== 'undefined') {
                PropertyMap.setIsochrone('sutherland', minutes);
            }
            this.save();
        });

        // If we restored saved filters with an isochrone, load it when map is ready
        if (hadSavedFilters) {
            const isochrone = document.getElementById('isochrone-overlay').value;
            if (isochrone && typeof PropertyMap !== 'undefined') {
                PropertyMap.onReady(() => PropertyMap.setIsochrone('sutherland', isochrone));
            }
        }
    },

    // Initialize a range slider with display update
    initRangeSlider(inputId, maxValue, unit, onApply) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);

        // Update display on input (while dragging)
        input.addEventListener('input', () => {
            if (parseInt(input.value, 10) >= maxValue) {
                display.textContent = 'Any';
            } else {
                display.textContent = `${input.value} ${unit}`;
            }
        });

        // Apply filter on change (when released)
        input.addEventListener('change', onApply);
    },

    // Initialize price slider with custom steps
    initPriceSlider(inputId, onApply) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);
        const isMin = inputId === 'price-min';

        // Update display on input (while dragging)
        input.addEventListener('input', () => {
            const idx = parseInt(input.value, 10);
            if (isMin && idx === 0) {
                display.textContent = 'Any';
            } else if (!isMin && idx === this.priceSteps.length - 1) {
                display.textContent = 'Any';
            } else {
                display.textContent = this.formatPrice(this.priceSteps[idx]);
            }
        });

        // Apply filter on change (when released)
        input.addEventListener('change', onApply);
    },

    // Initialize land size slider (10 HA increments, 10-100 HA, max = Any)
    initLandSizeSlider(inputId, onApply) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);

        // Update display on input (while dragging)
        input.addEventListener('input', () => {
            const idx = parseInt(input.value, 10);
            if (idx >= 10) {
                display.textContent = 'Any';
            } else {
                display.textContent = `${(idx + 1) * 10} HA`;
            }
        });

        // Apply filter on change (when released)
        input.addEventListener('change', onApply);
    },

    // Format drive time for display (e.g., 90 -> "1h 30m", 255 -> "Any")
    formatDriveTime(minutes) {
        if (minutes >= 255) return 'Any';
        const hours = Math.floor(minutes / 60);
        const mins = minutes % 60;
        if (hours === 0) return `${mins}m`;
        if (mins === 0) return `${hours}h`;
        return `${hours}h ${mins}m`;
    },

    // Initialize drive time slider (30 min increments, 30 min - 8 hours)
    initDriveTimeSlider(inputId, onApply) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);

        // Update display on input (while dragging)
        input.addEventListener('input', () => {
            const mins = parseInt(input.value, 10);
            display.textContent = this.formatDriveTime(mins);
        });

        // Apply filter on change (when released)
        input.addEventListener('change', onApply);
    },

    // Initialize drive time to town slider (5 min increments, 5-60 min)
    initDriveTimeSliderTown(inputId, onApply) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);

        // Update display on input (while dragging)
        input.addEventListener('input', () => {
            const mins = parseInt(input.value, 10);
            if (mins >= 60) {
                display.textContent = 'Any';
            } else {
                display.textContent = `${mins} min`;
            }
        });

        // Apply filter on change (when released)
        input.addEventListener('change', onApply);
    },

    // Update results count display
    updateResultsCount(count) {
        document.getElementById('results-count').textContent = count;
    },

    // ==================== LocalStorage Persistence ====================

    // Validate a single filter value against the schema
    validateFilterValue(key, value) {
        const schema = this.filterSchema[key];
        if (!schema) return false;

        if (schema.type === 'number') {
            if (typeof value !== 'number' || isNaN(value)) return false;
            if (value < schema.min || value > schema.max) return false;
            return true;
        }

        if (schema.type === 'string') {
            if (typeof value !== 'string') return false;
            if (schema.allowed && !schema.allowed.includes(value)) return false;
            return true;
        }

        return false;
    },

    // Validate entire saved data structure
    validateSavedData(data) {
        // Check basic structure
        if (!data || typeof data !== 'object') {
            console.log('[Filters] Invalid data: not an object');
            return false;
        }

        // Check version
        if (data.version !== this.STORAGE_VERSION) {
            console.log(`[Filters] Version mismatch: expected ${this.STORAGE_VERSION}, got ${data.version}`);
            return false;
        }

        // Check filters object exists
        if (!data.filters || typeof data.filters !== 'object') {
            console.log('[Filters] Invalid data: missing filters object');
            return false;
        }

        // Validate each saved filter value
        for (const [key, value] of Object.entries(data.filters)) {
            if (!this.filterSchema[key]) {
                console.log(`[Filters] Unknown filter key: ${key}`);
                return false;
            }
            if (!this.validateFilterValue(key, value)) {
                console.log(`[Filters] Invalid value for ${key}: ${value}`);
                return false;
            }
        }

        return true;
    },

    // Get current UI state for all filters
    getUIState() {
        return {
            'price-max': parseInt(document.getElementById('price-max').value, 10),
            'land-size-min': parseInt(document.getElementById('land-size-min').value, 10),
            'drive-time-sydney': parseInt(document.getElementById('drive-time-sydney').value, 10),
            'drive-time-town': parseInt(document.getElementById('drive-time-town').value, 10),
            'distance-school': parseInt(document.getElementById('distance-school').value, 10),
            'isochrone-overlay': document.getElementById('isochrone-overlay').value
        };
    },

    // Save current filter state to localStorage
    save() {
        try {
            const data = {
                version: this.STORAGE_VERSION,
                filters: this.getUIState(),
                savedAt: new Date().toISOString()
            };
            localStorage.setItem(this.STORAGE_KEY, JSON.stringify(data));
        } catch (err) {
            console.warn('[Filters] Failed to save to localStorage:', err);
        }
    },

    // Load and restore filter state from localStorage
    // Returns true if filters were restored, false if reset to defaults
    load() {
        try {
            const raw = localStorage.getItem(this.STORAGE_KEY);
            if (!raw) {
                console.log('[Filters] No saved filters found');
                return false;
            }

            const data = JSON.parse(raw);

            if (!this.validateSavedData(data)) {
                console.log('[Filters] Saved data invalid, resetting to defaults');
                this.clearSaved();
                return false;
            }

            // Restore filter values to UI
            this.restoreUIState(data.filters);
            console.log('[Filters] Restored saved filters from', data.savedAt);
            return true;
        } catch (err) {
            console.warn('[Filters] Failed to load from localStorage:', err);
            this.clearSaved();
            return false;
        }
    },

    // Restore UI state from saved filters
    restoreUIState(filters) {
        // Restore range sliders
        if (filters['price-max'] !== undefined) {
            const el = document.getElementById('price-max');
            el.value = filters['price-max'];
            const idx = filters['price-max'];
            const display = idx === this.priceSteps.length - 1 ? 'Any' : this.formatPrice(this.priceSteps[idx]);
            this.updateRangeDisplay('price-max', display);
        }

        if (filters['land-size-min'] !== undefined) {
            const el = document.getElementById('land-size-min');
            el.value = filters['land-size-min'];
            const idx = filters['land-size-min'];
            const display = idx >= 10 ? 'Any' : `${(idx + 1) * 10} HA`;
            this.updateRangeDisplay('land-size-min', display);
        }

        if (filters['drive-time-sydney'] !== undefined) {
            const el = document.getElementById('drive-time-sydney');
            el.value = filters['drive-time-sydney'];
            this.updateRangeDisplay('drive-time-sydney', this.formatDriveTime(filters['drive-time-sydney']));
        }

        if (filters['drive-time-town'] !== undefined) {
            const el = document.getElementById('drive-time-town');
            el.value = filters['drive-time-town'];
            const display = filters['drive-time-town'] >= 60 ? 'Any' : `${filters['drive-time-town']} min`;
            this.updateRangeDisplay('drive-time-town', display);
        }

        if (filters['distance-school'] !== undefined) {
            const el = document.getElementById('distance-school');
            el.value = filters['distance-school'];
            const display = filters['distance-school'] >= 50 ? 'Any' : `${filters['distance-school']} km`;
            this.updateRangeDisplay('distance-school', display);
        }

        // Restore isochrone overlay dropdown (but don't trigger load yet)
        if (filters['isochrone-overlay'] !== undefined) {
            document.getElementById('isochrone-overlay').value = filters['isochrone-overlay'];
        }
    },

    // Clear saved filters from localStorage
    clearSaved() {
        try {
            localStorage.removeItem(this.STORAGE_KEY);
            console.log('[Filters] Cleared saved filters');
        } catch (err) {
            console.warn('[Filters] Failed to clear localStorage:', err);
        }
    }
};
