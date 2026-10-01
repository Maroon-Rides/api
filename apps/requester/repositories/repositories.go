package repositories

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/apps/requester/repositories/gtfs"
)

var Module = fx.Options(
	fx.Provide(
		NewRouteDataRepository,
		NewTimetableRepository,
		func() *busapi.Client { return busapi.NewClient(busapi.ClientConfig{}) },
		func() *gtfs.Client { return gtfs.NewClient(gtfs.ClientConfig{}) },
	),
)
