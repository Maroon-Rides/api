package services_test

import (
	"context"
	"errors"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx/fxtest"

	"github.com/MaroonRides/api/apps/gateway/dtos"
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
}

func (r *fakeSubscriptionRepo) IsLiveDataAvailable(_ context.Context, _ uuid.UUID) (bool, error) {
	return r.available, nil
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
		ctx    = context.Background()
		repo   *fakeSubscriptionRepo
		svc    *services.WebsocketService
		routeA uuid.UUID
		routeB uuid.UUID
	)

	BeforeEach(func() {
		repo = &fakeSubscriptionRepo{}
		svc = services.NewWebsocketService(fxtest.NewLifecycle(GinkgoT()), repo)
		routeA, routeB = newV7(), newV7()
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
		svc := services.NewWebsocketService(fxtest.NewLifecycle(GinkgoT()), blocking)
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
