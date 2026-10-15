package repositories

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/requester/metrics"
	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/apps/requester/repositories/gtfs"
)

const (
	upstreamBusAPI = "busapi"
	upstreamGTFS   = "gtfs"
)

var Module = fx.Options(
	fx.Provide(
		NewRouteDataRepository,
		NewTimetableRepository,
		func(m *metrics.UpstreamMetrics) *busapi.Client {
			return busapi.NewClient(busapi.ClientConfig{WrapTransport: m.Transport(upstreamBusAPI)})
		},
		func(m *metrics.UpstreamMetrics) *gtfs.Client {
			return gtfs.NewClient(gtfs.ClientConfig{WrapTransport: m.Transport(upstreamGTFS)})
		},
	),
)
