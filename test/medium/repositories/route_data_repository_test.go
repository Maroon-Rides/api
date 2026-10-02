package repositories

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	requester "github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/test"
)

var _ = Describe("RouteDataRepository.SyncStops", Label("medium", "repository"), func() {
	var (
		bundb  *bun.DB
		repo   *requester.RouteDataRepository
		synced test.Network
		other  test.Network
		ctx    = context.Background()
	)

	BeforeEach(func() {
		bundb = test.Configure()
		repo = requester.NewRouteDataRepository(bundb)
		synced = test.CreateNetwork(bundb, "a")
		other = test.CreateNetwork(bundb, "b")
	})

	storedStopIDs := func() []uuid.UUID {
		var ids []uuid.UUID
		err := bundb.NewSelect().Model((*model.Stop)(nil)).Column("id").Scan(ctx, &ids)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return ids
	}

	It("keeps a stop the direction still serves", func() {
		Expect(repo.SyncStops(ctx, []uuid.UUID{synced.Direction.ID}, []model.Stop{synced.Stop})).To(Succeed())

		Expect(storedStopIDs()).To(ConsistOf(synced.Stop.ID, other.Stop.ID))
	})

	It("removes a stop the direction no longer serves, leaving other directions alone", func() {
		moved := model.Stop{DirectionID: synced.Direction.ID, SourceID: "moved", Name: "Moved"}

		Expect(repo.SyncStops(ctx, []uuid.UUID{synced.Direction.ID}, []model.Stop{moved})).To(Succeed())

		Expect(storedStopIDs()).To(HaveLen(2))
		Expect(storedStopIDs()).To(ContainElement(other.Stop.ID))
		Expect(storedStopIDs()).NotTo(ContainElement(synced.Stop.ID))
	})

	It("removes every stop of a direction that now has none", func() {
		Expect(repo.SyncStops(ctx, []uuid.UUID{synced.Direction.ID}, nil)).To(Succeed())

		Expect(storedStopIDs()).To(Equal([]uuid.UUID{other.Stop.ID}))
	})
})
