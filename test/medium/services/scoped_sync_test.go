package services

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/test"
)

// heldTimetable is what the app can compare: the payload fields plus the route it was synced under.
type heldTimetable struct {
	RouteID     uuid.UUID
	StopID      uuid.UUID
	DirectionID uuid.UUID
	ServiceDate string
	Departures  []int64
}

func holdTimetable(routeID uuid.UUID, t dtos.SyncTimetableV1) heldTimetable {
	departures := make([]int64, 0, len(t.Departures))
	for _, d := range t.Departures {
		departures = append(departures, d.UnixMilli())
	}
	return heldTimetable{RouteID: routeID, StopID: t.StopID, DirectionID: t.DirectionID, ServiceDate: t.ServiceDate, Departures: departures}
}

// replica applies timetable lines the way the app stores them, so specs can compare it with the database.
type replica struct {
	*syncClient
	timetables map[uuid.UUID]heldTimetable
}

func newReplica(bundb *bun.DB) *replica {
	return &replica{syncClient: newSyncClient(bundb), timetables: map[uuid.UUID]heldTimetable{}}
}

func (r *replica) pull() []dtos.SyncStreamLine {
	lines := r.sync(requests.RoutesV1, requests.TimetablesV1)
	for _, line := range lines {
		key, _, err := services.ParseAck(line.Ack)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		switch data := line.Data.(type) {
		case dtos.SyncTimetableV1:
			ExpectWithOffset(1, r.offlineRoutes).To(ContainElement(key.Scope), "sent a timetable for a route the client does not keep")
			r.timetables[data.ID] = holdTimetable(key.Scope, data)
		case dtos.SyncTimetableDeleteV1:
			ExpectWithOffset(1, r.offlineRoutes).To(ContainElement(key.Scope), "sent a tombstone for a route the client does not keep")
			delete(r.timetables, data.TimetableID)
		}
	}
	return lines
}

func (r *replica) drop(routeID uuid.UUID) {
	r.dropOffline(routeID)
	maps.DeleteFunc(r.timetables, func(_ uuid.UUID, t heldTimetable) bool { return t.RouteID == routeID })
}

func (r *replica) expectMatches(bundb *bun.DB) {
	want := map[uuid.UUID]heldTimetable{}
	if len(r.offlineRoutes) > 0 {
		var rows []model.Timetable
		err := bundb.NewSelect().Model(&rows).Where(`"routeId" IN (?)`, bun.In(r.offlineRoutes)).Scan(context.Background())
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		for _, row := range rows {
			want[row.ID] = holdTimetable(row.RouteID, dtos.NewSyncTimetableV1(row))
		}
	}
	ExpectWithOffset(1, r.timetables).To(Equal(want))
}

func timetableFor(n test.Network, serviceDate time.Time, departures ...time.Time) model.Timetable {
	return model.Timetable{
		StopID:      n.Stop.ID,
		DirectionID: n.Direction.ID,
		RouteID:     n.Route.ID,
		ServiceDate: serviceDate,
		Departures:  departures,
	}
}

func timetableLines(lines []dtos.SyncStreamLine) []dtos.SyncStreamLine {
	return slices.DeleteFunc(slices.Clone(lines), func(l dtos.SyncStreamLine) bool {
		return l.Type != entities.TimetableV1 && l.Type != entities.TimetableDeleteV1
	})
}

var _ = Describe("SyncService scoped streams", Label("medium", "service"), func() {
	var bundb *bun.DB
	var client *replica
	var a, b test.Network

	BeforeEach(func() {
		bundb = test.Configure()
		client = newReplica(bundb)
		a = test.CreateNetwork(bundb, "01")
		b = test.CreateNetwork(bundb, "02")
	})

	It("sends no timetables to a client that keeps no routes offline", func() {
		Expect(timetableLines(client.pull())).To(BeEmpty())
	})

	It("sends only the timetables of routes kept offline", func() {
		client.keepOffline(a.Route.ID)

		lines := timetableLines(client.pull())

		Expect(lines).To(HaveLen(1))
		Expect(lines[0].Data.(dtos.SyncTimetableV1).ID).To(Equal(a.Timetable.ID))
		Expect(lines[0].Ack).To(HavePrefix("TimetableV1:" + a.Route.ID.String() + "|"))
		client.expectMatches(bundb)
	})

	It("backfills a newly kept route without resending the others", func() {
		client.keepOffline(a.Route.ID)
		client.pull()

		client.keepOffline(b.Route.ID)
		lines := timetableLines(client.pull())

		Expect(lines).To(HaveLen(1))
		Expect(lines[0].Data.(dtos.SyncTimetableV1).ID).To(Equal(b.Timetable.ID))
		client.expectMatches(bundb)
	})

	It("sends a new route's rows without its tombstones", func() {
		test.Exec(bundb, `DELETE FROM "timetable" WHERE "id" = ?`, b.Timetable.ID)
		test.Insert(bundb, new(timetableFor(b, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC))))

		client.keepOffline(b.Route.ID)
		lines := timetableLines(client.pull())

		Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.TimetableV1}))
		client.expectMatches(bundb)
	})

	It("keeps one position per route, so one route's ack does not skip another's rows", func() {
		client.keepOffline(a.Route.ID)
		client.pull()
		aAck := client.acks[services.SyncAckKey{Entity: entities.TimetableV1, Scope: a.Route.ID}]

		lines := client.stream(dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.TimetablesV1},
			Scopes:   []dtos.SyncScope{{Type: dtos.SyncScopeTypes.OfflineRoutesV1, IDs: []uuid.UUID{a.Route.ID, b.Route.ID}}},
			Acks:     []string{aAck, completeAck(bundb)},
		})

		Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.TimetableV1, entities.SyncCompleteV1}))
		Expect(lines[0].Data.(dtos.SyncTimetableV1).ID).To(Equal(b.Timetable.ID))
	})

	It("sends changes only to clients of the changed route", func() {
		other := newReplica(bundb)
		client.keepOffline(a.Route.ID)
		other.keepOffline(b.Route.ID)
		client.pull()
		other.pull()

		test.Exec(bundb, `UPDATE "timetable" SET "departures" = ARRAY[now()] WHERE "id" = ?`, a.Timetable.ID)
		test.Exec(bundb, `DELETE FROM "timetable" WHERE "id" = ?`, a.Timetable.ID)

		Expect(types(timetableLines(client.pull()))).To(Equal([]dtos.SyncEntityType{entities.TimetableDeleteV1}))
		Expect(timetableLines(other.pull())).To(BeEmpty())
		client.expectMatches(bundb)
		other.expectMatches(bundb)
	})

	It("sends the tombstones of a deleted route to its clients", func() {
		client.keepOffline(a.Route.ID, b.Route.ID)
		client.pull()

		test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, a.Route.ID)
		lines := timetableLines(client.pull())

		Expect(lines).To(HaveLen(1))
		Expect(lines[0].Data).To(Equal(dtos.SyncTimetableDeleteV1{TimetableID: a.Timetable.ID}))
		Expect(lines[0].Ack).To(HavePrefix("TimetableDeleteV1:" + a.Route.ID.String() + "|"))
		client.expectMatches(bundb)
	})

	It("stops sending a dropped route and backfills it in full when kept again", func() {
		client.keepOffline(a.Route.ID)
		client.pull()

		client.drop(a.Route.ID)
		test.Exec(bundb, `UPDATE "timetable" SET "departures" = ARRAY[now()] WHERE "id" = ?`, a.Timetable.ID)
		Expect(timetableLines(client.pull())).To(BeEmpty())

		client.keepOffline(a.Route.ID)
		Expect(types(timetableLines(client.pull()))).To(Equal([]dtos.SyncEntityType{entities.TimetableV1}))
		client.expectMatches(bundb)
	})

	It("ignores acks for routes the request no longer keeps", func() {
		client.keepOffline(a.Route.ID)
		client.pull()
		client.offlineRoutes = nil

		Expect(timetableLines(client.pull())).To(BeEmpty())
	})

	It("sends nothing for a kept route that has no timetables", func() {
		client.keepOffline(uuid.New())

		Expect(timetableLines(client.pull())).To(BeEmpty())
	})

	Describe("the timetable table", func() {
		It("records the route on each tombstone", func() {
			test.Exec(bundb, `DELETE FROM "timetable" WHERE "id" = ?`, a.Timetable.ID)

			var audit model.TimetableAudit
			err := bundb.NewSelect().Model(&audit).Where(`"timetableId" = ?`, a.Timetable.ID).Scan(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(audit.RouteID).To(Equal(a.Route.ID))
		})

		It("refuses a route that is not its direction's", func() {
			_, err := bundb.NewInsert().
				Model(new(model.Timetable{StopID: a.Stop.ID, DirectionID: a.Direction.ID, RouteID: b.Route.ID, ServiceDate: time.Now(), Departures: []time.Time{}})).
				Exec(context.Background())
			Expect(err).To(MatchError(ContainSubstring("foreign key")))
		})

		It("refuses to move a row to another route", func() {
			_, err := bundb.ExecContext(context.Background(),
				`UPDATE "timetable" SET "routeId" = ?, "directionId" = ? WHERE "id" = ?`, b.Route.ID, b.Direction.ID, a.Timetable.ID)
			Expect(err).To(MatchError(ContainSubstring("cannot change once written")))
		})
	})
})

// Each seed plays a different run of writes, subscribes, drops and syncs. After
// every sync the client must hold exactly the database's rows for its routes.
var _ = Describe("SyncService scoped streams under random changes", Label("medium", "service"), func() {
	const (
		networks = 4
		steps    = 80
		// Route deletes are rarer than other actions so most of a run keeps several routes.
		routeDeleteOdds = 4
	)
	serviceDates := []time.Time{
		time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	}

	DescribeTable("the client converges on the database",
		func(seed uint64) {
			bundb := test.Configure()
			rng := rand.New(rand.NewPCG(seed, seed))
			client := newReplica(bundb)

			var all []test.Network
			for i := range networks {
				all = append(all, test.CreateNetwork(bundb, fmt.Sprintf("%02d", i)))
			}
			pick := func() test.Network { return all[rng.IntN(len(all))] }
			departure := func(date time.Time) time.Time {
				return date.Add(time.Duration(rng.IntN(24*60)) * time.Minute)
			}
			randomTimetable := func() uuid.UUID {
				var id uuid.UUID
				err := bundb.NewRaw(`SELECT "id" FROM "timetable" ORDER BY "id" OFFSET ? LIMIT 1`, rng.IntN(networks*len(serviceDates))).Scan(context.Background(), &id)
				if err != nil {
					return uuid.Nil
				}
				return id
			}

			actions := []func(){
				func() {
					n, date := pick(), serviceDates[rng.IntN(len(serviceDates))]
					test.Exec(bundb, `INSERT INTO "timetable" ("stopId", "directionId", "routeId", "serviceDate", "departures")
						VALUES (?, ?, ?, ?, ARRAY[?::timestamptz]) ON CONFLICT DO NOTHING`,
						n.Stop.ID, n.Direction.ID, n.Route.ID, date, departure(date))
				},
				func() {
					test.Exec(bundb, `UPDATE "timetable" SET "departures" = ARRAY[?::timestamptz] WHERE "id" = ?`,
						departure(serviceDates[0]), randomTimetable())
				},
				func() {
					test.Exec(bundb, `UPDATE "timetable" SET "departures" = "departures" WHERE "id" = ?`, randomTimetable())
				},
				func() { test.Exec(bundb, `DELETE FROM "timetable" WHERE "id" = ?`, randomTimetable()) },
				func() {
					test.Exec(bundb, `DELETE FROM "timetable" WHERE "serviceDate" < ?`, serviceDates[rng.IntN(len(serviceDates))])
				},
				func() {
					if n := pick(); !slices.Contains(client.offlineRoutes, n.Route.ID) {
						client.keepOffline(n.Route.ID)
					}
				},
				func() {
					if len(all) > 1 && rng.IntN(routeDeleteOdds) == 0 {
						gone := rng.IntN(len(all))
						test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, all[gone].Route.ID)
						all = slices.Delete(all, gone, gone+1)
					}
				},
				func() {
					if len(client.offlineRoutes) > 0 {
						client.drop(client.offlineRoutes[rng.IntN(len(client.offlineRoutes))])
					}
				},
				func() {
					client.pull()
					client.expectMatches(bundb)
				},
			}

			for range steps {
				actions[rng.IntN(len(actions))]()
			}
			client.pull()
			client.expectMatches(bundb)
		},
		func(seed uint64) string { return fmt.Sprintf("seed %d", seed) },
		Entry(nil, uint64(1)),
		Entry(nil, uint64(2)),
		Entry(nil, uint64(3)),
		Entry(nil, uint64(4)),
		Entry(nil, uint64(5)),
		Entry(nil, uint64(6)),
		Entry(nil, uint64(7)),
		Entry(nil, uint64(8)),
	)
})
