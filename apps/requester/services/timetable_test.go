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

func stopsByKey(stops ...model.Stop) map[string]model.Stop {
	return lo.KeyBy(stops, func(s model.Stop) string { return repositories.StopKey(s.DirectionID, s.SourceID) })
}

var _ = Describe("serviceDate", Label("unit"), func() {
	It("uses the service time zone", func() {
		location, err := NewServiceLocation()
		Expect(err).NotTo(HaveOccurred())

		// 03:00 UTC is still the previous evening in College Station
		got := serviceDate(time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC), location)

		Expect(got.Format(time.DateOnly)).To(Equal("2026-09-27"))
	})
})

var _ = Describe("scheduleDirectionIndex", Label("unit"), func() {
	var (
		index      scheduleDirectionIndex
		toMSC      model.Direction
		circulator model.Direction
	)

	BeforeEach(func() {
		toMSC = model.Direction{ID: uuid.New(), RouteID: uuid.New(), SourceID: "to-msc"}
		circulator = model.Direction{ID: uuid.New(), SourceID: "circulator"}

		routes := []busapi.MapRoute{
			{ShortName: "03", DirectionList: []busapi.MapDirectionList{
				{Direction: busapi.MapDirection{Key: "to-msc", Name: "to MSC"}},
				{Direction: busapi.MapDirection{Key: "to-white-creek", Name: "to White Creek"}},
			}},
			{ShortName: "01", DirectionList: []busapi.MapDirectionList{
				{Direction: busapi.MapDirection{Key: "circulator", Name: "Campus Circulator"}},
			}},
		}

		index = newScheduleDirectionIndex(routes, map[string]model.Direction{
			toMSC.SourceID:      toMSC,
			circulator.SourceID: circulator,
		})
	})

	DescribeTable("lookup",
		func(routeNumber, directionName, wantSourceID string, found bool) {
			got, ok := index.lookup(routeNumber, directionName)

			Expect(ok).To(Equal(found))
			Expect(got.SourceID).To(Equal(wantSourceID))
		},
		Entry("matches by name", "03", "to MSC", "to-msc", true),
		Entry("matches a single direction by name", "01", "Campus Circulator", "circulator", true),
		Entry("misses an unstored direction", "03", "to White Creek", "", false),
		Entry("misses an unknown route", "99", "to MSC", "", false),
	)

	It("skips stop schedules on unknown directions", func() {
		stop := model.Stop{ID: uuid.New(), DirectionID: toMSC.ID, SourceID: "0048"}
		date := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

		results := map[string]busapi.StopSchedulesResponse{
			"0048": {RouteStopSchedules: []busapi.RouteStopSchedule{
				{RouteNumber: "03", DirectionName: "to MSC", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:11:00Z"},
				}},
				{RouteNumber: "47", DirectionName: "Inbound", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:43:00Z"},
				}},
			}},
		}

		rows := timetableRows(results, stopsByKey(stop), index, date)

		Expect(rows).To(HaveLen(1))
		Expect(rows[0].StopID).To(Equal(stop.ID))
	})

	It("files a shared stop code under each direction's own stop", func() {
		mscStop := model.Stop{ID: uuid.New(), DirectionID: toMSC.ID, SourceID: "0100"}
		circulatorStop := model.Stop{ID: uuid.New(), DirectionID: circulator.ID, SourceID: "0100"}
		date := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

		results := map[string]busapi.StopSchedulesResponse{
			"0100": {RouteStopSchedules: []busapi.RouteStopSchedule{
				{RouteNumber: "03", DirectionName: "to MSC", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:11:00Z"},
				}},
				{RouteNumber: "01", DirectionName: "Campus Circulator", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:20:00Z"},
				}},
			}},
		}

		rows := timetableRows(results, stopsByKey(mscStop, circulatorStop), index, date)

		Expect(rows).To(ConsistOf(
			HaveField("StopID", mscStop.ID),
			HaveField("StopID", circulatorStop.ID),
		))
	})

	It("folds a stop's departures in one direction into one sorted timetable", func() {
		stop := model.Stop{ID: uuid.New(), DirectionID: toMSC.ID, SourceID: "0048"}
		date := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

		results := map[string]busapi.StopSchedulesResponse{
			"0048": {RouteStopSchedules: []busapi.RouteStopSchedule{
				{RouteNumber: "03", DirectionName: "to MSC", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:41:00Z"},
					{ScheduledDepartTimeUtc: "2026-09-28T12:11:00Z"},
				}},
				{RouteNumber: "03", DirectionName: "to MSC", StopTimes: []busapi.StopTime{
					{ScheduledDepartTimeUtc: "2026-09-28T12:11:00Z"},
					{ScheduledDepartTimeUtc: "2026-09-28T12:26:00Z"},
				}},
			}},
		}

		rows := timetableRows(results, stopsByKey(stop), index, date)

		Expect(rows).To(HaveLen(1))
		Expect(rows[0].ServiceDate).To(Equal(date))
		Expect(rows[0].Departures).To(Equal([]time.Time{
			time.Date(2026, 9, 28, 12, 11, 0, 0, time.UTC),
			time.Date(2026, 9, 28, 12, 26, 0, 0, time.UTC),
			time.Date(2026, 9, 28, 12, 41, 0, 0, time.UTC),
		}))
	})
})
