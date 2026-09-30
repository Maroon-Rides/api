package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/requester/services"
	"github.com/MaroonRides/api/internal/db/notify"
)

type DBListeners struct {
	liveData *services.LiveDataService
}

func NewDBListeners(lc fx.Lifecycle, db *bun.DB, liveData *services.LiveDataService) *DBListeners {
	l := &DBListeners{liveData: liveData}

	notify.ListenOnStart(lc, db, notify.Handlers{
		notify.LiveDataSubscriptionsChannel: l.handleLiveDataSubscription,
	})

	return l
}

func (l *DBListeners) handleLiveDataSubscription(ctx context.Context, payload string) error {
	routeID, err := uuid.Parse(payload)
	if err != nil {
		return fmt.Errorf("parse route id: %w", err)
	}
	return l.liveData.SubscriptionAdded(ctx, routeID)
}
