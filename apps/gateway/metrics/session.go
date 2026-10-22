package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	labelPlatform = "platform"
	labelVersion  = "version"

	// Versions past this many share one label so a forged user agent cannot grow the series count.
	maxAppVersions  = 50
	OtherAppVersion = "other"
)

type AppPlatform string

const (
	AppPlatformIOS     AppPlatform = "ios"
	AppPlatformAndroid AppPlatform = "android"
)

type SessionMetrics struct {
	sessions *prometheus.CounterVec

	mu       sync.Mutex
	versions map[string]struct{}
}

func NewSessionMetrics(registry prometheus.Registerer) *SessionMetrics {
	m := &SessionMetrics{
		sessions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "app_sessions_total",
			Help:      "App launches, counted by the version check each launch makes, by platform and app version.",
		}, []string{labelPlatform, labelVersion}),
		versions: map[string]struct{}{},
	}
	registry.MustRegister(m.sessions)
	return m
}

func (m *SessionMetrics) Started(platform AppPlatform, version string) {
	m.sessions.WithLabelValues(string(platform), m.versionLabel(version)).Inc()
}

func (m *SessionMetrics) versionLabel(version string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, seen := m.versions[version]; seen {
		return version
	}
	if len(m.versions) >= maxAppVersions {
		return OtherAppVersion
	}
	m.versions[version] = struct{}{}
	return version
}
