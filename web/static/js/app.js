// Main application entry point

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

        // Setup modal
        this.initModal();

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

    // Show property details in modal
    async showPropertyDetails(id) {
        // Show modal with loading state
        const container = document.getElementById('property-detail');
        container.innerHTML = '<div class="loading-spinner" style="margin: 40px auto;"></div>';
        this.showModal();

        try {
            const property = await API.getProperty(id);
            this.renderPropertyModal(property);
        } catch (err) {
            console.error('Failed to load property details:', err);
            container.innerHTML = '<p style="color: #dc2626; text-align: center;">Failed to load property details.</p>';
        }
    },

    // Render property details in modal
    renderPropertyModal(property) {
        const container = document.getElementById('property-detail');

        let imagesHtml = '';
        if (property.images && property.images.length > 0) {
            imagesHtml = `
                <div class="property-images">
                    ${property.images.slice(0, 5).map(url => 
                        `<img src="${url}" alt="Property image" onerror="this.style.display='none'">`
                    ).join('')}
                </div>
            `;
        }

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

        container.innerHTML = `
            <h2>${property.address || 'Property Details'}</h2>
            <div class="price">${property.price_text || 'Contact Agent'}</div>
            <div class="property-meta">
                ${property.property_type ? `<span>${property.property_type}</span>` : ''}
                ${property.bedrooms ? `<span>${property.bedrooms} beds</span>` : ''}
                ${property.bathrooms ? `<span>${property.bathrooms} baths</span>` : ''}
                ${property.land_size_sqm ? `<span>${formatLandSize(property.land_size_sqm)}</span>` : ''}
            </div>
            ${imagesHtml}
            <div class="description">${property.description || 'No description available.'}</div>
            ${sourcesHtml}
        `;
    },

    // Initialize modal functionality
    initModal() {
        const modal = document.getElementById('property-modal');
        const closeBtn = modal.querySelector('.modal-close');

        closeBtn.addEventListener('click', () => this.hideModal());

        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                this.hideModal();
            }
        });

        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                this.hideModal();
            }
        });
    },

    showModal() {
        document.getElementById('property-modal').classList.remove('hidden');
    },

    hideModal() {
        document.getElementById('property-modal').classList.add('hidden');
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
