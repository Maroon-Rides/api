package services

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/gateway/repositories"
)

var Module = fx.Provide(
	fx.Annotate(NewWebsocketService, fx.From(new(fx.Lifecycle), new(*repositories.LiveDataRepository))),
	NewSyncService,
)
