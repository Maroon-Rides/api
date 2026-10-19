package repositories

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/internal/busapi"
)

var Module = fx.Provide(
	NewSyncRepository,
	NewLiveDataRepository,
	NewTripPlanRepository,
	func() *busapi.Client { return busapi.NewClient(busapi.ClientConfig{}) },
)
