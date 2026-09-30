package repositories

import "go.uber.org/fx"

var Module = fx.Provide(
	NewSyncRepository,
	NewLiveDataRepository,
)
