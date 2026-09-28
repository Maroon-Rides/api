package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/go-fuego/fuego"
	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/fx"
)

const (
	websocketRoute  = "/ws/"
	websocketPath   = "/api/ws"
	routeRoomPrefix = "route:"
)

type WebsocketController struct {
	svc *services.WebsocketService
	io  *socket.Server
}

func NewWebsocketController(lc fx.Lifecycle, svc *services.WebsocketService) *WebsocketController {
	opts := socket.DefaultServerOptions()
	opts.SetPath(websocketPath)
	opts.SetTransports(types.NewSet("websocket"))

	c := &WebsocketController{svc: svc, io: socket.NewServer(nil, opts)}
	c.io.On("connection", func(args ...any) {
		c.handleConnection(args[0].(*socket.Socket))
	})

	adapter := c.io.Sockets().Adapter()
	adapter.On("create-room", func(args ...any) {
		if routeID, ok := routeIDFromRoom(args[0].(socket.Room)); ok {
			c.svc.RouteRoomCreated(routeID)
		}
	})
	adapter.On("delete-room", func(args ...any) {
		if routeID, ok := routeIDFromRoom(args[0].(socket.Room)); ok {
			c.svc.RouteRoomDeleted(routeID)
		}
	})

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			c.io.Close(nil)
			return nil
		},
	})

	return c
}

func (c *WebsocketController) Register(api *fuego.Server) {
	fuego.Handle(api, websocketRoute, c.io.ServeHandler(nil), fuego.OptionHide())
}

func (c *WebsocketController) Broadcast(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	c.io.Emit(event, string(data))
	return nil
}

func (c *WebsocketController) handleConnection(client *socket.Socket) {
	clientID := string(client.Id())
	c.svc.ClientConnected(clientID)

	client.On("disconnect", func(args ...any) {
		c.svc.ClientDisconnected(clientID, fmt.Sprint(args...))
	})
}

func routeRoom(routeID string) socket.Room {
	return socket.Room(routeRoomPrefix + routeID)
}

func routeIDFromRoom(room socket.Room) (string, bool) {
	return strings.CutPrefix(string(room), routeRoomPrefix)
}
