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
    updated_at DATETIME NOT NULL,
    drive_time_sydney INTEGER,  -- Drive time to Sutherland in minutes (via Valhalla routing)
    nearest_town_1 TEXT,        -- Name of nearest town
    nearest_town_1_km REAL,     -- Distance to nearest town in km
    nearest_town_1_mins INTEGER,-- Drive time to nearest town in minutes
    nearest_town_2 TEXT,        -- Name of second nearest town  
    nearest_town_2_km REAL,     -- Distance to second nearest town in km
    nearest_town_2_mins INTEGER,-- Drive time to second nearest town in minutes
    nearest_school_1 TEXT,      -- Name of nearest public school
    nearest_school_1_km REAL,   -- Distance to nearest school in km
    nearest_school_1_mins INTEGER, -- Drive time to nearest school in minutes
    nearest_school_2 TEXT,      -- Name of second nearest school
    nearest_school_2_km REAL,   -- Distance to second nearest school in km
    nearest_school_2_mins INTEGER  -- Drive time to second nearest school in minutes
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

-- Property links table (tracks when same property appears on multiple sources)
-- When properties are detected as duplicates, the older one becomes "canonical"
CREATE TABLE IF NOT EXISTS property_links (
    canonical_id INTEGER NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    duplicate_id INTEGER NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    match_type TEXT NOT NULL,  -- 'coords' (same lat/lng), 'address' (similar address)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (duplicate_id),  -- Each property can only be a duplicate of one canonical
    CHECK (canonical_id != duplicate_id)
);

CREATE INDEX IF NOT EXISTS idx_property_links_canonical ON property_links(canonical_id);

-- Cadastral lots table (NSW DCDB lot boundaries)
CREATE TABLE IF NOT EXISTS cadastral_lots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lot_id_string TEXT NOT NULL UNIQUE,  -- e.g., "699//DP752033"
    lot_number TEXT,                      -- e.g., "699"
    plan_label TEXT,                      -- e.g., "DP752033"
    area_sqm REAL,                        -- Area in square meters
    geometry TEXT NOT NULL,               -- GeoJSON geometry (Polygon)
    centroid_lat REAL,                    -- Centroid latitude
    centroid_lng REAL,                    -- Centroid longitude
    fetched_at DATETIME NOT NULL
);

-- Link properties to cadastral lots (a property may span multiple lots)
CREATE TABLE IF NOT EXISTS property_lots (
    property_id INTEGER NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    lot_id INTEGER NOT NULL REFERENCES cadastral_lots(id) ON DELETE CASCADE,
    PRIMARY KEY (property_id, lot_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_properties_coords ON properties(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_properties_price ON properties(price_min, price_max);
CREATE INDEX IF NOT EXISTS idx_properties_type ON properties(property_type);
CREATE INDEX IF NOT EXISTS idx_distances_property ON property_distances(property_id);
CREATE INDEX IF NOT EXISTS idx_distances_type ON property_distances(target_type, target_name);
CREATE INDEX IF NOT EXISTS idx_towns_coords ON towns(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_schools_coords ON schools(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_cadastral_lots_coords ON cadastral_lots(centroid_lat, centroid_lng);
CREATE INDEX IF NOT EXISTS idx_property_lots_lot ON property_lots(lot_id);
