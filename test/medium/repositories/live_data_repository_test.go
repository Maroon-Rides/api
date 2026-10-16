package repositories

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/test"
)

var _ = Describe("LiveDataRepository", Label("medium", "repository"), func() {
	var (
		bundb *bun.DB
		repo  *repositories.LiveDataRepository
		net   test.Network
		ctx   = context.Background()
	)

	BeforeEach(func() {
		bundb = test.Configure()
		repo = repositories.NewLiveDataRepository(bundb)
		net = test.CreateNetwork(bundb, "a")
	})

	available := func() bool {
		ok, err := repo.IsLiveDataAvailable(ctx, net.Route.ID)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return ok
	}

	It("reports a route available only after its first fetch is marked", func() {
		Expect(repo.SyncLiveDataSubscriptions(ctx, uuid.New(), []uuid.UUID{net.Route.ID})).To(Succeed())
		Expect(available()).To(BeFalse())

		test.Exec(bundb, `UPDATE "route" SET "liveDataAvailable" = TRUE WHERE "id" = ?`, net.Route.ID)

		Expect(available()).To(BeTrue())
	})

	It("reads a route's short name", func() {
		Expect(repo.GetRouteShortName(ctx, net.Route.ID)).To(Equal(net.Route.ShortName))
	})

	It("fails to read the short name of an unknown route", func() {
		_, err := repo.GetRouteShortName(ctx, uuid.New())
		Expect(err).To(HaveOccurred())
	})
})
