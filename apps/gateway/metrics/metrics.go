package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

const (
	namespace = "gateway"

	labelRoute  = "route"
	labelStatus = "status"
	labelKind   = "kind"

	// Requests that match no route share one label so stray paths cannot grow the series count.
	unmatchedRoute = "unmatched"
)

var Module = fx.Provide(NewHTTPMetrics, NewWebsocketMetrics, NewSyncMetrics)

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
	Clients            prometheus.Gauge
	SubscribedRoutes   prometheus.Gauge
	RouteClients       *prometheus.GaugeVec
	RouteSubscriptions *prometheus.CounterVec
	MessagesSent       prometheus.Counter
	SendFailures       prometheus.Counter
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
		RouteClients: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "websocket_route_clients",
			Help:      "Websocket clients subscribed, by route.",
		}, []string{labelRoute}),
		RouteSubscriptions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_route_subscriptions_total",
			Help:      "Websocket subscriptions made, by route.",
		}, []string{labelRoute}),
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
	registry.MustRegister(m.Clients, m.SubscribedRoutes, m.RouteClients, m.RouteSubscriptions, m.MessagesSent, m.SendFailures)
	return m
}

type SyncKind string

const (
	// The client sent no acks, so it holds nothing yet.
	SyncKindFull SyncKind = "full"
	// The client's acks were too old to trust, so it was told to start over.
	SyncKindReset SyncKind = "reset"
	// The client got only what changed since its acks.
	SyncKindIncremental SyncKind = "incremental"
)

// Clients older than a month are reset, so nothing past 30 days needs its own bucket.
var syncIntervalBuckets = []float64{
	time.Minute.Seconds(),
	(5 * time.Minute).Seconds(),
	(15 * time.Minute).Seconds(),
	time.Hour.Seconds(),
	(3 * time.Hour).Seconds(),
	(6 * time.Hour).Seconds(),
	(12 * time.Hour).Seconds(),
	(24 * time.Hour).Seconds(),
	(2 * 24 * time.Hour).Seconds(),
	(7 * 24 * time.Hour).Seconds(),
	(14 * 24 * time.Hour).Seconds(),
	(30 * 24 * time.Hour).Seconds(),
}

type SyncMetrics struct {
	syncs    *prometheus.CounterVec
	interval prometheus.Histogram
}

func NewSyncMetrics(registry prometheus.Registerer) *SyncMetrics {
	m := &SyncMetrics{
		syncs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "syncs_total",
			Help:      "Sync streams served, by kind.",
		}, []string{labelKind}),
		interval: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sync_interval_seconds",
			Help:      "Time since the client's previous completed sync.",
			Buckets:   syncIntervalBuckets,
		}),
	}
	registry.MustRegister(m.syncs, m.interval)
	return m
}

// Synced takes the zero time when the client has never completed a sync.
func (m *SyncMetrics) Synced(kind SyncKind, previous, now time.Time) {
	m.syncs.WithLabelValues(string(kind)).Inc()
	if !previous.IsZero() {
		m.interval.Observe(now.Sub(previous).Seconds())
	}
}
