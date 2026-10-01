package services

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/gtfs"
)

var _ = Describe("feedTimepoints", Label("unit"), func() {
	It("keys stops by route short name and stop code", func() {
		feed := &gtfs.Feed{
			Routes: []gtfs.Route{{RouteID: "r12", ShortName: "12"}},
			Stops:  []gtfs.Stop{{StopID: "s1", StopCode: "1200"}, {StopID: "s2", StopCode: "1202"}},
			Trips:  []gtfs.Trip{{TripID: "t1", RouteID: "r12"}},
			StopTimes: []gtfs.StopTime{
				{TripID: "t1", StopID: "s1", Timepoint: true},
				{TripID: "t1", StopID: "s2", Timepoint: false},
			},
		}

		Expect(feedTimepoints(feed)).To(Equal(map[routeStop]bool{
			{routeShortName: "12", stopCode: "1200"}: true,
			{routeShortName: "12", stopCode: "1202"}: false,
		}))
	})

	It("marks a stop that any trip times exactly", func() {
		feed := &gtfs.Feed{
			Routes: []gtfs.Route{{RouteID: "r12", ShortName: "12"}},
			Stops:  []gtfs.Stop{{StopID: "s1", StopCode: "1200"}},
			Trips:  []gtfs.Trip{{TripID: "t1", RouteID: "r12"}, {TripID: "t2", RouteID: "r12"}},
			StopTimes: []gtfs.StopTime{
				{TripID: "t1", StopID: "s1", Timepoint: false},
				{TripID: "t2", StopID: "s1", Timepoint: true},
			},
		}

		Expect(feedTimepoints(feed)).To(HaveKeyWithValue(routeStop{routeShortName: "12", stopCode: "1200"}, true))
	})
})

var _ = Describe("timepointChanges", Label("unit"), func() {
	timepoints := map[routeStop]bool{
		{routeShortName: "12", stopCode: "1200"}: true,
		{routeShortName: "12", stopCode: "1202"}: false,
	}

	It("updates only direction stops whose flag differs from the feed", func() {
		marked := repositories.DirectionStopRef{ID: uuid.New(), RouteShortName: "12", StopSourceID: "1200"}
		cleared := repositories.DirectionStopRef{ID: uuid.New(), RouteShortName: "12", StopSourceID: "1202", IsTimepoint: true}
		unchanged := repositories.DirectionStopRef{ID: uuid.New(), RouteShortName: "12", StopSourceID: "1200", IsTimepoint: true}

		changes := timepointChanges([]repositories.DirectionStopRef{marked, cleared, unchanged}, timepoints)

		Expect(changes.marked).To(Equal([]uuid.UUID{marked.ID}))
		Expect(changes.cleared).To(Equal([]uuid.UUID{cleared.ID}))
	})

	It("leaves direction stops the feed does not schedule", func() {
		unscheduled := repositories.DirectionStopRef{ID: uuid.New(), RouteShortName: "01", StopSourceID: "1200", IsTimepoint: true}

		changes := timepointChanges([]repositories.DirectionStopRef{unscheduled}, timepoints)

		Expect(changes.marked).To(BeEmpty())
		Expect(changes.cleared).To(BeEmpty())
	})
})
