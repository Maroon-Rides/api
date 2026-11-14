package utils

import (
	"net/http"

	"github.com/MaroonRides/api/apps/gateway/dtos"
)

// A browser cannot set its user agent, so the app names itself in these headers on every platform.
const (
	appPlatformHeader = "X-App-Platform"
	appVersionHeader  = "X-App-Version"
)

func AppClient(r *http.Request) dtos.AppClient {
	return dtos.AppClient{
		Platform:  r.Header.Get(appPlatformHeader),
		Version:   r.Header.Get(appVersionHeader),
		UserAgent: r.UserAgent(),
	}
}
