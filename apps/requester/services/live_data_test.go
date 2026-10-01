package services

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

var _ = Describe("parseUpstreamTime", Label("unit"), func() {
	DescribeTable("treats zoneless times as UTC",
		func(value string) {
			got, err := parseUpstreamTime(value)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeTemporally("==", time.Date(2026, 9, 27, 19, 30, 0, 0, time.UTC)))
		},
		Entry("UTC suffix", "2026-09-27T19:30:00Z"),
		Entry("no zone", "2026-09-27T19:30:00"),
		Entry("offset", "2026-09-27T14:30:00-05:00"),
	)
})

var _ = Describe("departureRow", Label("unit"), func() {
	It("requires a scheduled time", func() {
		estimated := "2026-09-27T19:30:00Z"

		_, err := departureRow(busapi.DepartureTime{EstimatedDepartTimeUtc: &estimated})
		Expect(err).To(HaveOccurred())
	})

	It("leaves the estimate empty when upstream omits it", func() {
		scheduled := "2026-09-27T19:30:00Z"

		departure, err := departureRow(busapi.DepartureTime{ScheduledDepartTimeUtc: &scheduled})
		Expect(err).NotTo(HaveOccurred())
		Expect(departure.EstimatedAt).To(BeNil())
	})
})

var _ = Describe("routeKeysSynced", Label("unit"), func() {
	var (
		routeID      uuid.UUID
		apiRoutes    []busapi.MapRoute
		dbRoutes     []model.Route
		dbDirections []model.Direction
	)

	BeforeEach(func() {
		routeID = uuid.New()
		apiRoutes = []busapi.MapRoute{
			{Key: "route", DirectionList: []busapi.MapDirectionList{{Direction: busapi.MapDirection{Key: "direction"}}}},
			{Key: "route-without-directions"},
		}
		dbRoutes = []model.Route{{ID: routeID, SourceID: "route"}}
		dbDirections = []model.Direction{
			{RouteID: routeID, SourceID: "direction"},
			{RouteID: uuid.New(), SourceID: "stale-direction"},
		}
	})

	It("reports matching keys as in sync", func() {
		Expect(routeKeysSynced(apiRoutes, dbRoutes, dbDirections)).To(BeTrue())
	})

	It("reports a rotated direction key as out of sync", func() {
		rotated := []busapi.MapRoute{
			{Key: "route", DirectionList: []busapi.MapDirectionList{{Direction: busapi.MapDirection{Key: "new-direction"}}}},
		}

		Expect(routeKeysSynced(rotated, dbRoutes, dbDirections)).To(BeFalse())
	})

	It("ignores a direction removed upstream", func() {
		withRemoved := append(dbDirections, model.Direction{RouteID: routeID, SourceID: "removed-direction"})

		Expect(routeKeysSynced(apiRoutes, dbRoutes, withRemoved)).To(BeTrue())
	})
})

var _ = Describe("departureRows", Label("unit"), func() {
	It("keeps only the route directions that were requested at each stop", func() {
		scheduled := "2026-09-27T19:30:00Z"
		departs := []busapi.DepartureTime{{ScheduledDepartTimeUtc: &scheduled}}

		target := repositories.DepartureTarget{
			StopID: uuid.New(), StopSourceID: "shared",
			RouteID: uuid.New(), RouteSourceID: "route",
			DirectionID: uuid.New(), DirectionSourceID: "outbound",
		}
		results := map[string]busapi.NextDepartureTimesResponse{
			"shared": {RouteDirectionTimes: []busapi.RouteDirectionTime{
				{RouteKey: "route", DirectionKey: "outbound", NextDeparts: departs},
				{RouteKey: "other-route", DirectionKey: "inbound", NextDeparts: departs},
			}},
		}

		rows := departureRows(results, lo.KeyBy([]repositories.DepartureTarget{target}, keyDepartureTarget))

		Expect(rows).To(HaveLen(1))
		Expect(rows[0].RouteID).To(Equal(target.RouteID))
		Expect(rows[0].StopID).To(Equal(target.StopID))
		Expect(rows[0].DirectionID).To(Equal(target.DirectionID))
	})
})
