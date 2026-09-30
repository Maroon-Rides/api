package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db/notify"
)

func ListenForLiveData(lc fx.Lifecycle, db *bun.DB, websocket *services.WebsocketService) {
	notify.ListenOnStart(lc, db, notify.Handlers{
		notify.LiveDataAvailableChannel: routeHandler(websocket.LiveDataAvailable),
		notify.VehiclesChannel:          routeHandler(websocket.VehiclesChanged),
		notify.DeparturesChannel:        routeHandler(websocket.DeparturesChanged),
	})
}

func routeHandler(handle func(context.Context, uuid.UUID) error) func(context.Context, string) error {
	return func(ctx context.Context, payload string) error {
		routeID, err := uuid.Parse(payload)
		if err != nil {
			return fmt.Errorf("parse route id: %w", err)
		}
		return handle(ctx, routeID)
	}
}
