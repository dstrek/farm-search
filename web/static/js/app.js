// Main application entry point

// Image Gallery component for property images
const ImageGallery = {
    currentIndex: 0,
    images: [],

    // Generate gallery HTML
    render(images) {
        this.images = images || [];
        this.currentIndex = 0;

        if (this.images.length === 0) {
            return '';
        }

        const thumbnailsHtml = this.images.map((url, i) => 
            `<div class="gallery-thumb${i === 0 ? ' active' : ''}" data-index="${i}">
                <img src="${url}" alt="Image ${i + 1}" onerror="this.parentElement.style.display='none'">
            </div>`
        ).join('');

        return `
            <div class="image-gallery" data-gallery>
                <div class="gallery-main">
                    <img src="${this.images[0]}" alt="Property image" class="gallery-main-img" onerror="this.src='/static/img/no-image.png'">
                    ${this.images.length > 1 ? `
                        <button class="gallery-nav gallery-prev" aria-label="Previous image">&lsaquo;</button>
                        <button class="gallery-nav gallery-next" aria-label="Next image">&rsaquo;</button>
                    ` : ''}
                    <div class="gallery-counter">${this.images.length > 1 ? `1 / ${this.images.length}` : ''}</div>
                </div>
                ${this.images.length > 1 ? `
                    <div class="gallery-thumbs">
                        ${thumbnailsHtml}
                    </div>
                ` : ''}
            </div>
        `;
    },

    // Initialize gallery event handlers
    init(container) {
        const gallery = container.querySelector('[data-gallery]');
        if (!gallery) return;

        const mainImg = gallery.querySelector('.gallery-main-img');
        const counter = gallery.querySelector('.gallery-counter');
        const thumbs = gallery.querySelectorAll('.gallery-thumb');
        const prevBtn = gallery.querySelector('.gallery-prev');
        const nextBtn = gallery.querySelector('.gallery-next');

        const updateImage = (index) => {
            if (index < 0) index = this.images.length - 1;
            if (index >= this.images.length) index = 0;
            
            this.currentIndex = index;
            mainImg.src = this.images[index];
            
            if (counter) {
                counter.textContent = `${index + 1} / ${this.images.length}`;
            }

            thumbs.forEach((thumb, i) => {
                thumb.classList.toggle('active', i === index);
            });

            // Scroll active thumbnail into view
            const activeThumb = thumbs[index];
            if (activeThumb) {
                activeThumb.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
            }
        };

        // Thumbnail clicks
        thumbs.forEach(thumb => {
            thumb.addEventListener('click', () => {
                updateImage(parseInt(thumb.dataset.index));
            });
        });

        // Nav button clicks
        if (prevBtn) {
            prevBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                updateImage(this.currentIndex - 1);
            });
        }
        if (nextBtn) {
            nextBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                updateImage(this.currentIndex + 1);
            });
        }

        // Store reference for keyboard navigation
        gallery._updateImage = updateImage;
    },

    // Handle keyboard navigation (called from App)
    handleKeydown(e, container) {
        const gallery = container.querySelector('[data-gallery]');
        if (!gallery || !gallery._updateImage) return false;

        if (e.key === 'ArrowLeft') {
            gallery._updateImage(this.currentIndex - 1);
            return true;
        } else if (e.key === 'ArrowRight') {
            gallery._updateImage(this.currentIndex + 1);
            return true;
        }
        return false;
    }
};

const App = {
    // Initialize the application
    async init() {
        console.log('Initializing Farm Search...');

        // Initialize map
        PropertyMap.init('map');

        // Initialize filters
        Filters.init(
            () => this.applyFilters(),
            () => this.loadProperties()
        );

        // Setup property sidebar
        this.initPropertySidebar();

        // Setup layer switcher
        this.initLayerSwitcher();

        // Load initial properties
        this.loadProperties();
    },

    // Show/hide loading overlay
    showLoading(message = 'Loading properties...') {
        const overlay = document.getElementById('loading-overlay');
        overlay.querySelector('span').textContent = message;
        overlay.classList.remove('hidden');
    },

    hideLoading() {
        document.getElementById('loading-overlay').classList.add('hidden');
    },

    // Load properties with current filters
    async loadProperties() {
        this.showLoading('Loading properties...');
        try {
            const filters = Filters.getValues();
            
            // Optionally include map bounds
            // filters.bounds = PropertyMap.getBoundsString();

            const data = await API.getProperties(filters);
            
            PropertyMap.addPropertyMarkers(data.properties, (id) => {
                this.showPropertyDetails(id);
            });

            Filters.updateResultsCount(data.count);

            console.log(`Loaded ${data.count} properties`);
        } catch (err) {
            console.error('Failed to load properties:', err);
            Filters.updateResultsCount(0);
        } finally {
            this.hideLoading();
        }
    },

    // Apply filters and reload
    async applyFilters() {
        await this.loadProperties();
    },

    // Show property details in sidebar
    async showPropertyDetails(id) {
        // Show sidebar with loading state
        const container = document.getElementById('property-detail');
        container.innerHTML = '<div class="loading-spinner" style="margin: 40px auto;"></div>';
        this.showPropertySidebar();

        try {
            const property = await API.getProperty(id);
            this.renderPropertySidebar(property);

            // Show route to nearest town (by drive time if available, otherwise by distance)
            let routeTown = null;
            if (property.nearest_town_1_mins && property.nearest_town_2_mins) {
                // Both have drive times - pick the faster one
                routeTown = property.nearest_town_1_mins <= property.nearest_town_2_mins 
                    ? property.nearest_town_1 
                    : property.nearest_town_2;
            } else if (property.nearest_town_1) {
                // Fall back to nearest by distance
                routeTown = property.nearest_town_1;
            }

            if (property.lat && property.lng && routeTown) {
                PropertyMap.showRoute(property.lat, property.lng, routeTown);
            } else {
                PropertyMap.clearRoute();
            }
        } catch (err) {
            console.error('Failed to load property details:', err);
            container.innerHTML = '<p style="color: #dc2626; text-align: center;">Failed to load property details.</p>';
            PropertyMap.clearRoute();
        }
    },

    // Render property details in sidebar
    renderPropertySidebar(property) {
        const container = document.getElementById('property-detail');

        // Use image gallery for all images (no limit)
        const imagesHtml = ImageGallery.render(property.images);

        // Build source links - show all sources if property is listed on multiple sites
        let sourcesHtml = '';
        if (property.sources && property.sources.length > 1) {
            // Multiple sources - show all
            sourcesHtml = `
                <div class="property-sources">
                    <span class="sources-label">Listed on:</span>
                    ${property.sources.map(s => 
                        `<a href="${s.url}" target="_blank" rel="noopener" class="source-link">${formatSourceName(s.source)}</a>`
                    ).join('')}
                </div>
            `;
        } else {
            // Single source
            sourcesHtml = `
                <a href="${property.url}" target="_blank" rel="noopener" class="view-listing">
                    View on ${formatSourceName(property.source)}
                </a>
            `;
        }

        // Format drive time if available
        let driveTimeHtml = '';
        if (property.drive_time_sydney) {
            const hours = Math.floor(property.drive_time_sydney / 60);
            const mins = property.drive_time_sydney % 60;
            const timeStr = hours > 0 ? `${hours}h ${mins}m` : `${mins} min`;
            driveTimeHtml = `<div class="drive-time-info">${timeStr} drive to Sutherland</div>`;
        }

        // Format nearest towns if available (show drive time if available, otherwise distance)
        let nearestTownsHtml = '';
        if (property.nearest_town_1) {
            let town1Info = property.nearest_town_1_mins 
                ? `${property.nearest_town_1_mins} min` 
                : (property.nearest_town_1_km ? `${property.nearest_town_1_km.toFixed(0)} km` : '?');
            let townsContent = `<span class="town-item">${property.nearest_town_1} (${town1Info})</span>`;
            
            if (property.nearest_town_2) {
                let town2Info = property.nearest_town_2_mins 
                    ? `${property.nearest_town_2_mins} min` 
                    : (property.nearest_town_2_km ? `${property.nearest_town_2_km.toFixed(0)} km` : '?');
                townsContent += `<span class="town-item">${property.nearest_town_2} (${town2Info})</span>`;
            }
            
            nearestTownsHtml = `<div class="nearest-towns">Nearest towns: ${townsContent}</div>`;
        }

        container.innerHTML = `
            <h2>${property.address || 'Property Details'}</h2>
            <div class="price">${property.price_text || 'Contact Agent'}</div>
            <div class="property-meta">
                ${property.property_type ? `<span>${property.property_type}</span>` : ''}
                ${property.bedrooms ? `<span>${property.bedrooms} beds</span>` : ''}
                ${property.bathrooms ? `<span>${property.bathrooms} baths</span>` : ''}
                ${property.land_size_sqm ? `<span>${formatLandSize(property.land_size_sqm)}</span>` : ''}
            </div>
            ${driveTimeHtml}
            ${nearestTownsHtml}
            ${imagesHtml}
            <div class="description">${property.description || 'No description available.'}</div>
            ${sourcesHtml}
        `;

        // Initialize image gallery
        ImageGallery.init(container);
    },

    // Initialize property sidebar functionality
    initPropertySidebar() {
        const sidebar = document.getElementById('property-sidebar');
        const closeBtn = sidebar.querySelector('.sidebar-close');

        closeBtn.addEventListener('click', () => this.hidePropertySidebar());

        // Keyboard navigation
        document.addEventListener('keydown', (e) => {
            if (sidebar.classList.contains('hidden')) return;

            // Close on Escape key
            if (e.key === 'Escape') {
                this.hidePropertySidebar();
                return;
            }

            // Arrow keys for gallery navigation
            const container = document.getElementById('property-detail');
            if (ImageGallery.handleKeydown(e, container)) {
                e.preventDefault();
            }
        });
    },

    // Initialize layer switcher
    initLayerSwitcher() {
        const streetsBtn = document.getElementById('layer-streets');
        const satelliteBtn = document.getElementById('layer-satellite');

        streetsBtn.addEventListener('click', () => {
            PropertyMap.setBaseLayer('streets');
            streetsBtn.classList.add('active');
            satelliteBtn.classList.remove('active');
        });

        satelliteBtn.addEventListener('click', () => {
            PropertyMap.setBaseLayer('satellite');
            satelliteBtn.classList.add('active');
            streetsBtn.classList.remove('active');
        });
    },

    showPropertySidebar() {
        document.getElementById('property-sidebar').classList.remove('hidden');
    },

    hidePropertySidebar() {
        document.getElementById('property-sidebar').classList.add('hidden');
        PropertyMap.clearRoute();
    }
};

// Utility function for formatting land size
function formatLandSize(sqm) {
    if (sqm >= 10000) {
        return `${(sqm / 10000).toFixed(1)} ha`;
    }
    return `${sqm.toLocaleString()} sqm`;
}

// Format source name for display
function formatSourceName(source) {
    const names = {
        'farmproperty': 'FarmProperty',
        'farmbuy': 'FarmBuy',
        'rea': 'realestate.com.au',
        'domain': 'Domain'
    };
    return names[source] || source.toUpperCase();
}

// Start the app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    App.init();
});
