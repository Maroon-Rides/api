package services

import "log/slog"

type WebsocketService struct{}

func NewWebsocketService() *WebsocketService {
	return &WebsocketService{}
}

func (s *WebsocketService) ClientConnected(clientID string) {
	slog.Info("Websocket client connected", "id", clientID)
}

func (s *WebsocketService) ClientDisconnected(clientID string, reason string) {
	slog.Info("Websocket client disconnected", "id", clientID, "reason", reason)
}

func (s *WebsocketService) RouteRoomCreated(routeID string) {
	slog.Info("Route room created", "routeId", routeID)
}

func (s *WebsocketService) RouteRoomDeleted(routeID string) {
	slog.Info("Route room deleted", "routeId", routeID)
}
