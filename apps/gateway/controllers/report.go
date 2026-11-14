package controllers

import (
	"errors"
	"net/http"

	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
)

const maxReportBodyBytes = 256 << 10

type ReportController struct {
	svc *services.ReportService
}

func NewReportController(svc *services.ReportService) *ReportController {
	return &ReportController{svc: svc}
}

func (c *ReportController) Register(api *fuego.Server) {
	fuego.Post(api, "/reports", c.report,
		fuego.OptionOperationID("report"),
		fuego.OptionSummary("Report client events"),
		fuego.OptionDescription("Records each event. An event with an unknown type or a bad payload is dropped without failing the rest."),
		fuego.OptionDefaultStatusCode(http.StatusNoContent),
		fuego.OptionMiddleware(limitBody),
	)
	registerReportEvent(api.OpenAPI)
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxReportBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func (c *ReportController) report(fc fuego.ContextWithBody[dtos.ReportRequest]) (any, error) {
	req, err := fc.Body()
	if err != nil {
		return nil, err
	}

	err = c.svc.Record(req, fc.Request().UserAgent())
	if errors.Is(err, services.ErrInvalidReport) {
		return nil, fuego.BadRequestError{Detail: err.Error(), Err: err}
	}
	return nil, err
}
