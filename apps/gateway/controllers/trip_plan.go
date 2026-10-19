package controllers

import (
	"errors"
	"net/http"

	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
)

type TripPlanController struct {
	svc *services.TripPlanService
}

func NewTripPlanController(svc *services.TripPlanService) *TripPlanController {
	return &TripPlanController{svc: svc}
}

func (c *TripPlanController) Register(api *fuego.Server) {
	fuego.Post(api, "/trip-plan", c.plan,
		fuego.OptionOperationID("planTrip"),
		fuego.OptionSummary("Plan a trip between two coordinates"),
		fuego.OptionAddError(http.StatusBadGateway, "The upstream trip planner failed"),
	)
}

func (c *TripPlanController) plan(fc fuego.ContextWithBody[dtos.TripPlanRequest]) (dtos.TripPlanResponse, error) {
	req, err := fc.Body()
	if err != nil {
		return dtos.TripPlanResponse{}, err
	}

	res, err := c.svc.Plan(fc.Context(), req)
	if errors.Is(err, services.ErrTripPlannerUnavailable) {
		return res, fuego.HTTPError{Status: http.StatusBadGateway, Detail: err.Error(), Err: err}
	}
	return res, err
}
