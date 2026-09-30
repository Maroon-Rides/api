package config

import "os"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Port() string {
	return env("PORT", "3000")
}

const MinimumSupportedVersion = "3.0.0"
