package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/MaroonRides/api/apps/gateway/config"
	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
)

const (
	websocketRoute = "/ws"

	// A client that sends nothing, not even a ping, for this long is dropped.
	clientIdleTimeout  = 60 * time.Second
	clientWriteTimeout = 10 * time.Second
	clientOutboxSize   = 32
)

var (
	errClientClosed  = errors.New("websocket client closed")
	errClientTooSlow = errors.New("websocket client fell behind")
)

type WebsocketController struct {
	svc      *services.WebsocketService
	upgrader websocket.Upgrader
}

type websocketClient struct {
	conn      *websocket.Conn
	outbox    chan any
	closed    chan struct{}
	closeOnce sync.Once
}

func newWebsocketClient(conn *websocket.Conn) *websocketClient {
	return &websocketClient{
		conn:   conn,
		outbox: make(chan any, clientOutboxSize),
		closed: make(chan struct{}),
	}
}

// Send never blocks, so a stalled client cannot hold up a broadcast. A client
// whose outbox is full is dropped.
func (c *websocketClient) Send(message any) error {
	select {
	case <-c.closed:
		return errClientClosed
	case c.outbox <- message:
		return nil
	default:
		c.close()
		return errClientTooSlow
	}
}

func (c *websocketClient) writeLoop() {
	for {
		select {
		case <-c.closed:
			return
		case message := <-c.outbox:
			if err := c.conn.SetWriteDeadline(time.Now().Add(clientWriteTimeout)); err != nil {
				c.close()
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				c.close()
				return
			}
		}
	}
}

// close also ends the read loop, which unsubscribes the client.
func (c *websocketClient) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.conn.Close()
	})
}

func NewWebsocketController(svc *services.WebsocketService) *WebsocketController {
	return &WebsocketController{
		svc:      svc,
		upgrader: websocket.Upgrader{CheckOrigin: isAllowedOrigin},
	}
}

// Clients outside a browser send no Origin, so only a browser on another site is refused.
func isAllowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || slices.Contains(config.AllowedOrigins, origin)
}

func (c *WebsocketController) Register(api *fuego.Server) {
	fuego.Handle(api, websocketRoute, http.HandlerFunc(c.serve), fuego.OptionHide())
}

func (c *WebsocketController) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	clientID := uuid.Must(uuid.NewV7())
	client := newWebsocketClient(conn)
	defer client.close()
	go client.writeLoop()
	c.svc.ClientConnected(clientID)

	err = c.readLoop(r.Context(), clientID, client)
	c.svc.ClientDisconnected(context.Background(), clientID, client, err.Error())
}

func (c *WebsocketController) readLoop(ctx context.Context, clientID uuid.UUID, client *websocketClient) error {
	for {
		if err := client.conn.SetReadDeadline(time.Now().Add(clientIdleTimeout)); err != nil {
			return err
		}
		_, data, err := client.conn.ReadMessage()
		if err != nil {
			return err
		}

		var message dtos.WebsocketClientMessage
		if err := json.Unmarshal(data, &message); err != nil {
			sendError(client, "invalid message")
			continue
		}

		c.handleMessage(ctx, clientID, client, message)
	}
}

func (c *WebsocketController) handleMessage(ctx context.Context, clientID uuid.UUID, client *websocketClient, message dtos.WebsocketClientMessage) {
	switch message.Type {
	case dtos.WebsocketMessageTypes.Subscribe:
		if message.RouteID == uuid.Nil {
			sendError(client, "routeId is required")
			return
		}
		if err := c.svc.Subscribe(ctx, client, message.RouteID); err != nil {
			slog.Error("Failed to subscribe to route", "id", clientID, "routeId", message.RouteID, "error", err)
			sendError(client, "failed to subscribe")
		}
	case dtos.WebsocketMessageTypes.Unsubscribe:
		c.svc.Unsubscribe(ctx, client)
	case dtos.WebsocketMessageTypes.Ping:
		if err := client.Send(dtos.WebsocketPongMessage{Type: dtos.WebsocketMessageTypes.Pong}); err != nil {
			slog.Warn("Failed to send websocket pong", "id", clientID, "error", err)
		}
	default:
		sendError(client, "unknown message type")
	}
}

func sendError(client *websocketClient, message string) {
	err := client.Send(dtos.WebsocketErrorMessage{Type: dtos.WebsocketMessageTypes.Error, Message: message})
	if err != nil {
		slog.Warn("Failed to send websocket error", "error", err)
	}
}
