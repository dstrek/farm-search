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

    // Initialize filter event listeners
    init(onApply, onClear) {
        // Apply button
        document.getElementById('apply-filters').addEventListener('click', onApply);

        // Clear button
        document.getElementById('clear-filters').addEventListener('click', () => {
            this.clear();
            onClear();
        });

        // Range slider updates
        this.initRangeSlider('distance-sydney', 500, 'km');
        this.initRangeSlider('distance-town', 100, 'km');
        this.initRangeSlider('distance-school', 50, 'km');

        // Drive time dropdown - update isochrone on change
        document.getElementById('drive-time-sydney').addEventListener('change', (e) => {
            const minutes = e.target.value;
            if (typeof PropertyMap !== 'undefined') {
                PropertyMap.setIsochrone('sydney', minutes);
            }
        });
    },

    // Initialize a range slider with display update
    initRangeSlider(inputId, maxValue, unit) {
        const input = document.getElementById(inputId);
        const display = document.getElementById(`${inputId}-value`);

        input.addEventListener('input', () => {
            if (parseInt(input.value, 10) >= maxValue) {
                display.textContent = 'Any';
            } else {
                display.textContent = `${input.value} ${unit}`;
            }
        });
    },

    // Update results count display
    updateResultsCount(count) {
        document.getElementById('results-count').textContent = count;
    }
};
