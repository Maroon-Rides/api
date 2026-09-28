package utils

import (
	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/samber/lo"
)

var AmenityNameMap = map[string]string{
	"Air Conditioning":      "AIR_CONDITIONING",
	"Wheelchair Accessible": "WHEELCHAIR_ACCESSIBLE",
	"Wheelchair Lift":       "WHEELCHAIR_LIFT",
	"Bicycle Rack":          "BICYCLE_RACK",
	"Shelter":               "SHELTER",
	"Time Point":            "TIME_POINT",
}

func AmenityNames(amenities []busapi.Amenity) []string {
	return lo.Map(amenities, func(a busapi.Amenity, _ int) string {
		return AmenityNameMap[a.Name]
	})
}
