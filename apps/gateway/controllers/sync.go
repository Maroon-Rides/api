package controllers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
)

const JSONLinesContentType = "application/jsonlines+json"

type SyncController struct {
	svc *services.SyncService
}

func NewSyncController(svc *services.SyncService) *SyncController {
	return &SyncController{svc: svc}
}

func (c *SyncController) Register(api *fuego.Server) {
	fuego.Post(api, "/sync/stream", c.stream,
		fuego.OptionOperationID("syncStream"),
		fuego.OptionSummary("Stream sync changes"),
		fuego.OptionDescription("Streams every change after the given acks as JSON lines. Each line is one SyncStreamLine."),
		fuego.OptionAddResponse(http.StatusOK, "One SyncStreamLine per line", fuego.Response{
			Type:         dtos.SyncStreamLine{},
			ContentTypes: []string{JSONLinesContentType},
		}),
	)
	registerSyncStreamLine(api.OpenAPI)
}

func (c *SyncController) stream(fc fuego.ContextWithBody[dtos.SyncRequest]) (any, error) {
	req, err := fc.Body()
	if err != nil {
		return nil, err
	}

	plan, err := c.svc.Plan(req)
	if errors.Is(err, services.ErrInvalidSyncRequest) {
		return nil, fuego.BadRequestError{Detail: err.Error(), Err: err}
	}
	if err != nil {
		return nil, err
	}

	w := newLineWriter(fc.Response())
	err = c.svc.Stream(fc.Context(), plan, w.send)
	if err != nil && !w.started {
		return nil, err
	}
	if err != nil {
		slog.Warn("sync stream ended early", "error", err)
	}
	return nil, nil
}

// lineWriter holds the status line back until the first line, so an error before any data is still a proper error response.
type lineWriter struct {
	w       http.ResponseWriter
	enc     *json.Encoder
	started bool
}

func newLineWriter(w http.ResponseWriter) *lineWriter {
	return &lineWriter{w: w, enc: json.NewEncoder(w)}
}

func (l *lineWriter) send(line dtos.SyncStreamLine) error {
	if !l.started {
		l.w.Header().Set("Content-Type", JSONLinesContentType)
		l.w.WriteHeader(http.StatusOK)
		l.started = true
	}
	return l.enc.Encode(line)
}
