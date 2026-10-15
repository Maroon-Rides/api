package main

import (
	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/metrics"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db"
	internalmetrics "github.com/MaroonRides/api/internal/metrics"
	"go.uber.org/fx"
)

var app = fx.Options(
	controllers.Module,
	services.Module,
	repositories.Module,
	metrics.Module,
	db.Module,
	internalmetrics.Module,

	fx.Invoke(NewServer),
	fx.Invoke(ListenForLiveData),
)

func main() {
	fx.New(fx.NopLogger, app).Run()
}
