package services

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/internal/db/model"
)

const subscriptionSyncInterval = model.LiveDataSubscriptionRefreshInterval

type LiveDataRepository interface {
	SyncLiveDataSubscriptions(ctx context.Context, clientID uuid.UUID, routeIDs []uuid.UUID) error
	IsLiveDataAvailable(ctx context.Context, routeID uuid.UUID) (bool, error)
	GetRouteVehicles(ctx context.Context, routeID uuid.UUID) ([]model.Vehicle, error)
	GetRouteDepartures(ctx context.Context, routeID uuid.UUID) ([]model.Departure, error)
}

type WebsocketClient interface {
	Send(message any) error
}

type WebsocketService struct {
	repo      LiveDataRepository
	gatewayID uuid.UUID

	mu           sync.Mutex
	routes       map[uuid.UUID][]WebsocketClient
	clientRoutes map[WebsocketClient]uuid.UUID

	// syncMu keeps subscription writes in order, so the last one stored holds the newest routes.
	syncMu sync.Mutex
}

func NewWebsocketService(lc fx.Lifecycle, repo LiveDataRepository) *WebsocketService {
	s := &WebsocketService{
		repo:         repo,
		gatewayID:    uuid.Must(uuid.NewV7()),
		routes:       map[uuid.UUID][]WebsocketClient{},
		clientRoutes: map[WebsocketClient]uuid.UUID{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go s.syncPeriodically(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return s.unsubscribeAll(ctx)
		},
	})

	return s
}

func (s *WebsocketService) ClientConnected(clientID uuid.UUID) {
	slog.Info("Websocket client connected", "id", clientID)
}

func (s *WebsocketService) ClientDisconnected(ctx context.Context, clientID uuid.UUID, client WebsocketClient, reason string) {
	slog.Info("Websocket client disconnected", "id", clientID, "reason", reason)
	s.Unsubscribe(ctx, client)
}

// Subscribe moves the client to routeID, leaving any route it was on.
// A client joining after the route's first fetch gets the current data straight away;
// one joining before it gets the data from LiveDataAvailable.
func (s *WebsocketService) Subscribe(ctx context.Context, client WebsocketClient, routeID uuid.UUID) error {
	joined, routesChanged := s.join(client, routeID)
	if !joined {
		return nil
	}

	if routesChanged {
		if err := s.syncSubscriptions(ctx); err != nil {
			s.mu.Lock()
			s.remove(client)
			s.mu.Unlock()
			return err
		}
	}

	s.sendIfAvailable(ctx, client, routeID)
	return nil
}

// join reports whether the client moved onto routeID, and whether that changed the set of routes with clients.
func (s *WebsocketService) join(client WebsocketClient, routeID uuid.UUID) (joined, routesChanged bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if current, ok := s.clientRoutes[client]; ok && current == routeID {
		return false, false
	}
	leftEmpty := s.remove(client)

	s.routes[routeID] = append(s.routes[routeID], client)
	s.clientRoutes[client] = routeID
	return true, leftEmpty || len(s.routes[routeID]) == 1
}

func (s *WebsocketService) sendIfAvailable(ctx context.Context, client WebsocketClient, routeID uuid.UUID) {
	available, err := s.repo.IsLiveDataAvailable(ctx, routeID)
	if err != nil {
		slog.Error("Failed to check live data availability", "routeId", routeID, "error", err)
		return
	}
	if !available {
		return
	}

	messages, err := s.liveDataMessages(ctx, routeID)
	if err != nil {
		slog.Error("Failed to read live data", "routeId", routeID, "error", err)
		return
	}

	for _, message := range messages {
		if err := client.Send(message); err != nil {
			slog.Warn("Failed to send websocket message", "routeId", routeID, "error", err)
		}
	}
}

func (s *WebsocketService) Unsubscribe(ctx context.Context, client WebsocketClient) {
	s.mu.Lock()
	leftEmpty := s.remove(client)
	s.mu.Unlock()

	if leftEmpty {
		s.syncSubscriptions(ctx)
	}
}

func (s *WebsocketService) Broadcast(routeID uuid.UUID, message any) {
	s.mu.Lock()
	clients := slices.Clone(s.routes[routeID])
	s.mu.Unlock()

	for _, client := range clients {
		if err := client.Send(message); err != nil {
			slog.Warn("Failed to send websocket message", "routeId", routeID, "error", err)
		}
	}
}

func (s *WebsocketService) LiveDataAvailable(ctx context.Context, routeID uuid.UUID) error {
	if !s.hasClients(routeID) {
		return nil
	}

	messages, err := s.liveDataMessages(ctx, routeID)
	if err != nil {
		return err
	}

	for _, message := range messages {
		s.Broadcast(routeID, message)
	}
	return nil
}

func (s *WebsocketService) VehiclesChanged(ctx context.Context, routeID uuid.UUID) error {
	if !s.hasClients(routeID) {
		return nil
	}

	message, err := s.vehiclesMessage(ctx, routeID)
	if err != nil {
		return err
	}

	s.Broadcast(routeID, message)
	return nil
}

func (s *WebsocketService) DeparturesChanged(ctx context.Context, routeID uuid.UUID) error {
	if !s.hasClients(routeID) {
		return nil
	}

	message, err := s.departuresMessage(ctx, routeID)
	if err != nil {
		return err
	}

	s.Broadcast(routeID, message)
	return nil
}

func (s *WebsocketService) liveDataMessages(ctx context.Context, routeID uuid.UUID) ([]any, error) {
	vehicles, err := s.vehiclesMessage(ctx, routeID)
	if err != nil {
		return nil, err
	}

	departures, err := s.departuresMessage(ctx, routeID)
	if err != nil {
		return nil, err
	}

	return []any{vehicles, departures}, nil
}

func (s *WebsocketService) vehiclesMessage(ctx context.Context, routeID uuid.UUID) (dtos.WebsocketVehiclesMessage, error) {
	vehicles, err := s.repo.GetRouteVehicles(ctx, routeID)
	if err != nil {
		return dtos.WebsocketVehiclesMessage{}, fmt.Errorf("fetching route vehicles: %w", err)
	}

	return dtos.NewWebsocketVehiclesMessage(routeID, vehicles), nil
}

func (s *WebsocketService) departuresMessage(ctx context.Context, routeID uuid.UUID) (dtos.WebsocketDeparturesMessage, error) {
	departures, err := s.repo.GetRouteDepartures(ctx, routeID)
	if err != nil {
		return dtos.WebsocketDeparturesMessage{}, fmt.Errorf("fetching route departures: %w", err)
	}

	return dtos.NewWebsocketDeparturesMessage(routeID, departures), nil
}

func (s *WebsocketService) hasClients(routeID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.routes[routeID]) > 0
}

func (s *WebsocketService) syncPeriodically(ctx context.Context) {
	ticker := time.NewTicker(subscriptionSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncSubscriptions(ctx)
		}
	}
}

func (s *WebsocketService) unsubscribeAll(ctx context.Context) error {
	s.mu.Lock()
	clear(s.routes)
	clear(s.clientRoutes)
	s.mu.Unlock()

	return s.syncSubscriptions(ctx)
}

// syncSubscriptions reads the routes after taking syncMu, so a write that waited its turn never stores an older set.
func (s *WebsocketService) syncSubscriptions(ctx context.Context) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	err := s.repo.SyncLiveDataSubscriptions(ctx, s.gatewayID, s.subscribedRoutes())
	if err != nil {
		slog.Error("Failed to sync live data subscriptions", "gatewayId", s.gatewayID, "error", err)
	}

	return err
}

func (s *WebsocketService) subscribedRoutes() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Collect(maps.Keys(s.routes))
}

// remove reports whether the client was the last one on its route.
func (s *WebsocketService) remove(client WebsocketClient) bool {
	routeID, ok := s.clientRoutes[client]
	if !ok {
		return false
	}
	delete(s.clientRoutes, client)

	s.routes[routeID] = slices.DeleteFunc(s.routes[routeID], func(c WebsocketClient) bool { return c == client })
	if len(s.routes[routeID]) > 0 {
		return false
	}
	delete(s.routes, routeID)
	return true
}
