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

// Major NSW towns with population > 5000
// Source: ABS 2021 Census
var NSWTowns = []Location{
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
	{Name: "Cessnock", Latitude: -32.8333, Longitude: 151.3500},
	{Name: "Grafton", Latitude: -29.6833, Longitude: 152.9333},
	{Name: "Lake Macquarie", Latitude: -33.0833, Longitude: 151.5833},
	{Name: "Shellharbour", Latitude: -34.5833, Longitude: 150.8667},
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

// DistanceToSydney calculates distance from a point to Sydney CBD
func DistanceToSydney(lat, lng float64) float64 {
	return Haversine(lat, lng, Sydney.Latitude, Sydney.Longitude)
}
