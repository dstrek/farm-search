package geo

import (
	"math"
)

const (
	// Earth radius in kilometers
	EarthRadiusKm = 6371.0
)

// Haversine calculates the great-circle distance between two points
// Returns distance in kilometers
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	// Haversine formula
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// Location represents a geographic point
type Location struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// Sydney CBD location
var Sydney = Location{
	Name:      "Sydney",
	Latitude:  -33.8688,
	Longitude: 151.2093,
}

// NSW towns and regional centres
// Includes major towns (pop >5000) and smaller regional service centres
// Source: ABS 2021 Census, NSW Geographic Names Board
var NSWTowns = []Location{
	// Major cities and large towns (pop >10000)
	{Name: "Newcastle", Latitude: -32.9283, Longitude: 151.7817},
	{Name: "Wollongong", Latitude: -34.4278, Longitude: 150.8931},
	{Name: "Central Coast", Latitude: -33.4245, Longitude: 151.3418},
	{Name: "Maitland", Latitude: -32.7325, Longitude: 151.5587},
	{Name: "Tweed Heads", Latitude: -28.1768, Longitude: 153.5414},
	{Name: "Wagga Wagga", Latitude: -35.1082, Longitude: 147.3598},
	{Name: "Albury", Latitude: -36.0737, Longitude: 146.9135},
	{Name: "Port Macquarie", Latitude: -31.4333, Longitude: 152.9000},
	{Name: "Tamworth", Latitude: -31.0833, Longitude: 150.9167},
	{Name: "Orange", Latitude: -33.2833, Longitude: 149.1000},
	{Name: "Dubbo", Latitude: -32.2500, Longitude: 148.6167},
	{Name: "Bathurst", Latitude: -33.4167, Longitude: 149.5833},
	{Name: "Lismore", Latitude: -28.8167, Longitude: 153.2833},
	{Name: "Coffs Harbour", Latitude: -30.3000, Longitude: 153.1333},
	{Name: "Nowra", Latitude: -34.8833, Longitude: 150.6000},
	{Name: "Armidale", Latitude: -30.5167, Longitude: 151.6667},
	{Name: "Goulburn", Latitude: -34.7500, Longitude: 149.7167},
	{Name: "Queanbeyan", Latitude: -35.3500, Longitude: 149.2333},
	{Name: "Broken Hill", Latitude: -31.9500, Longitude: 141.4667},
	{Name: "Griffith", Latitude: -34.2833, Longitude: 146.0333},

	// Medium towns (pop 5000-10000)
	{Name: "Cessnock", Latitude: -32.8333, Longitude: 151.3500},
	{Name: "Grafton", Latitude: -29.6833, Longitude: 152.9333},
	{Name: "Taree", Latitude: -31.9000, Longitude: 152.4500},
	{Name: "Ballina", Latitude: -28.8667, Longitude: 153.5667},
	{Name: "Singleton", Latitude: -32.5667, Longitude: 151.1667},
	{Name: "Muswellbrook", Latitude: -32.2667, Longitude: 150.8833},
	{Name: "Forster", Latitude: -32.1833, Longitude: 152.5167},
	{Name: "Raymond Terrace", Latitude: -32.7667, Longitude: 151.7500},
	{Name: "Moree", Latitude: -29.4667, Longitude: 149.8500},
	{Name: "Parkes", Latitude: -33.1333, Longitude: 148.1833},
	{Name: "Mudgee", Latitude: -32.5833, Longitude: 149.5833},
	{Name: "Lithgow", Latitude: -33.4833, Longitude: 150.1500},
	{Name: "Cowra", Latitude: -33.8333, Longitude: 148.6833},
	{Name: "Forbes", Latitude: -33.3833, Longitude: 148.0167},
	{Name: "Young", Latitude: -34.3167, Longitude: 148.3000},
	{Name: "Inverell", Latitude: -29.7833, Longitude: 151.1167},
	{Name: "Glen Innes", Latitude: -29.7333, Longitude: 151.7333},
	{Name: "Cooma", Latitude: -36.2333, Longitude: 149.1333},
	{Name: "Yass", Latitude: -34.8333, Longitude: 148.9167},
	{Name: "Leeton", Latitude: -34.5500, Longitude: 146.4000},
	{Name: "Narrabri", Latitude: -30.3333, Longitude: 149.7833},
	{Name: "Deniliquin", Latitude: -35.5333, Longitude: 144.9667},
	{Name: "Cootamundra", Latitude: -34.6333, Longitude: 148.0333},
	{Name: "Junee", Latitude: -34.8667, Longitude: 147.5833},
	{Name: "Tumut", Latitude: -35.3000, Longitude: 148.2167},
	{Name: "Casino", Latitude: -28.8667, Longitude: 153.0500},
	{Name: "Kempsey", Latitude: -31.0833, Longitude: 152.8333},
	{Name: "Byron Bay", Latitude: -28.6500, Longitude: 153.6167},

	// Smaller regional towns (pop 1000-5000)
	{Name: "Gunnedah", Latitude: -30.9833, Longitude: 150.2500},
	{Name: "Narrandera", Latitude: -34.7500, Longitude: 146.5500},
	{Name: "Temora", Latitude: -34.4500, Longitude: 147.5333},
	{Name: "West Wyalong", Latitude: -33.9333, Longitude: 147.2167},
	{Name: "Condobolin", Latitude: -33.0833, Longitude: 147.1500},
	{Name: "Wellington", Latitude: -32.5500, Longitude: 148.9500},
	{Name: "Coonabarabran", Latitude: -31.2667, Longitude: 149.2833},
	{Name: "Coonamble", Latitude: -30.9500, Longitude: 148.3833},
	{Name: "Walgett", Latitude: -30.0167, Longitude: 148.1167},
	{Name: "Bourke", Latitude: -30.0833, Longitude: 145.9333},
	{Name: "Brewarrina", Latitude: -29.9667, Longitude: 146.8667},
	{Name: "Lightning Ridge", Latitude: -29.4333, Longitude: 147.9833},
	{Name: "Nyngan", Latitude: -31.5667, Longitude: 147.2000},
	{Name: "Cobar", Latitude: -31.4833, Longitude: 145.8333},
	{Name: "Gilgandra", Latitude: -31.7167, Longitude: 148.6667},
	{Name: "Narromine", Latitude: -32.2333, Longitude: 148.2333},
	{Name: "Peak Hill", Latitude: -32.7167, Longitude: 148.1833},
	{Name: "Tullamore", Latitude: -32.6333, Longitude: 147.5667},
	{Name: "Tottenham", Latitude: -32.2333, Longitude: 147.3500},
	{Name: "Warren", Latitude: -31.7000, Longitude: 147.8333},
	{Name: "Trangie", Latitude: -31.9833, Longitude: 147.9833},
	{Name: "Hay", Latitude: -34.5167, Longitude: 144.8500},
	{Name: "Balranald", Latitude: -34.6333, Longitude: 143.5667},
	{Name: "Hillston", Latitude: -33.4833, Longitude: 145.5333},
	{Name: "Lake Cargelligo", Latitude: -33.3000, Longitude: 146.3667},
	{Name: "Molong", Latitude: -33.0833, Longitude: 148.8667},
	{Name: "Canowindra", Latitude: -33.5667, Longitude: 148.6667},
	{Name: "Grenfell", Latitude: -33.8833, Longitude: 148.1667},
	{Name: "Harden", Latitude: -34.5500, Longitude: 148.3667},
	{Name: "Boorowa", Latitude: -34.4333, Longitude: 148.7167},
	{Name: "Crookwell", Latitude: -34.4500, Longitude: 149.4667},
	{Name: "Gunning", Latitude: -34.7833, Longitude: 149.2667},
	{Name: "Braidwood", Latitude: -35.4500, Longitude: 149.8000},
	{Name: "Bega", Latitude: -36.6833, Longitude: 149.8500},
	{Name: "Merimbula", Latitude: -36.8833, Longitude: 149.9000},
	{Name: "Eden", Latitude: -37.0667, Longitude: 149.9000},
	{Name: "Bombala", Latitude: -36.9167, Longitude: 149.2333},
	{Name: "Delegate", Latitude: -37.0500, Longitude: 148.9333},
	{Name: "Jindabyne", Latitude: -36.4167, Longitude: 148.6167},
	{Name: "Berridale", Latitude: -36.3667, Longitude: 148.8333},
	{Name: "Adaminaby", Latitude: -35.9833, Longitude: 148.7667},
	{Name: "Tumbarumba", Latitude: -35.7833, Longitude: 148.0167},
	{Name: "Batlow", Latitude: -35.5167, Longitude: 148.1500},
	{Name: "Adelong", Latitude: -35.3167, Longitude: 148.0667},
	{Name: "Gundagai", Latitude: -35.0667, Longitude: 148.1000},
	{Name: "Holbrook", Latitude: -35.7167, Longitude: 147.3167},
	{Name: "Culcairn", Latitude: -35.6667, Longitude: 147.0333},
	{Name: "Corowa", Latitude: -35.9833, Longitude: 146.3833},
	{Name: "Mulwala", Latitude: -35.9833, Longitude: 146.0000},
	{Name: "Finley", Latitude: -35.6500, Longitude: 145.5667},
	{Name: "Tocumwal", Latitude: -35.8167, Longitude: 145.5667},
	{Name: "Jerilderie", Latitude: -35.3500, Longitude: 145.7333},
	{Name: "Berrigan", Latitude: -35.6667, Longitude: 145.8000},
	{Name: "Barham", Latitude: -35.6333, Longitude: 144.1333},
	{Name: "Moulamein", Latitude: -35.0833, Longitude: 144.0333},
	{Name: "Wakool", Latitude: -35.4667, Longitude: 144.4000},
	{Name: "Swan Hill", Latitude: -35.3333, Longitude: 143.5500},
	{Name: "Wentworth", Latitude: -34.1000, Longitude: 141.9167},
	{Name: "Wilcannia", Latitude: -31.5500, Longitude: 143.3833},
	{Name: "Menindee", Latitude: -32.3833, Longitude: 142.4167},
	{Name: "Ivanhoe", Latitude: -32.9000, Longitude: 144.3000},
	{Name: "White Cliffs", Latitude: -30.8500, Longitude: 143.0833},
	{Name: "Tibooburra", Latitude: -29.4333, Longitude: 142.0167},
	{Name: "Collarenebri", Latitude: -29.5333, Longitude: 148.5833},
	{Name: "Mungindi", Latitude: -28.9833, Longitude: 149.0667},
	{Name: "Boggabilla", Latitude: -28.6000, Longitude: 150.0500},
	{Name: "Goondiwindi", Latitude: -28.5500, Longitude: 150.3167},
	{Name: "Tenterfield", Latitude: -29.0500, Longitude: 152.0167},
	{Name: "Uralla", Latitude: -30.6333, Longitude: 151.5000},
	{Name: "Walcha", Latitude: -31.0000, Longitude: 151.6000},
	{Name: "Quirindi", Latitude: -31.5000, Longitude: 150.6833},
	{Name: "Werris Creek", Latitude: -31.3500, Longitude: 150.6500},
	{Name: "Manilla", Latitude: -30.7500, Longitude: 150.7167},
	{Name: "Barraba", Latitude: -30.3833, Longitude: 150.6167},
	{Name: "Bingara", Latitude: -29.8667, Longitude: 150.5667},
	{Name: "Warialda", Latitude: -29.5333, Longitude: 150.5667},
	{Name: "Ashford", Latitude: -29.3167, Longitude: 151.0833},
	{Name: "Emmaville", Latitude: -29.4500, Longitude: 151.6000},
	{Name: "Deepwater", Latitude: -29.4333, Longitude: 151.8667},
	{Name: "Guyra", Latitude: -30.2167, Longitude: 151.6667},
	{Name: "Dorrigo", Latitude: -30.3333, Longitude: 152.7167},
	{Name: "Bellingen", Latitude: -30.4500, Longitude: 152.9000},
	{Name: "Nambucca Heads", Latitude: -30.6500, Longitude: 153.0000},
	{Name: "Macksville", Latitude: -30.7000, Longitude: 152.9167},
	{Name: "South West Rocks", Latitude: -30.8833, Longitude: 153.0333},
	{Name: "Wauchope", Latitude: -31.4500, Longitude: 152.7333},
	{Name: "Laurieton", Latitude: -31.6500, Longitude: 152.8000},
	{Name: "Gloucester", Latitude: -32.0000, Longitude: 151.9667},
	{Name: "Dungog", Latitude: -32.4000, Longitude: 151.7500},
	{Name: "Stroud", Latitude: -32.4000, Longitude: 151.9667},
	{Name: "Bulahdelah", Latitude: -32.4167, Longitude: 152.2167},
	{Name: "Tea Gardens", Latitude: -32.6667, Longitude: 152.1500},
	{Name: "Nabiac", Latitude: -32.1000, Longitude: 152.3667},
	{Name: "Wingham", Latitude: -31.8667, Longitude: 152.3667},
	{Name: "Old Bar", Latitude: -31.9667, Longitude: 152.5833},
	{Name: "Harrington", Latitude: -31.8667, Longitude: 152.6833},
	{Name: "Maclean", Latitude: -29.4500, Longitude: 153.2000},
	{Name: "Yamba", Latitude: -29.4333, Longitude: 153.3500},
	{Name: "Iluka", Latitude: -29.4000, Longitude: 153.3500},
	{Name: "Evans Head", Latitude: -29.1167, Longitude: 153.4333},
	{Name: "Woodburn", Latitude: -29.0667, Longitude: 153.3500},
	{Name: "Coraki", Latitude: -28.9833, Longitude: 153.2833},
	{Name: "Kyogle", Latitude: -28.6167, Longitude: 153.0000},
	{Name: "Nimbin", Latitude: -28.5833, Longitude: 153.2167},
	{Name: "Mullumbimby", Latitude: -28.5500, Longitude: 153.5000},
	{Name: "Brunswick Heads", Latitude: -28.5333, Longitude: 153.5500},
	{Name: "Murwillumbah", Latitude: -28.3333, Longitude: 153.4000},
	{Name: "Uki", Latitude: -28.4167, Longitude: 153.3500},
	{Name: "Oberon", Latitude: -33.7000, Longitude: 149.8500},
	{Name: "Rylstone", Latitude: -32.8000, Longitude: 149.9667},
	{Name: "Kandos", Latitude: -32.8500, Longitude: 149.9667},
	{Name: "Gulgong", Latitude: -32.3667, Longitude: 149.5333},
	{Name: "Dunedoo", Latitude: -32.0167, Longitude: 149.3833},
	{Name: "Mendooran", Latitude: -31.8167, Longitude: 149.1167},
	{Name: "Coolah", Latitude: -31.8333, Longitude: 149.7167},
	{Name: "Cassilis", Latitude: -32.0167, Longitude: 149.9833},
	{Name: "Merriwa", Latitude: -32.1333, Longitude: 150.3500},
	{Name: "Scone", Latitude: -32.0500, Longitude: 150.8667},
	{Name: "Aberdeen", Latitude: -32.1667, Longitude: 150.9000},
	{Name: "Murrurundi", Latitude: -31.7667, Longitude: 150.8333},
	{Name: "Willow Tree", Latitude: -31.6500, Longitude: 150.7333},
	{Name: "Blandford", Latitude: -31.8000, Longitude: 150.8833},
	{Name: "Denman", Latitude: -32.3833, Longitude: 150.6833},
	{Name: "Jerry Plains", Latitude: -32.5000, Longitude: 150.9000},
	{Name: "Broke", Latitude: -32.7500, Longitude: 151.1000},
	{Name: "Pokolbin", Latitude: -32.7833, Longitude: 151.2833},
	{Name: "Kurri Kurri", Latitude: -32.8167, Longitude: 151.4833},
	{Name: "Branxton", Latitude: -32.6500, Longitude: 151.3500},
	{Name: "Paterson", Latitude: -32.6167, Longitude: 151.5833},
	{Name: "Morpeth", Latitude: -32.7333, Longitude: 151.6333},
	{Name: "Clarence Town", Latitude: -32.5833, Longitude: 151.7833},
	{Name: "Karuah", Latitude: -32.6500, Longitude: 151.9667},
	{Name: "Moruya", Latitude: -35.9167, Longitude: 150.0833},
	{Name: "Narooma", Latitude: -36.2167, Longitude: 150.0667},
	{Name: "Bermagui", Latitude: -36.4167, Longitude: 150.0667},
	{Name: "Tathra", Latitude: -36.7333, Longitude: 149.9833},
	{Name: "Pambula", Latitude: -36.9333, Longitude: 149.8833},
	{Name: "Cobargo", Latitude: -36.3833, Longitude: 149.8833},
	{Name: "Candelo", Latitude: -36.7667, Longitude: 149.6833},
	{Name: "Bemboka", Latitude: -36.6333, Longitude: 149.5667},
	{Name: "Nimmitabel", Latitude: -36.5167, Longitude: 149.2833},
	{Name: "Captains Flat", Latitude: -35.5833, Longitude: 149.4500},
	{Name: "Bungendore", Latitude: -35.2500, Longitude: 149.4500},
	{Name: "Murrumbateman", Latitude: -34.9667, Longitude: 149.0333},
	{Name: "Sutton", Latitude: -35.1333, Longitude: 149.2500},
	{Name: "Collector", Latitude: -34.9167, Longitude: 149.4333},
	{Name: "Tarago", Latitude: -35.0667, Longitude: 149.6500},
	{Name: "Marulan", Latitude: -34.7000, Longitude: 150.0000},
	{Name: "Bundanoon", Latitude: -34.6500, Longitude: 150.3000},
	{Name: "Mittagong", Latitude: -34.4500, Longitude: 150.4500},
	{Name: "Bowral", Latitude: -34.4833, Longitude: 150.4167},
	{Name: "Moss Vale", Latitude: -34.5500, Longitude: 150.3833},
	{Name: "Robertson", Latitude: -34.5833, Longitude: 150.5833},
	{Name: "Berry", Latitude: -34.7667, Longitude: 150.6833},
	{Name: "Kangaroo Valley", Latitude: -34.7333, Longitude: 150.5333},
	{Name: "Ulladulla", Latitude: -35.3500, Longitude: 150.4667},
	{Name: "Milton", Latitude: -35.3167, Longitude: 150.4333},
	{Name: "Sussex Inlet", Latitude: -35.1500, Longitude: 150.5833},
	{Name: "Shoalhaven Heads", Latitude: -34.8500, Longitude: 150.7500},
	{Name: "Huskisson", Latitude: -35.0333, Longitude: 150.6667},
	{Name: "Vincentia", Latitude: -35.0667, Longitude: 150.6667},
	{Name: "Batemans Bay", Latitude: -35.7167, Longitude: 150.1833},
	{Name: "Mogo", Latitude: -35.7833, Longitude: 150.1333},
}

// FindNearestTown finds the nearest town to a given location
func FindNearestTown(lat, lng float64) (Location, float64) {
	var nearest Location
	minDist := math.MaxFloat64

	for _, town := range NSWTowns {
		dist := Haversine(lat, lng, town.Latitude, town.Longitude)
		if dist < minDist {
			minDist = dist
			nearest = town
		}
	}

	return nearest, minDist
}

// NearestTownResult contains info about a nearby town
type NearestTownResult struct {
	Name       string
	DistanceKm float64
}

// FindTwoNearestTowns finds the two nearest towns to a given location
func FindTwoNearestTowns(lat, lng float64) (NearestTownResult, NearestTownResult) {
	var first, second NearestTownResult
	first.DistanceKm = math.MaxFloat64
	second.DistanceKm = math.MaxFloat64

	for _, town := range NSWTowns {
		dist := Haversine(lat, lng, town.Latitude, town.Longitude)
		if dist < first.DistanceKm {
			// Current first becomes second
			second = first
			// New first
			first = NearestTownResult{Name: town.Name, DistanceKm: dist}
		} else if dist < second.DistanceKm {
			second = NearestTownResult{Name: town.Name, DistanceKm: dist}
		}
	}

	return first, second
}

// DistanceToSydney calculates distance from a point to Sydney CBD
func DistanceToSydney(lat, lng float64) float64 {
	return Haversine(lat, lng, Sydney.Latitude, Sydney.Longitude)
}
