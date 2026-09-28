package main

import (
	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.NopLogger,
		controllers.Module,
		services.Module,
		repositories.Module,
		db.Module,

		fx.Invoke(NewServer),
	)

	app.Run()
}
