package utils

// A browser cannot set its user agent, so the app names itself in these headers on every platform.
const (
	AppPlatformHeader = "X-App-Platform"
	AppVersionHeader  = "X-App-Version"
)
