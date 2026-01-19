// Filters module - handles sidebar filter UI

const Filters = {
    // Get current filter values from UI
    getValues() {
        const filters = {};

        // Price range
        const priceMin = document.getElementById('price-min').value;
        const priceMax = document.getElementById('price-max').value;
        if (priceMin) filters.priceMin = parseInt(priceMin, 10);
        if (priceMax) filters.priceMax = parseInt(priceMax, 10);

        // Property types
        const typeCheckboxes = document.querySelectorAll('#property-types input[type="checkbox"]:checked');
        const types = Array.from(typeCheckboxes).map(cb => cb.value);
        if (types.length > 0) filters.types = types;

        // Land size
        const landSizeMin = document.getElementById('land-size-min').value;
        const landSizeMax = document.getElementById('land-size-max').value;
        if (landSizeMin) filters.landSizeMin = parseFloat(landSizeMin);
        if (landSizeMax) filters.landSizeMax = parseFloat(landSizeMax);

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

        // Drive time from Sydney
        const driveTime = document.getElementById('drive-time-sydney').value;
        if (driveTime) {
            filters.driveTimeSydneyMax = parseInt(driveTime, 10);
        }

        return filters;
    },

    // Clear all filters
    clear() {
        document.getElementById('price-min').value = '';
        document.getElementById('price-max').value = '';
        
        document.querySelectorAll('#property-types input[type="checkbox"]').forEach(cb => {
            cb.checked = false;
        });

        document.getElementById('land-size-min').value = '';
        document.getElementById('land-size-max').value = '';

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

        // Price inputs - debounced
        document.getElementById('price-min').addEventListener('input', debouncedApply);
        document.getElementById('price-max').addEventListener('input', debouncedApply);

        // Property type checkboxes - immediate
        document.querySelectorAll('#property-types input[type="checkbox"]').forEach(cb => {
            cb.addEventListener('change', onApply);
        });

        // Land size inputs - debounced
        document.getElementById('land-size-min').addEventListener('input', debouncedApply);
        document.getElementById('land-size-max').addEventListener('input', debouncedApply);

        // Range sliders - apply on change (mouseup/touchend)
        this.initRangeSlider('distance-sydney', 500, 'km', onApply);
        this.initRangeSlider('distance-town', 100, 'km', onApply);
        this.initRangeSlider('distance-school', 50, 'km', onApply);

        // Drive time dropdown - update isochrone and apply filter
        document.getElementById('drive-time-sydney').addEventListener('change', (e) => {
            const minutes = e.target.value;
            if (typeof PropertyMap !== 'undefined') {
                PropertyMap.setIsochrone('sydney', minutes);
            }
            onApply();
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

    // Update results count display
    updateResultsCount(count) {
        document.getElementById('results-count').textContent = count;
    }
};
