package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/MaroonRides/api/apps/gateway/config"
	"github.com/MaroonRides/api/apps/gateway/controllers"

	"github.com/go-fuego/fuego"
	"github.com/rs/cors"
	"go.uber.org/fx"
)

type Controllers struct {
	fx.In
	All []controllers.Controller `group:"controllers"`
}

func corsMiddleware(next http.Handler) http.Handler {
	return cors.New(cors.Options{AllowedOrigins: config.AllowedOrigins}).Handler(next)
}

func registerAPI(server *fuego.Server, all []controllers.Controller) {
	server.OpenAPI.SetGeneratorSchemaCustomizer(schemaCustomizer)

	api := fuego.Group(server, "/api")
	for _, controller := range all {
		controller.Register(api)
		slog.Info(fmt.Sprintf("Registered controller: %T", controller))
	}

	sanitizeOpenAPISpec(server.OpenAPI.Description())
}

func NewServer(lc fx.Lifecycle, controllers Controllers) *fuego.Server {
	addr := ":" + config.Port()

	server := fuego.NewServer(
		fuego.WithGlobalMiddlewares(corsMiddleware),
		fuego.WithLoggingMiddleware(fuego.LoggingConfig{
			DisableRequest:  true,
			DisableResponse: true,
		}),
		fuego.WithSerializer(fuego.SendJSON),
		fuego.WithEngineOptions(
			fuego.WithRequestContentType("application/json"),
			fuego.WithResponseContentType("application/json"),
		),
		fuego.WithAddr(addr),
	)

	server.OpenAPI.Config.JSONFilePath = "openapi.json"
	server.OpenAPI.Config.SpecURL = "/openapi.json"
	server.OpenAPI.Config.DisableSwaggerUI = true
	server.OpenAPI.Config.DisableDefaultServer = true
	server.OpenAPI.Config.PrettyFormatJSON = true

	registerAPI(server, controllers.All)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			openapi := flag.Bool("open-api", false, "Generate the OpenAPI spec and exit")
			flag.Parse()

			if *openapi {
				server.OutputOpenAPISpec()
				slog.Info("OpenAPI spec generated successfully.")
				os.Exit(0)
			}

			go func() {
				if err := server.Run(); err != nil {
					slog.Error("Failed to start server", "error", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})

	return server
}
