package notify

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/notify"
	"github.com/MaroonRides/api/test"
)

const quietWindow = 200 * time.Millisecond

type listener struct {
	pg *pgx.Conn
}

func listen(bundb *bun.DB, channel string) *listener {
	ctx := context.Background()
	conn, err := bundb.Conn(ctx)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(conn.Close)

	l := &listener{}
	Expect(conn.Raw(func(driverConn any) error {
		l.pg = driverConn.(*stdlib.Conn).Conn()
		return nil
	})).To(Succeed())

	_, err = l.pg.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize())
	Expect(err).NotTo(HaveOccurred())
	return l
}

// payloads drains every notification that arrives before the channel goes quiet.
func (l *listener) payloads() []string {
	var out []string
	for {
		ctx, cancel := context.WithTimeout(context.Background(), quietWindow)
		n, err := l.pg.WaitForNotification(ctx)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			return out
		}
		Expect(err).NotTo(HaveOccurred())
		out = append(out, n.Payload)
	}
}

var _ = Describe("route rows changed triggers", Label("medium", "notify"), func() {
	var (
		bundb *bun.DB
		net   test.Network
		ctx   = context.Background()
	)

	BeforeEach(func() {
		bundb = test.Configure()
		net = test.CreateNetwork(bundb, "a")
	})

	vehicle := func(sourceID string) *model.Vehicle {
		return &model.Vehicle{
			SourceID:    sourceID,
			RouteID:     net.Route.ID,
			DirectionID: net.Direction.ID,
			Name:        sourceID,
			Amenities:   []string{},
		}
	}

	It("notifies a route's vehicle changes once per transaction", func() {
		l := listen(bundb, notify.VehiclesChannel)

		Expect(bundb.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, v := range []*model.Vehicle{vehicle("1"), vehicle("2")} {
				if _, err := tx.NewInsert().Model(v).Exec(ctx); err != nil {
					return err
				}
			}
			return nil
		})).To(Succeed())

		Expect(l.payloads()).To(Equal([]string{net.Route.ID.String()}))
	})

	It("notifies when a route's vehicles are updated or deleted", func() {
		test.Insert(bundb, vehicle("1"))
		l := listen(bundb, notify.VehiclesChannel)

		test.Exec(bundb, `UPDATE "vehicle" SET "lat" = 1`)
		test.Exec(bundb, `DELETE FROM "vehicle"`)

		Expect(l.payloads()).To(Equal([]string{net.Route.ID.String(), net.Route.ID.String()}))
	})

	It("notifies a route's departure changes", func() {
		l := listen(bundb, notify.DeparturesChannel)

		test.Insert(bundb, &model.Departure{
			RouteID:     net.Route.ID,
			StopID:      net.Stop.ID,
			DirectionID: net.Direction.ID,
			ScheduledAt: time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC),
		})

		Expect(l.payloads()).To(Equal([]string{net.Route.ID.String()}))
	})
})
