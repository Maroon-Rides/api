package repositories

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/test"
)

var _ = Describe("TripPlanRepository", Label("medium", "repository"), func() {
	var (
		bundb *bun.DB
		repo  *repositories.TripPlanRepository
		a, b  test.Network
		ctx   = context.Background()
	)

	BeforeEach(func() {
		bundb = test.Configure()
		repo = repositories.NewTripPlanRepository(bundb, nil)
		a = test.CreateNetwork(bundb, "a")
		b = test.CreateNetwork(bundb, "b")
	})

	It("reads the stops with a code on the named routes, with their route", func() {
		stops, err := repo.GetRouteStops(ctx, []string{"a"}, []string{a.Stop.SourceID, b.Stop.SourceID})
		Expect(err).NotTo(HaveOccurred())

		Expect(stops).To(HaveLen(1))
		Expect(stops[0].ID).To(Equal(a.Stop.ID))
		Expect(stops[0].Direction.ID).To(Equal(a.Direction.ID))
		Expect(stops[0].Direction.Route.ShortName).To(Equal("a"))
	})

	It("reads nothing without routes or codes", func() {
		Expect(repo.GetRouteStops(ctx, nil, []string{a.Stop.SourceID})).To(BeEmpty())
		Expect(repo.GetRouteStops(ctx, []string{"a"}, nil)).To(Equal([]model.Stop{}))
	})
})
