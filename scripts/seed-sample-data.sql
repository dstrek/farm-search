-- Sample property data for development/testing
-- These are fictional listings based on realistic NSW locations

INSERT OR REPLACE INTO properties (
    external_id, source, url, address, suburb, state, postcode,
    latitude, longitude, price_min, price_max, price_text,
    property_type, bedrooms, bathrooms, land_size_sqm,
    description, images, scraped_at, updated_at
) VALUES
-- Rural properties near Sydney
('sample-001', 'sample', 'https://example.com/1', '123 Valley Road', 'Wisemans Ferry', 'NSW', '2775',
 -33.3833, 151.0167, 850000, 950000, '$850,000 - $950,000',
 'rural', 4, 2, 100000,
 'Beautiful 10-hectare rural property with stunning valley views. Features include a renovated 4-bedroom homestead, separate studio, dam, and established gardens.',
 '[]', datetime('now'), datetime('now')),

('sample-002', 'sample', 'https://example.com/2', '45 Hilltop Lane', 'Kurrajong Heights', 'NSW', '2758',
 -33.5333, 150.6333, 1200000, 1400000, '$1.2M - $1.4M',
 'acreage-semi-rural', 5, 3, 40000,
 'Spectacular 4-acre property with panoramic Blue Mountains views. Modern 5-bedroom home with pool, tennis court, and multiple outbuildings.',
 '[]', datetime('now'), datetime('now')),

('sample-003', 'sample', 'https://example.com/3', '789 Creek Road', 'Bilpin', 'NSW', '2758',
 -33.4833, 150.5167, 750000, 850000, '$750,000 - $850,000',
 'farm', 3, 2, 80000,
 'Working apple orchard on 8 hectares. Includes 3-bedroom cottage, packing shed, and established fruit trees. Great income potential.',
 '[]', datetime('now'), datetime('now')),

-- Hunter Valley properties
('sample-004', 'sample', 'https://example.com/4', '22 Vineyard Drive', 'Pokolbin', 'NSW', '2320',
 -32.7833, 151.2833, 2500000, 3000000, '$2.5M - $3M',
 'farm', 5, 4, 200000,
 'Premium 20-hectare vineyard property in the heart of Hunter Valley wine country. Includes cellar door, restaurant, and luxury homestead.',
 '[]', datetime('now'), datetime('now')),

('sample-005', 'sample', 'https://example.com/5', '156 Hunter Road', 'Singleton', 'NSW', '2330',
 -32.5667, 151.1667, 450000, 500000, '$450,000 - $500,000',
 'rural', 3, 1, 150000,
 'Entry-level grazing property of 15 hectares. Original 3-bedroom cottage requiring renovation. Good fencing and water.',
 '[]', datetime('now'), datetime('now')),

-- Central Tablelands
('sample-006', 'sample', 'https://example.com/6', '88 Oberon Road', 'Oberon', 'NSW', '2787',
 -33.6833, 149.8500, 680000, 750000, '$680,000 - $750,000',
 'rural', 4, 2, 500000,
 'Productive 50-hectare cattle property with modern improvements. New 4-bedroom home, quality cattle yards, and reliable water.',
 '[]', datetime('now'), datetime('now')),

('sample-007', 'sample', 'https://example.com/7', '234 Orange Road', 'Bathurst', 'NSW', '2795',
 -33.4167, 149.5833, 520000, 580000, '$520,000 - $580,000',
 'acreage-semi-rural', 4, 2, 20000,
 'Quality 2-hectare lifestyle property just 10 minutes from Bathurst CBD. Modern 4-bedroom home with shed, bore, and paddocks.',
 '[]', datetime('now'), datetime('now')),

-- Southern Highlands
('sample-008', 'sample', 'https://example.com/8', '12 Highland Lane', 'Bowral', 'NSW', '2576',
 -34.4833, 150.4167, 1800000, 2000000, '$1.8M - $2M',
 'rural', 5, 3, 60000,
 'Prestigious 6-hectare estate in the Southern Highlands. Grand 5-bedroom home with tennis court, pool, and English-style gardens.',
 '[]', datetime('now'), datetime('now')),

('sample-009', 'sample', 'https://example.com/9', '67 Moss Vale Road', 'Berrima', 'NSW', '2577',
 -34.4833, 150.3333, 950000, 1050000, '$950,000 - $1.05M',
 'acreage-semi-rural', 4, 2, 30000,
 'Charming 3-hectare property near historic Berrima village. Renovated 4-bedroom weatherboard cottage with separate studio.',
 '[]', datetime('now'), datetime('now')),

-- South Coast
('sample-010', 'sample', 'https://example.com/10', '445 Coast Road', 'Milton', 'NSW', '2538',
 -35.3167, 150.4333, 890000, 980000, '$890,000 - $980,000',
 'rural', 3, 2, 80000,
 '8-hectare coastal hinterland property with ocean glimpses. 3-bedroom home, multiple dams, and lush pastures.',
 '[]', datetime('now'), datetime('now')),

-- Far North Coast
('sample-011', 'sample', 'https://example.com/11', '78 Hinterland Drive', 'Bangalow', 'NSW', '2479',
 -28.6833, 153.5167, 1500000, 1700000, '$1.5M - $1.7M',
 'farm', 4, 3, 120000,
 'Stunning 12-hectare macadamia farm near Byron Bay. Contemporary 4-bedroom home with pool and spectacular hinterland views.',
 '[]', datetime('now'), datetime('now')),

('sample-012', 'sample', 'https://example.com/12', '23 River Road', 'Lismore', 'NSW', '2480',
 -28.8167, 153.2833, 380000, 420000, '$380,000 - $420,000',
 'rural', 3, 1, 40000,
 'Affordable 4-hectare hobby farm with older 3-bedroom home. Flood-free location with good road access.',
 '[]', datetime('now'), datetime('now')),

-- New England
('sample-013', 'sample', 'https://example.com/13', '567 Tablelands Road', 'Armidale', 'NSW', '2350',
 -30.5167, 151.6667, 720000, 800000, '$720,000 - $800,000',
 'farm', 4, 2, 2000000,
 'Quality 200-hectare grazing property on the New England Tablelands. Well-improved with 4-bedroom home and excellent infrastructure.',
 '[]', datetime('now'), datetime('now')),

-- Western NSW
('sample-014', 'sample', 'https://example.com/14', '1234 Mitchell Highway', 'Dubbo', 'NSW', '2830',
 -32.2500, 148.6167, 550000, 620000, '$550,000 - $620,000',
 'rural', 4, 2, 1000000,
 '100-hectare mixed farming property west of Dubbo. Solid 4-bedroom home, good sheds, and reliable bore water.',
 '[]', datetime('now'), datetime('now')),

('sample-015', 'sample', 'https://example.com/15', '89 Outback Road', 'Broken Hill', 'NSW', '2880',
 -31.9500, 141.4667, 180000, 220000, '$180,000 - $220,000',
 'rural', 3, 1, 5000000,
 'Remote 500-hectare station property in Far Western NSW. Original homestead with character, perfect for adventurous buyers.',
 '[]', datetime('now'), datetime('now'));

-- Add distance calculations for sample properties
-- Using simplified distance calculation (approximate km)
-- Note: SQLite doesn't have SQRT, so we pre-calculate distances

-- Sydney distances (pre-calculated approximations in km)
INSERT OR REPLACE INTO property_distances (property_id, target_type, target_name, distance_km)
SELECT p.id, 'capital', 'Sydney',
    CASE p.external_id
        WHEN 'sample-001' THEN 62.5   -- Wisemans Ferry
        WHEN 'sample-002' THEN 58.3   -- Kurrajong Heights
        WHEN 'sample-003' THEN 75.2   -- Bilpin
        WHEN 'sample-004' THEN 165.4  -- Pokolbin
        WHEN 'sample-005' THEN 195.8  -- Singleton
        WHEN 'sample-006' THEN 170.2  -- Oberon
        WHEN 'sample-007' THEN 201.5  -- Bathurst
        WHEN 'sample-008' THEN 110.6  -- Bowral
        WHEN 'sample-009' THEN 125.3  -- Berrima
        WHEN 'sample-010' THEN 215.4  -- Milton
        WHEN 'sample-011' THEN 765.2  -- Bangalow
        WHEN 'sample-012' THEN 730.5  -- Lismore
        WHEN 'sample-013' THEN 480.3  -- Armidale
        WHEN 'sample-014' THEN 395.7  -- Dubbo
        WHEN 'sample-015' THEN 1150.8 -- Broken Hill
        ELSE 100.0
    END
FROM properties p WHERE p.source = 'sample';

-- Add nearest town distances (simplified)
INSERT OR REPLACE INTO property_distances (property_id, target_type, target_name, distance_km)
SELECT 
    p.id,
    'town',
    p.suburb,
    5.0  -- Assume 5km to nearest major town for sample data
FROM properties p WHERE p.source = 'sample';

-- Add nearest school distances (simplified)
INSERT OR REPLACE INTO property_distances (property_id, target_type, target_name, distance_km)
SELECT 
    p.id,
    'school',
    'Local School',
    8.0  -- Assume 8km to nearest school for sample data
FROM properties p WHERE p.source = 'sample';
