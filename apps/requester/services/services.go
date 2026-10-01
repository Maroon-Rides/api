package services

import (
	"fmt"
	"time"

	_ "time/tzdata"

	"go.uber.org/fx"
)

// ServiceTimeZone is where the buses run. The alpine image ships no zoneinfo,
// hence the embedded tzdata.
const ServiceTimeZone = "America/Chicago"

var Module = fx.Provide(
	NewServiceLocation,
	NewRouteDataService,
	NewLiveDataService,
	NewTimetableService,
	NewTimepointService,
	NewDatabaseCleanupService,
)

func NewServiceLocation() (*time.Location, error) {
	location, err := time.LoadLocation(ServiceTimeZone)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", ServiceTimeZone, err)
	}
	return location, nil
}
