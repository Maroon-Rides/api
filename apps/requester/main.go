package main

import (
	"github.com/MaroonRides/api/apps/requester/jobs"
	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/services"
	"github.com/MaroonRides/api/internal/db"
	"go.uber.org/fx"
)

var app = fx.Options(
	jobs.Module,
	repositories.Module,
	services.Module,
	db.Module,
	db.MigrateOnStart,
	fx.Provide(NewDBListeners),
	fx.Invoke(func(*DBListeners) {}),
)

func main() {
	fx.New(fx.NopLogger, app).Run()
}
