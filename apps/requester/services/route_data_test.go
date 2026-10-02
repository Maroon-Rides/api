package services

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

var _ = Describe("stopRows", Label("unit"), func() {
	It("keeps each route's own location for a shared stop code", func() {
		campus := model.Direction{ID: uuid.New(), SourceID: "campus"}
		gameday := model.Direction{ID: uuid.New(), SourceID: "gameday"}

		paths := []busapi.PatternPathsResponse{
			{PatternPaths: []busapi.MapPatternPath{{DirectionKey: "campus", PatternPoints: []busapi.MapPatternPoint{
				{Latitude: 30.61454, Longitude: -96.34241, Stop: &busapi.MapStop{StopCode: "0100", Name: "MSC"}},
			}}}},
			{PatternPaths: []busapi.MapPatternPath{{DirectionKey: "gameday", PatternPoints: []busapi.MapPatternPoint{
				{Latitude: 30.6, Longitude: -96.3},
				{Latitude: 30.61518, Longitude: -96.33739, Stop: &busapi.MapStop{StopCode: "0100", Name: "Commons"}},
			}}}},
		}

		rows := stopRows(paths, map[string]model.Direction{"campus": campus, "gameday": gameday})

		Expect(rows).To(Equal([]model.Stop{
			{DirectionID: campus.ID, SourceID: "0100", Name: "MSC", Lat: 30.61454, Lon: -96.34241, Sequence: 0},
			{DirectionID: gameday.ID, SourceID: "0100", Name: "Commons", Lat: 30.61518, Lon: -96.33739, Sequence: 0},
		}))
	})
})

var _ = Describe("alertRows", Label("unit"), func() {
	It("leaves the end empty for alerts that run until further notice", func() {
		rows := alertRows([]busapi.MapServiceInterruption{
			{Key: "3579", StartDateUtc: "2026-09-03T17:00:00+00:00", EndDateUtc: ""},
		})

		Expect(rows).To(HaveLen(1))
		Expect(rows[0].EndsAt).To(BeNil())
	})

	It("skips alerts with an unparseable start", func() {
		rows := alertRows([]busapi.MapServiceInterruption{{Key: "1", StartDateUtc: "soon"}})

		Expect(rows).To(BeEmpty())
	})
})

var _ = Describe("alertDirectionRows", Label("unit"), func() {
	It("matches alerts by their numeric keys", func() {
		alert := model.Alert{ID: uuid.New(), SourceID: "3582"}
		direction := model.Direction{ID: uuid.New(), SourceID: "inbound"}

		routes := []busapi.MapRoute{{
			DirectionList: []busapi.MapDirectionList{
				{Direction: busapi.MapDirection{Key: "inbound"}, ServiceInterruptionKeys: []int{3582, 9999}},
				{Direction: busapi.MapDirection{Key: "outbound"}, ServiceInterruptionKeys: []int{3582}},
			},
		}}

		rows := alertDirectionRows(routes,
			map[string]model.Alert{alert.SourceID: alert},
			map[string]model.Direction{direction.SourceID: direction},
		)

		Expect(rows).To(Equal([]model.AlertDirection{{AlertID: alert.ID, DirectionID: direction.ID}}))
	})
})
