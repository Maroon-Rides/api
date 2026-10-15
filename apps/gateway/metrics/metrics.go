package metrics

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

const (
	namespace = "gateway"

	labelRoute  = "route"
	labelStatus = "status"

	// Requests that match no route share one label so stray paths cannot grow the series count.
	unmatchedRoute = "unmatched"
)

var Module = fx.Provide(NewHTTPMetrics, NewWebsocketMetrics)

type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewHTTPMetrics(registry prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "HTTP requests served, by route and status.",
		}, []string{labelRoute, labelStatus}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "Time to serve an HTTP request, by route.",
			Buckets:   prometheus.DefBuckets,
		}, []string{labelRoute}),
	}
	registry.MustRegister(m.requests, m.duration)
	return m
}

// Middleware skips websocket upgrades, whose duration is the life of the connection.
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}

		captured := httpsnoop.CaptureMetrics(next, w, r)

		route := r.Pattern
		if route == "" {
			route = unmatchedRoute
		}
		m.requests.WithLabelValues(route, strconv.Itoa(captured.Code)).Inc()
		m.duration.WithLabelValues(route).Observe(captured.Duration.Seconds())
	})
}

type WebsocketMetrics struct {
	Clients          prometheus.Gauge
	SubscribedRoutes prometheus.Gauge
	MessagesSent     prometheus.Counter
	SendFailures     prometheus.Counter
}

func NewWebsocketMetrics(registry prometheus.Registerer) *WebsocketMetrics {
	m := &WebsocketMetrics{
		Clients: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "websocket_clients",
			Help:      "Websocket clients connected.",
		}),
		SubscribedRoutes: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "websocket_subscribed_routes",
			Help:      "Routes with at least one websocket client.",
		}),
		MessagesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_messages_sent_total",
			Help:      "Live data messages queued for websocket clients.",
		}),
		SendFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_send_failures_total",
			Help:      "Live data messages dropped because the client was closed or too slow.",
		}),
	}
	registry.MustRegister(m.Clients, m.SubscribedRoutes, m.MessagesSent, m.SendFailures)
	return m
}
