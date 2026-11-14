package services

import (
	"regexp"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/metrics"
)

var appVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Values of the X-App-Platform header, as Capacitor's getPlatform() returns them.
var headerPlatforms = map[string]metrics.AppPlatform{
	"ios":     metrics.AppPlatformIOS,
	"android": metrics.AppPlatformAndroid,
	"web":     metrics.AppPlatformWeb,
}

// Builds from before the app sent its own headers only name themselves here, set in capacitor.config.ts, e.g. "Maroon Rides App (iOS) v3.0.0".
var appUserAgent = regexp.MustCompile(`Maroon Rides App \((iOS|Android)\) v(\d+\.\d+\.\d+)`)

var userAgentPlatforms = map[string]metrics.AppPlatform{
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
func (s *SessionService) Started(client dtos.AppClient) {
	platform, version, ok := identifyApp(client)
	if !ok {
		return
	}
	s.metrics.Started(platform, version)
}

func identifyApp(client dtos.AppClient) (metrics.AppPlatform, string, bool) {
	platform, known := headerPlatforms[client.Platform]
	if known && appVersion.MatchString(client.Version) {
		return platform, client.Version, true
	}

	match := appUserAgent.FindStringSubmatch(client.UserAgent)
	if match == nil {
		return "", "", false
	}
	return userAgentPlatforms[match[1]], match[2], true
}
