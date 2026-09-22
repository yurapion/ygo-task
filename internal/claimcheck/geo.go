package claimcheck

import "math"

// knownCities is a fixed lookup, not a geocoder. It is enough to turn a
// coordinate pair into a claim that can be compared against a stated city,
// which is the whole job here; a feed naming a city outside this table gets no
// derived claim rather than a wrong one.
var knownCities = []struct {
	Name string
	Lat  float64
	Lng  float64
}{
	{"Berlin", 52.5200, 13.4050},
	{"Hamburg", 53.5511, 9.9937},
	{"Munich", 48.1351, 11.5820},
	{"Innsbruck", 47.2692, 11.4041},
	{"Rimini", 44.0678, 12.5695},
	{"Playa del Carmen", 20.6296, -87.0739},
}

// derivedCityRadiusKM is how close a coordinate must be to a known city before
// we are willing to assert that city. A hotel is not always in the centre, but
// it is not 50km outside it either.
const derivedCityRadiusKM = 50

// nearestCity returns the known city containing these coordinates, or false if
// none is close enough to assert honestly.
func nearestCity(c Coords) (string, bool) {
	best, bestKM := "", math.Inf(1)
	for _, city := range knownCities {
		km := haversineKM(c.Lat, c.Lng, city.Lat, city.Lng)
		if km < bestKM {
			best, bestKM = city.Name, km
		}
	}
	if bestKM > derivedCityRadiusKM {
		return "", false
	}
	return best, true
}

// haversineKM is great-circle distance in kilometres.
func haversineKM(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKM = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
