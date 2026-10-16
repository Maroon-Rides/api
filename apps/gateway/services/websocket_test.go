package services_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/fx/fxtest"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/metrics"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db/model"
)

type fakeSubscriptionRepo struct {
	routes     []uuid.UUID
	syncs      int
	syncErr    error
	vehicles   []model.Vehicle
	departures []model.Departure
	reads      []uuid.UUID
	available  bool
	shortNames map[uuid.UUID]string
	nameReads  []uuid.UUID
}

func (r *fakeSubscriptionRepo) IsLiveDataAvailable(_ context.Context, _ uuid.UUID) (bool, error) {
	return r.available, nil
}

func (r *fakeSubscriptionRepo) GetRouteShortName(_ context.Context, routeID uuid.UUID) (string, error) {
	r.nameReads = append(r.nameReads, routeID)
	shortName, ok := r.shortNames[routeID]
	if !ok {
		return "", errors.New("no such route")
	}
	return shortName, nil
}

func (r *fakeSubscriptionRepo) GetRouteVehicles(_ context.Context, routeID uuid.UUID) ([]model.Vehicle, error) {
	r.reads = append(r.reads, routeID)
	return r.vehicles, nil
}

func (r *fakeSubscriptionRepo) GetRouteDepartures(_ context.Context, routeID uuid.UUID) ([]model.Departure, error) {
	r.reads = append(r.reads, routeID)
	return r.departures, nil
}

func (r *fakeSubscriptionRepo) SyncLiveDataSubscriptions(_ context.Context, _ uuid.UUID, routeIDs []uuid.UUID) error {
	r.syncs++
	if r.syncErr != nil {
		return r.syncErr
	}
	r.routes = routeIDs
	return nil
}

// blockingSubscriptionRepo holds every subscription write until release is closed.
type blockingSubscriptionRepo struct {
	fakeSubscriptionRepo
	writing chan struct{}
	release chan struct{}
}

func (r *blockingSubscriptionRepo) SyncLiveDataSubscriptions(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	r.writing <- struct{}{}
	<-r.release
	return nil
}

type fakeClient struct {
	received []any
}

func (c *fakeClient) Send(message any) error {
	c.received = append(c.received, message)
	return nil
}

var _ = Describe("WebsocketService", Label("unit"), func() {
	var (
		ctx      = context.Background()
		repo     *fakeSubscriptionRepo
		registry *prometheus.Registry
		svc      *services.WebsocketService
		routeA   uuid.UUID
		routeB   uuid.UUID
	)

	BeforeEach(func() {
		routeA, routeB = newV7(), newV7()
		repo = &fakeSubscriptionRepo{shortNames: map[uuid.UUID]string{routeA: "01", routeB: "12"}}
		registry = prometheus.NewRegistry()
		svc = services.NewWebsocketService(fxtest.NewLifecycle(GinkgoT()), repo, metrics.NewWebsocketMetrics(registry))
	})

	expectMetrics := func(expected string, names ...string) {
		ExpectWithOffset(1, testutil.GatherAndCompare(registry, strings.NewReader(expected), names...)).To(Succeed())
	}

	It("counts clients and subscriptions per route by short name", func() {
		first, second := &fakeClient{}, &fakeClient{}

		Expect(svc.Subscribe(ctx, first, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeB)).To(Succeed())

		expectMetrics(`
# HELP gateway_websocket_route_clients Websocket clients subscribed, by route.
# TYPE gateway_websocket_route_clients gauge
gateway_websocket_route_clients{route="01"} 1
gateway_websocket_route_clients{route="12"} 1
# HELP gateway_websocket_route_subscriptions_total Websocket subscriptions made, by route.
# TYPE gateway_websocket_route_subscriptions_total counter
gateway_websocket_route_subscriptions_total{route="01"} 2
gateway_websocket_route_subscriptions_total{route="12"} 1
`, "gateway_websocket_route_clients", "gateway_websocket_route_subscriptions_total")
	})

	It("drops a route's client count once its last client leaves", func() {
		first, second := &fakeClient{}, &fakeClient{}
		Expect(svc.Subscribe(ctx, first, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeB)).To(Succeed())

		svc.ClientDisconnected(ctx, newV7(), first, "closed")

		expectMetrics(`
# HELP gateway_websocket_route_clients Websocket clients subscribed, by route.
# TYPE gateway_websocket_route_clients gauge
gateway_websocket_route_clients{route="12"} 1
`, "gateway_websocket_route_clients")
	})

	It("reads a route's short name once", func() {
		client := &fakeClient{}
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())
		svc.Unsubscribe(ctx, client)
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		Expect(repo.nameReads).To(Equal([]uuid.UUID{routeA}))
	})

	It("labels a route by its ID until its short name can be read", func() {
		delete(repo.shortNames, routeA)
		client := &fakeClient{}
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		expectMetrics(fmt.Sprintf(`
# HELP gateway_websocket_route_clients Websocket clients subscribed, by route.
# TYPE gateway_websocket_route_clients gauge
gateway_websocket_route_clients{route=%q} 1
`, routeA.String()), "gateway_websocket_route_clients")

		svc.Unsubscribe(ctx, client)
		repo.shortNames[routeA] = "01"
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		expectMetrics(`
# HELP gateway_websocket_route_clients Websocket clients subscribed, by route.
# TYPE gateway_websocket_route_clients gauge
gateway_websocket_route_clients{route="01"} 1
`, "gateway_websocket_route_clients")
	})

	It("syncs only when a route gains its first client or loses its last", func() {
		first, second := &fakeClient{}, &fakeClient{}

		Expect(svc.Subscribe(ctx, first, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeA)).To(Succeed())
		Expect(repo.syncs).To(Equal(1))
		Expect(repo.routes).To(Equal([]uuid.UUID{routeA}))

		svc.Unsubscribe(ctx, first)
		Expect(repo.syncs).To(Equal(1))

		svc.ClientDisconnected(ctx, newV7(), second, "closed")
		Expect(repo.syncs).To(Equal(2))
		Expect(repo.routes).To(BeEmpty())
	})

	It("serves other clients while a subscription is being stored", func() {
		blocking := &blockingSubscriptionRepo{writing: make(chan struct{}), release: make(chan struct{})}
		svc := services.NewWebsocketService(fxtest.NewLifecycle(GinkgoT()), blocking, metrics.NewWebsocketMetrics(prometheus.NewRegistry()))
		first, second := &fakeClient{}, &fakeClient{}

		subscribed := make(chan error)
		go func() { subscribed <- svc.Subscribe(ctx, first, routeA) }()
		<-blocking.writing

		Expect(svc.Subscribe(ctx, second, routeA)).To(Succeed())
		svc.Broadcast(routeA, "a")
		Expect(second.received).To(Equal([]any{"a"}))

		close(blocking.release)
		Expect(<-subscribed).To(Succeed())
	})

	It("moves a client to the new route on a second subscribe", func() {
		client := &fakeClient{}
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, client, routeB)).To(Succeed())
		Expect(repo.routes).To(Equal([]uuid.UUID{routeB}))

		svc.Broadcast(routeA, "a")
		svc.Broadcast(routeB, "b")
		Expect(client.received).To(Equal([]any{"b"}))
	})

	It("drops the client when the subscription cannot be stored", func() {
		repo.syncErr = errors.New("no such route")
		client := &fakeClient{}

		Expect(svc.Subscribe(ctx, client, routeA)).NotTo(Succeed())
		svc.Broadcast(routeA, "a")
		Expect(client.received).To(BeEmpty())
	})

	It("sends a route's vehicles to its subscribers only", func() {
		onA, onB := &fakeClient{}, &fakeClient{}
		Expect(svc.Subscribe(ctx, onA, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, onB, routeB)).To(Succeed())
		repo.vehicles = []model.Vehicle{{ID: newV7(), RouteID: routeA, Name: "101"}}

		Expect(svc.VehiclesChanged(ctx, routeA)).To(Succeed())

		Expect(onA.received).To(Equal([]any{dtos.NewWebsocketVehiclesMessage(routeA, repo.vehicles)}))
		Expect(onB.received).To(BeEmpty())
	})

	It("sends a route's departures to its subscribers", func() {
		client := &fakeClient{}
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())
		repo.departures = []model.Departure{{ID: newV7(), RouteID: routeA}}

		Expect(svc.DeparturesChanged(ctx, routeA)).To(Succeed())

		Expect(client.received).To(Equal([]any{dtos.NewWebsocketDeparturesMessage(routeA, repo.departures)}))
	})

	It("skips the read when no client is on the route", func() {
		Expect(svc.VehiclesChanged(ctx, routeA)).To(Succeed())
		Expect(svc.DeparturesChanged(ctx, routeA)).To(Succeed())
		Expect(svc.LiveDataAvailable(ctx, routeA)).To(Succeed())
		Expect(repo.reads).To(BeEmpty())
	})

	It("sends live data on subscribe once the route's first fetch finished", func() {
		repo.available = true
		repo.vehicles = []model.Vehicle{{ID: newV7(), RouteID: routeA, Name: "101"}}
		repo.departures = []model.Departure{{ID: newV7(), RouteID: routeA}}
		client := &fakeClient{}

		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		Expect(client.received).To(Equal([]any{
			dtos.NewWebsocketVehiclesMessage(routeA, repo.vehicles),
			dtos.NewWebsocketDeparturesMessage(routeA, repo.departures),
		}))
	})

	It("sends nothing on subscribe before the route's first fetch finished", func() {
		client := &fakeClient{}

		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		Expect(client.received).To(BeEmpty())
		Expect(repo.reads).To(BeEmpty())
	})

	It("sends nothing when a client resubscribes to its current route", func() {
		client := &fakeClient{}
		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())
		repo.available = true

		Expect(svc.Subscribe(ctx, client, routeA)).To(Succeed())

		Expect(client.received).To(BeEmpty())
	})

	It("sends live data to every client on the route when it becomes available", func() {
		first, second := &fakeClient{}, &fakeClient{}
		Expect(svc.Subscribe(ctx, first, routeA)).To(Succeed())
		Expect(svc.Subscribe(ctx, second, routeA)).To(Succeed())
		repo.vehicles = []model.Vehicle{{ID: newV7(), RouteID: routeA, Name: "101"}}

		Expect(svc.LiveDataAvailable(ctx, routeA)).To(Succeed())

		want := []any{
			dtos.NewWebsocketVehiclesMessage(routeA, repo.vehicles),
			dtos.NewWebsocketDeparturesMessage(routeA, repo.departures),
		}
		Expect(first.received).To(Equal(want))
		Expect(second.received).To(Equal(want))
	})
})
