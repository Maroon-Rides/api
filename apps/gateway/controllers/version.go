package controllers

import (
	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/config"
	"github.com/MaroonRides/api/apps/gateway/services"
)

type VersionController struct {
	sessions *services.SessionService
}

func NewVersionController(sessions *services.SessionService) *VersionController {
	return &VersionController{sessions: sessions}
}

func (c *VersionController) Register(api *fuego.Server) {
	fuego.Get(api, "/version/supported", c.supportedVersion,
		fuego.OptionOperationID("getSupportedVersion"),
		fuego.OptionSummary("Get the minimum supported version that the server supports communicating with"),
	)
}

type SupportedVersionResponse struct {
	Version string `json:"version"`
}

// The app checks its version once per launch, so each call is one session.
func (c *VersionController) supportedVersion(fc fuego.ContextNoBody) (SupportedVersionResponse, error) {
	c.sessions.Started(fc.Request().UserAgent())
	return SupportedVersionResponse{
		Version: config.MinimumSupportedVersion,
	}, nil
}
