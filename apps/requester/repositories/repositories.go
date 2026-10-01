package repositories

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
)

var Module = fx.Options(
	fx.Provide(
		NewRouteDataRepository,
		NewTimetableRepository,
		func() *busapi.Client { return busapi.NewClient(busapi.ClientConfig{}) },
	),
)
