package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-fuego/fuego"
)

// fuego writes any other error to the response as JSON, which exposes the exported fields of
// errors like *pgconn.PgError. Those become a bare 500, and only the log sees the cause.
func handleError(ctx context.Context, err error) error {
	var withStatus fuego.ErrorWithStatus
	if errors.As(err, &fuego.HTTPError{}) || errors.As(err, &withStatus) {
		return fuego.HandleHTTPError(ctx, err)
	}
	return fuego.HandleHTTPError(ctx, fuego.InternalServerError{Status: http.StatusInternalServerError, Err: err})
}
