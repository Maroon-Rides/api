package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-fuego/fuego"
	"github.com/klauspost/compress/gzhttp"

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
		fuego.OptionMiddleware(gzipped),
	)
	registerSyncStreamLine(api.OpenAPI)
}

func gzipped(next http.Handler) http.Handler {
	return gzhttp.GzipHandler(next)
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

	// The whole response is read before any of it is written, so a slow client never holds a database connection.
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	if err := c.svc.Stream(fc.Context(), plan, func(line dtos.SyncStreamLine) error { return enc.Encode(line) }); err != nil {
		return nil, err
	}

	w := fc.Response()
	w.Header().Set("Content-Type", JSONLinesContentType)
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(http.StatusOK)
	if _, err := body.WriteTo(w); err != nil {
		slog.Warn("sync stream ended early", "error", err)
	}
	return nil, nil
}
