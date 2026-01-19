// Filters module - handles sidebar filter UI

const Filters = {
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

        // Distance from Sydney
        const distanceSydney = document.getElementById('distance-sydney');
        if (distanceSydney.value < distanceSydney.max) {
            filters.distanceSydneyMax = parseInt(distanceSydney.value, 10);
        }

        // Distance from town
        const distanceTown = document.getElementById('distance-town');
        if (distanceTown.value < distanceTown.max) {
            filters.distanceTownMax = parseInt(distanceTown.value, 10);
        }

        // Distance from school
        const distanceSchool = document.getElementById('distance-school');
        if (distanceSchool.value < distanceSchool.max) {
            filters.distanceSchoolMax = parseInt(distanceSchool.value, 10);
        }

        // Note: Drive time dropdown only controls isochrone display, not property filtering

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

        const distanceSydney = document.getElementById('distance-sydney');
        distanceSydney.value = distanceSydney.max;
        this.updateRangeDisplay('distance-sydney', 'Any');

        const distanceTown = document.getElementById('distance-town');
        distanceTown.value = distanceTown.max;
        this.updateRangeDisplay('distance-town', 'Any');

        const distanceSchool = document.getElementById('distance-school');
        distanceSchool.value = distanceSchool.max;
        this.updateRangeDisplay('distance-school', 'Any');

        document.getElementById('drive-time-sydney').value = '';
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

        // Debounced version for text inputs
        const debouncedApply = this.debounce(onApply, 300);

        // Clear button
        document.getElementById('clear-filters').addEventListener('click', () => {
            this.clear();
            onClear();
        });

        // Price slider (max only)
        this.initPriceSlider('price-max', onApply);

        // Land size slider
        this.initLandSizeSlider('land-size-min', onApply);

        // Range sliders - apply on change (mouseup/touchend)
        this.initRangeSlider('distance-sydney', 500, 'km', onApply);
        this.initRangeSlider('distance-town', 100, 'km', onApply);
        this.initRangeSlider('distance-school', 50, 'km', onApply);

        // Drive time dropdown - only updates isochrone display, no property filtering
        document.getElementById('drive-time-sydney').addEventListener('change', (e) => {
            const minutes = e.target.value;
            if (typeof PropertyMap !== 'undefined') {
                PropertyMap.setIsochrone('sydney', minutes);
            }
        });
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

    // Update results count display
    updateResultsCount(count) {
        document.getElementById('results-count').textContent = count;
    }
};
