-- Properties table
CREATE TABLE IF NOT EXISTS properties (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id TEXT NOT NULL,
    source TEXT NOT NULL,
    url TEXT NOT NULL,
    address TEXT,
    suburb TEXT,
    state TEXT DEFAULT 'NSW',
    postcode TEXT,
    latitude REAL,
    longitude REAL,
    price_min INTEGER,
    price_max INTEGER,
    price_text TEXT,
    property_type TEXT,
    bedrooms INTEGER,
    bathrooms INTEGER,
    land_size_sqm REAL,
    description TEXT,
    images TEXT,
    listed_at DATETIME,
    scraped_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Pre-computed distances for filtering
CREATE TABLE IF NOT EXISTS property_distances (
    property_id INTEGER REFERENCES properties(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_name TEXT NOT NULL,
    distance_km REAL,
    drive_time_mins INTEGER,
    PRIMARY KEY (property_id, target_type, target_name)
);

-- Towns reference table
CREATE TABLE IF NOT EXISTS towns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    state TEXT DEFAULT 'NSW',
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    population INTEGER
);

-- Schools reference table
CREATE TABLE IF NOT EXISTS schools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    school_type TEXT,
    suburb TEXT,
    state TEXT DEFAULT 'NSW',
    latitude REAL NOT NULL,
    longitude REAL NOT NULL
);

-- Unique constraint on external_id + source (same property ID can exist on different sites)
CREATE UNIQUE INDEX IF NOT EXISTS idx_properties_external_source ON properties(external_id, source);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_properties_coords ON properties(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_properties_price ON properties(price_min, price_max);
CREATE INDEX IF NOT EXISTS idx_properties_type ON properties(property_type);
CREATE INDEX IF NOT EXISTS idx_distances_property ON property_distances(property_id);
CREATE INDEX IF NOT EXISTS idx_distances_type ON property_distances(target_type, target_name);
CREATE INDEX IF NOT EXISTS idx_towns_coords ON towns(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_schools_coords ON schools(latitude, longitude);
