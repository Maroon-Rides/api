package services

import (
	"regexp"

	"github.com/MaroonRides/api/apps/gateway/metrics"
)

// The app sets its user agent in capacitor.config.ts, e.g. "Maroon Rides App (iOS) v3.0.0".
var appUserAgent = regexp.MustCompile(`Maroon Rides App \((iOS|Android)\) v(\d+\.\d+\.\d+)`)

var appPlatforms = map[string]metrics.AppPlatform{
	"iOS":     metrics.AppPlatformIOS,
	"Android": metrics.AppPlatformAndroid,
}

type SessionService struct {
	metrics *metrics.SessionMetrics
}

func NewSessionService(m *metrics.SessionMetrics) *SessionService {
	return &SessionService{metrics: m}
}

// Started ignores anything that is not the app, since the kubelet probes the same endpoint.
func (s *SessionService) Started(userAgent string) {
	match := appUserAgent.FindStringSubmatch(userAgent)
	if match == nil {
		return
	}
	s.metrics.Started(appPlatforms[match[1]], match[2])
}
