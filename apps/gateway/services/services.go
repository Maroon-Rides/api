package services

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/gateway/metrics"
	"github.com/MaroonRides/api/apps/gateway/repositories"
)

var Module = fx.Provide(
	func(lc fx.Lifecycle, repo *repositories.LiveDataRepository, metrics *metrics.WebsocketMetrics) *WebsocketService {
		return NewWebsocketService(lc, repo, metrics)
	},
	NewSyncService,
)
