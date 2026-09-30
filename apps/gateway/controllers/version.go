package controllers

import (
	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/config"
	"github.com/MaroonRides/api/apps/gateway/services"
)

type VersionController struct {
	svc *services.SyncService
}

func NewVersionController(svc *services.SyncService) *VersionController {
	return &VersionController{svc: svc}
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

func (c *VersionController) supportedVersion(fc fuego.ContextNoBody) (SupportedVersionResponse, error) {
	return SupportedVersionResponse{
		Version: config.MinimumSupportedVersion,
	}, nil
}
