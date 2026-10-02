package services

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
	"github.com/MaroonRides/api/test"
)

var (
	requests = dtos.SyncRequestTypes
	entities = dtos.SyncEntityTypes

	allTypes = []dtos.SyncRequestType{
		requests.RoutesV1, requests.DirectionsV1, requests.StopsV1,
		requests.AlertsV1, requests.AlertDirectionsV1, requests.TimetablesV1,
	}
)

// syncClient behaves like the app: it sends every ack it holds and keeps the newest one per entity type.
type syncClient struct {
	svc  *services.SyncService
	acks map[dtos.SyncEntityType]string
}

func newSyncClient(bundb *bun.DB) *syncClient {
	return &syncClient{
		svc:  services.NewSyncService(repositories.NewSyncRepository(bundb)),
		acks: map[dtos.SyncEntityType]string{},
	}
}

func (c *syncClient) request(types ...dtos.SyncRequestType) dtos.SyncRequest {
	return dtos.SyncRequest{
		Protocol: dtos.SyncProtocolVersions.V1,
		Types:    types,
		Acks:     c.storedAcks(),
	}
}

func (c *syncClient) sync(types ...dtos.SyncRequestType) []dtos.SyncStreamLine {
	lines := c.stream(c.request(types...))
	for _, line := range lines {
		entity, _, err := services.ParseAck(line.Ack)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the server sent a bad ack")
		c.acks[entity] = line.Ack
	}
	return lines
}

func (c *syncClient) stream(req dtos.SyncRequest) []dtos.SyncStreamLine {
	waitPastNowIDLag()

	plan, err := c.svc.Plan(req)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	var lines []dtos.SyncStreamLine
	err = c.svc.Stream(context.Background(), plan, func(line dtos.SyncStreamLine) error {
		lines = append(lines, line)
		return nil
	})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return lines
}

func (c *syncClient) storedAcks() []string {
	return slices.Collect(maps.Values(c.acks))
}

// completeAck stands in for the ack of a sync that just finished.
func completeAck(bundb *bun.DB) string {
	return services.FormatAck(entities.SyncCompleteV1, test.UUIDv7Ago(bundb, 0))
}

func setResetBefore(bundb *bun.DB, id uuid.UUID) {
	test.Exec(bundb, `INSERT INTO "sync_metadata" ("key", "value") VALUES (?, ?)
		ON CONFLICT ("key") DO UPDATE SET "value" = EXCLUDED."value"`, model.SyncMetadataKeys.ResetBefore, id)
}

// Rows written inside the lag are left for the next sync by design; specs wait it out so their writes are visible.
func waitPastNowIDLag() {
	time.Sleep(2 * repositories.NowIDLag)
}

func types(lines []dtos.SyncStreamLine) []dtos.SyncEntityType {
	out := make([]dtos.SyncEntityType, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Type)
	}
	return out
}

var _ = Describe("SyncService", Label("medium", "service"), func() {
	var bundb *bun.DB
	var client *syncClient

	BeforeEach(func() {
		bundb = test.Configure()
		client = newSyncClient(bundb)
	})

	Describe("a first sync", func() {
		var network test.Network

		BeforeEach(func() {
			network = test.CreateNetwork(bundb, "01")
		})

		It("sends every requested table, parents first, then completes", func() {
			lines := client.sync(allTypes...)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{
				entities.RouteV1, entities.DirectionV1, entities.StopV1,
				entities.AlertV1, entities.AlertDirectionV1, entities.TimetableV1, entities.SyncCompleteV1,
			}))
		})

		It("maps each row into its payload", func() {
			lines := client.sync(allTypes...)

			Expect(lines[0].Data).To(Equal(dtos.NewSyncRouteV1(network.Route)))
			Expect(lines[1].Data).To(Equal(dtos.NewSyncDirectionV1(network.Direction)))
			Expect(lines[2].Data).To(Equal(dtos.NewSyncStopV1(network.Stop)))
			Expect(lines[4].Data).To(Equal(dtos.NewSyncAlertDirectionV1(network.AlertDirection)))
			Expect(lines[6].Data).To(Equal(dtos.SyncCompleteV1{}))

			stop := lines[2].Data.(dtos.SyncStopV1)
			Expect(stop.DirectionID).To(Equal(network.Direction.ID))
			Expect(stop.Amenities).To(Equal([]string{"shelter"}))

			alert := lines[3].Data.(dtos.SyncAlertV1)
			Expect(alert.ID).To(Equal(network.Alert.ID))
			Expect(alert.StartsAt).To(BeTemporally("==", network.Alert.StartsAt))
			Expect(alert.EndsAt).To(BeNil())

			timetable := lines[5].Data.(dtos.SyncTimetableV1)
			Expect(timetable.ServiceDate).To(Equal("2026-09-27"))
			Expect(timetable.Departures).To(HaveLen(len(network.Timetable.Departures)))
			for i, departure := range network.Timetable.Departures {
				Expect(timetable.Departures[i]).To(BeTemporally("==", departure))
			}
		})

		It("acks each row with its update id", func() {
			lines := client.sync(requests.RoutesV1)

			Expect(lines[0].Ack).To(Equal(services.FormatAck(entities.RouteV1, network.Route.UpdateID)))
		})

		It("sends only the requested types, in server order", func() {
			lines := client.sync(requests.AlertsV1, requests.StopsV1)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.StopV1, entities.AlertV1, entities.SyncCompleteV1}))
		})

		It("acks the complete line with the stream's upper bound", func() {
			lines := client.sync(requests.RoutesV1)

			_, bound, err := services.ParseAck(lines[len(lines)-1].Ack)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes.Compare(bound[:], network.Route.UpdateID[:])).To(Equal(1), "the bound is not after the last row")
		})
	})

	Describe("a later sync", func() {
		var first test.Network

		BeforeEach(func() {
			first = test.CreateNetwork(bundb, "01")
			client.sync(allTypes...)
		})

		It("sends nothing when nothing changed", func() {
			Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
		})

		It("sends only changed and new rows", func() {
			test.Exec(bundb, `UPDATE "route" SET "longName" = 'Renamed' WHERE "id" = ?`, first.Route.ID)
			second := test.CreateNetwork(bundb, "02")

			lines := client.sync(allTypes...)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{
				entities.RouteV1, entities.RouteV1, entities.DirectionV1, entities.StopV1,
				entities.AlertV1, entities.AlertDirectionV1, entities.TimetableV1, entities.SyncCompleteV1,
			}))
			Expect(lines[0].Data.(dtos.SyncRouteV1).LongName).To(Equal("Renamed"))
			Expect(lines[1].Data.(dtos.SyncRouteV1).ID).To(Equal(second.Route.ID))
		})

		It("does not resend a row when only a server-only column changed", func() {
			test.Exec(bundb, `UPDATE "route" SET "sourceId" = 'moved' WHERE "id" = ?`, first.Route.ID)

			Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
		})

		It("sends deletes before upserts, including cascades", func() {
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, first.Route.ID)
			kept := test.CreateNetwork(bundb, "02")

			lines := client.sync(requests.RoutesV1, requests.DirectionsV1, requests.TimetablesV1)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{
				entities.RouteDeleteV1, entities.RouteV1,
				entities.DirectionDeleteV1, entities.DirectionV1,
				entities.TimetableDeleteV1, entities.TimetableV1,
				entities.SyncCompleteV1,
			}))
			Expect(lines[0].Data).To(Equal(dtos.SyncRouteDeleteV1{RouteID: first.Route.ID}))
			Expect(lines[1].Data.(dtos.SyncRouteV1).ID).To(Equal(kept.Route.ID))
			Expect(lines[2].Data).To(Equal(dtos.SyncDirectionDeleteV1{DirectionID: first.Direction.ID}))
			Expect(lines[4].Data).To(Equal(dtos.SyncTimetableDeleteV1{TimetableID: first.Timetable.ID}))
		})

		It("does not resend a delete once acked", func() {
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, first.Route.ID)
			client.sync(allTypes...)

			Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
		})
	})

	Describe("tombstones", func() {
		var deleted, kept test.Network

		BeforeEach(func() {
			deleted = test.CreateNetwork(bundb, "01")
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, deleted.Route.ID)
			kept = test.CreateNetwork(bundb, "02")
		})

		It("are not sent to a client with no acks", func() {
			lines := client.sync(requests.RoutesV1)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.RouteV1, entities.SyncCompleteV1}))
		})

		It("older than the upsert ack are not sent to a client with no delete ack", func() {
			client.sync(requests.RoutesV1)
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, kept.Route.ID)

			lines := client.sync(requests.RoutesV1)

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.RouteDeleteV1, entities.SyncCompleteV1}))
			Expect(lines[0].Data).To(Equal(dtos.SyncRouteDeleteV1{RouteID: kept.Route.ID}))
		})
	})

	Describe("acks", func() {
		var network test.Network

		BeforeEach(func() {
			network = test.CreateNetwork(bundb, "01")
		})

		It("only move the entity type they name", func() {
			lines := client.stream(dtos.SyncRequest{
				Protocol: dtos.SyncProtocolVersions.V1,
				Types:    []dtos.SyncRequestType{requests.RoutesV1, requests.StopsV1},
				Acks:     []string{services.FormatAck(entities.StopV1, network.Stop.UpdateID), completeAck(bundb)},
			})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.RouteV1, entities.SyncCompleteV1}))
		})

		It("use the newest when a type is acked twice", func() {
			older := test.UUIDv7Ago(bundb, time.Hour)
			lines := client.stream(dtos.SyncRequest{
				Protocol: dtos.SyncProtocolVersions.V1,
				Types:    []dtos.SyncRequestType{requests.RoutesV1},
				Acks: []string{
					services.FormatAck(entities.RouteV1, network.Route.UpdateID),
					services.FormatAck(entities.RouteV1, older),
					completeAck(bundb),
				},
			})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
		})
	})

	Describe("reset", func() {
		BeforeEach(func() {
			test.CreateNetwork(bundb, "01")
		})

		It("tells a client whose last complete sync is too old to start over", func() {
			stale := test.UUIDv7Ago(bundb, sync.ResetAfter+time.Hour)

			lines := client.stream(dtos.SyncRequest{
				Protocol: dtos.SyncProtocolVersions.V1,
				Types:    allTypes,
				Acks:     []string{services.FormatAck(entities.SyncCompleteV1, stale)},
			})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.SyncResetV1}))
			Expect(lines[0].Data).To(Equal(dtos.SyncResetV1{}))
		})

		It("tells a client holding acks but no complete ack to start over", func() {
			network := test.CreateNetwork(bundb, "02")

			lines := client.stream(dtos.SyncRequest{
				Protocol: dtos.SyncProtocolVersions.V1,
				Types:    []dtos.SyncRequestType{requests.RoutesV1},
				Acks:     []string{services.FormatAck(entities.RouteV1, network.Route.UpdateID)},
			})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.SyncResetV1}))
		})

		Describe("the reset floor", func() {
			BeforeEach(func() {
				client.sync(allTypes...)
			})

			It("tells a client whose last complete sync is older to start over", func() {
				setResetBefore(bundb, test.UUIDv7Ago(bundb, -time.Minute))

				Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncResetV1}))
			})

			It("lets a client that completed a sync after it continue", func() {
				setResetBefore(bundb, test.UUIDv7Ago(bundb, time.Hour))

				Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
			})

			It("resets a client once, even though its row acks stay older than the floor", func() {
				setResetBefore(bundb, test.UUIDv7Ago(bundb, 0))
				Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncResetV1}))

				client.acks = map[dtos.SyncEntityType]string{}
				client.sync(allTypes...)
				routeAck := client.acks[entities.RouteV1]
				_, routeID, err := services.ParseAck(routeAck)
				Expect(err).NotTo(HaveOccurred())

				var floor model.SyncMetadata
				Expect(bundb.NewSelect().Model(&floor).Where(`"key" = ?`, model.SyncMetadataKeys.ResetBefore).Scan(context.Background())).To(Succeed())
				Expect(bytes.Compare(routeID[:], floor.Value[:])).To(Equal(-1), "the route ack should predate the floor")

				Expect(types(client.sync(allTypes...))).To(Equal([]dtos.SyncEntityType{entities.SyncCompleteV1}))
			})

			It("does not reset a client that holds no acks", func() {
				setResetBefore(bundb, test.UUIDv7Ago(bundb, 0))
				fresh := newSyncClient(bundb)

				Expect(types(fresh.sync(requests.RoutesV1))).To(Equal([]dtos.SyncEntityType{entities.RouteV1, entities.SyncCompleteV1}))
			})
		})

		It("lets a client inside the window continue", func() {
			recent := test.UUIDv7Ago(bundb, sync.ResetAfter-time.Hour)

			lines := client.stream(dtos.SyncRequest{
				Protocol: dtos.SyncProtocolVersions.V1,
				Types:    []dtos.SyncRequestType{requests.RoutesV1},
				Acks:     []string{services.FormatAck(entities.SyncCompleteV1, recent)},
			})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{entities.RouteV1, entities.SyncCompleteV1}))
		})
	})

	When("a new version of a resource ships", func() {
		const routesV2, routeV2 = dtos.SyncRequestType("RoutesV2"), dtos.SyncEntityType("RouteV2")

		BeforeEach(func() {
			original := services.SyncStreams
			services.SyncStreams = append(slices.Clone(original),
				services.NewSyncStream(routesV2, routeV2, "RouteDeleteV2", dtos.NewSyncRouteV1, dtos.NewSyncRouteDeleteV1))
			DeferCleanup(func() { services.SyncStreams = original })
		})

		It("backfills every row for a client that only holds V1 acks", func() {
			network := test.CreateNetwork(bundb, "01")
			client.sync(requests.RoutesV1)

			lines := client.stream(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{routesV2}, Acks: client.storedAcks()})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{routeV2, entities.SyncCompleteV1}))
			Expect(lines[0].Data.(dtos.SyncRouteV1).ID).To(Equal(network.Route.ID))
		})

		It("ignores V1 delete acks and sends no tombstones", func() {
			deleted := test.CreateNetwork(bundb, "01")
			kept := test.CreateNetwork(bundb, "02")
			client.sync(requests.RoutesV1)
			test.Exec(bundb, `DELETE FROM "route" WHERE "id" = ?`, deleted.Route.ID)
			client.sync(requests.RoutesV1)
			Expect(client.acks).To(HaveKey(entities.RouteDeleteV1))

			lines := client.stream(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{routesV2}, Acks: client.storedAcks()})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{routeV2, entities.SyncCompleteV1}))
			Expect(lines[0].Data.(dtos.SyncRouteV1).ID).To(Equal(kept.Route.ID))
		})
	})

	When("an older version of a scoped table's stream stays unscoped", func() {
		const allTimetablesV1, allTimetableV1 = dtos.SyncRequestType("AllTimetablesV1"), dtos.SyncEntityType("AllTimetableV1")

		BeforeEach(func() {
			original := services.SyncStreams
			services.SyncStreams = append(slices.Clone(original),
				services.NewSyncStream(allTimetablesV1, allTimetableV1, "AllTimetableDeleteV1", dtos.NewSyncTimetableV1, dtos.NewSyncTimetableDeleteV1))
			DeferCleanup(func() { services.SyncStreams = original })
		})

		It("sends every route's rows to a client that requests it", func() {
			a := test.CreateNetwork(bundb, "01")
			b := test.CreateNetwork(bundb, "02")

			lines := client.stream(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{allTimetablesV1}})

			Expect(types(lines)).To(Equal([]dtos.SyncEntityType{allTimetableV1, allTimetableV1, entities.SyncCompleteV1}))
			Expect([]uuid.UUID{lines[0].Data.(dtos.SyncTimetableV1).ID, lines[1].Data.(dtos.SyncTimetableV1).ID}).
				To(ConsistOf(a.Timetable.ID, b.Timetable.ID))
		})
	})

	Describe("failures", func() {
		var svc *services.SyncService

		BeforeEach(func() {
			test.CreateNetwork(bundb, "01")
			test.CreateNetwork(bundb, "02")
			svc = services.NewSyncService(repositories.NewSyncRepository(bundb))
			waitPastNowIDLag()
		})

		It("stops at the first send error and returns it", func() {
			plan, err := svc.Plan(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: allTypes})
			Expect(err).NotTo(HaveOccurred())

			gone := errors.New("client went away")
			var sent []dtos.SyncStreamLine
			err = svc.Stream(context.Background(), plan, func(line dtos.SyncStreamLine) error {
				sent = append(sent, line)
				return gone
			})

			Expect(err).To(MatchError(gone))
			Expect(sent).To(HaveLen(1))
		})

		It("returns a database error before sending anything", func() {
			plan, err := svc.Plan(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: allTypes})
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			var sent []dtos.SyncStreamLine
			err = svc.Stream(ctx, plan, func(line dtos.SyncStreamLine) error {
				sent = append(sent, line)
				return nil
			})

			Expect(err).To(HaveOccurred())
			Expect(sent).To(BeEmpty())
		})
	})
})
