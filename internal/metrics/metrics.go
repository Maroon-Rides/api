// Package metrics serves Prometheus metrics on a port kept apart from public traffic.
package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
)

const (
	envPort     = "METRICS_PORT"
	defaultPort = "9090"
	route       = "/metrics"

	readHeaderTimeout = 5 * time.Second
)

var Module = fx.Options(
	fx.Provide(
		NewRegistry,
		func(r *prometheus.Registry) prometheus.Registerer { return r },
	),
	fx.Invoke(Serve),
)

func Port() string {
	if v := os.Getenv(envPort); v != "" {
		return v
	}
	return defaultPort
}

func NewRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registry
}

func Serve(lc fx.Lifecycle, registry *prometheus.Registry) {
	mux := http.NewServeMux()
	mux.Handle(route, promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))
	server := &http.Server{Addr: ":" + Port(), Handler: mux, ReadHeaderTimeout: readHeaderTimeout}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			listener, err := net.Listen("tcp", server.Addr)
			if err != nil {
				return err
			}
			go func() {
				if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
					slog.Error("Metrics server stopped", "error", err)
				}
			}()
			return nil
		},
		OnStop: server.Shutdown,
	})
}
