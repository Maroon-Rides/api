package repositories

import (
	"context"
	"iter"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
	"github.com/MaroonRides/api/test"
)

func collect[M any](rows iter.Seq2[M, error]) []M {
	var out []M
	for row, err := range rows {
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		out = append(out, row)
	}
	return out
}

func routeIDs(routes []model.Route) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(routes))
	for _, r := range routes {
		ids = append(ids, r.ID)
	}
	return ids
}

var _ = Describe("SyncRepository", Label("medium", "repository"), func() {
	var bundb *bun.DB
	var repo *repositories.SyncRepository
	ctx := context.Background()

	BeforeEach(func() {
		bundb = test.Configure()
		repo = repositories.NewSyncRepository(bundb)
	})

	Describe("NowID", func() {
		It("mints a uuidv7 from just now", func() {
			id, err := repo.NowID(ctx)
			Expect(err).NotTo(HaveOccurred())

			minted, ok := sync.MintedAt(id)
			Expect(ok).To(BeTrue())
			Expect(minted).To(BeTemporally("~", time.Now(), time.Second))
		})
	})

	Describe("Upserts", func() {
		var first, second model.Route
		var bound uuid.UUID

		BeforeEach(func() {
			first = test.CreateNetwork(bundb, "01").Route
			second = test.CreateNetwork(bundb, "02").Route
			bound = test.UUIDv7Ago(bundb, 0)
			test.CreateNetwork(bundb, "03")
		})

		It("reads from the beginning up to the bound", func() {
			got := collect(repositories.Upserts[model.Route](ctx, repo, uuid.Nil, bound))
			Expect(routeIDs(got)).To(Equal([]uuid.UUID{first.ID, second.ID}))
		})

		It("treats after as exclusive", func() {
			got := collect(repositories.Upserts[model.Route](ctx, repo, first.UpdateID, bound))
			Expect(routeIDs(got)).To(Equal([]uuid.UUID{second.ID}))
		})

		It("reads nothing after the last row", func() {
			Expect(collect(repositories.Upserts[model.Route](ctx, repo, second.UpdateID, bound))).To(BeEmpty())
		})

		It("orders by update, not insert", func() {
			test.Exec(bundb, `UPDATE "route" SET "longName" = 'Renamed' WHERE "id" = ?`, first.ID)

			got := collect(repositories.Upserts[model.Route](ctx, repo, uuid.Nil, test.UUIDv7Ago(bundb, 0)))
			Expect(got).To(HaveLen(3))
			Expect(got[2].ID).To(Equal(first.ID))
			Expect(got[2].LongName).To(Equal("Renamed"))
		})

		It("stops querying when the caller stops reading", func() {
			for _, err := range repositories.Upserts[model.Route](ctx, repo, uuid.Nil, bound) {
				Expect(err).NotTo(HaveOccurred())
				break
			}

			Expect(collect(repositories.Upserts[model.Route](ctx, repo, uuid.Nil, bound))).To(HaveLen(2))
		})

		It("yields the query error", func() {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()

			var errs []error
			for _, err := range repositories.Upserts[model.Route](cancelled, repo, uuid.Nil, bound) {
				errs = append(errs, err)
			}
			Expect(errs).To(HaveLen(1))
			Expect(errs[0]).To(HaveOccurred())
		})
	})

	Describe("Deletes", func() {
		var network test.Network

		BeforeEach(func() {
			network = test.CreateNetwork(bundb, "01")
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, network.Route.ID)
		})

		It("reads the tombstone of a deleted row", func() {
			got := collect(repositories.Deletes[model.RouteAudit](ctx, repo, uuid.Nil, test.UUIDv7Ago(bundb, 0)))
			Expect(got).To(HaveLen(1))
			Expect(got[0].RouteID).To(Equal(network.Route.ID))
		})

		It("reads tombstones left by a cascade", func() {
			got := collect(repositories.Deletes[model.DirectionAudit](ctx, repo, uuid.Nil, test.UUIDv7Ago(bundb, 0)))
			Expect(got).To(HaveLen(1))
			Expect(got[0].DirectionID).To(Equal(network.Direction.ID))
		})

		It("treats after as exclusive", func() {
			bound := test.UUIDv7Ago(bundb, 0)
			tombstone := collect(repositories.Deletes[model.RouteAudit](ctx, repo, uuid.Nil, bound))[0]

			Expect(collect(repositories.Deletes[model.RouteAudit](ctx, repo, tombstone.ID, bound))).To(BeEmpty())
		})
	})
})
