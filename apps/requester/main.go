package main

import (
	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/apps/requester/jobs"
	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/internal/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.NopLogger,
		jobs.Module,
		busapi.Module,
		repositories.Module,
		db.Module,
		db.MigrateOnStart,
	)

	app.Run()
}
