package notify

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/notify"
	"github.com/MaroonRides/api/test"
)

var _ = Describe("live data available triggers", Label("medium", "notify"), func() {
	var (
		bundb *bun.DB
		net   test.Network
		ctx   = context.Background()
	)

	BeforeEach(func() {
		bundb = test.Configure()
		net = test.CreateNetwork(bundb, "a")
	})

	subscribe := func() uuid.UUID {
		clientID := uuid.New()
		test.Insert(bundb, &model.LiveDataSubscription{RouteID: net.Route.ID, ClientID: clientID})
		return clientID
	}

	unsubscribe := func(clientID uuid.UUID) {
		test.Exec(bundb, `DELETE FROM "live_data_subscriptions" WHERE "clientId" = ?`, clientID)
	}

	markAvailable := func() {
		test.Exec(bundb, `UPDATE "route" SET "liveDataAvailable" = TRUE WHERE "id" = ?`, net.Route.ID)
	}

	available := func() bool {
		var route model.Route
		ExpectWithOffset(1, bundb.NewSelect().Model(&route).Where(`"id" = ?`, net.Route.ID).Scan(ctx)).To(Succeed())
		return route.LiveDataAvailable
	}

	It("keeps a route available while it has subscribers", func() {
		first := subscribe()
		subscribe()
		markAvailable()

		unsubscribe(first)

		Expect(available()).To(BeTrue())
	})

	It("marks a route unavailable when its last subscriber leaves", func() {
		first, second := subscribe(), subscribe()
		markAvailable()

		test.Exec(bundb, `DELETE FROM "live_data_subscriptions" WHERE "clientId" IN (?, ?)`, first, second)

		Expect(available()).To(BeFalse())
	})

	It("does not mark a route available when it gains a subscriber", func() {
		subscribe()

		Expect(available()).To(BeFalse())
	})

	It("lets a subscribed route be deleted", func() {
		subscribe()
		markAvailable()

		test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, net.Route.ID)
	})

	It("notifies when a route becomes available", func() {
		l := listen(bundb, notify.LiveDataAvailableChannel)

		markAvailable()

		Expect(l.payloads()).To(Equal([]string{net.Route.ID.String()}))
	})

	It("stays quiet when the route is already available", func() {
		markAvailable()
		l := listen(bundb, notify.LiveDataAvailableChannel)

		markAvailable()
		test.Exec(bundb, `UPDATE "route" SET "longName" = 'Renamed' WHERE "id" = ?`, net.Route.ID)

		Expect(l.payloads()).To(BeEmpty())
	})

	It("leaves the route's sync version alone", func() {
		updateID := net.Route.UpdateID

		markAvailable()

		var route model.Route
		Expect(bundb.NewSelect().Model(&route).Where(`"id" = ?`, net.Route.ID).Scan(ctx)).To(Succeed())
		Expect(route.UpdateID).To(Equal(updateID))
	})
})
