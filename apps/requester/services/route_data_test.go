package services

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/internal/busapi"
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

var _ = Describe("routeRows", Label("unit"), func() {
	It("marks game day routes as not looping", func() {
		listed := []busapi.MapDirectionList{{LineColor: "#500000"}}

		rows := routeRows([]busapi.MapRoute{
			{Key: "bonfire", Name: "Bonfire", ShortName: "01", DirectionList: listed},
			{Key: "gameday", Name: "Gameday Bonfire", ShortName: "BF", DirectionList: listed},
		})

		Expect(rows).To(HaveLen(2))
		Expect(rows[0].IsLoop).To(BeTrue())
		Expect(rows[1].IsLoop).To(BeFalse())
	})
})

var _ = Describe("destinations", Label("unit"), func() {
	It("puts pregame before postgame", func() {
		got := destinations([]busapi.MapRoute{{DirectionList: []busapi.MapDirectionList{
			{Direction: busapi.MapDirection{Key: "post"}, Destination: "1 Hour Postgame"},
			{Direction: busapi.MapDirection{Key: "pre"}, Destination: "3 Hours Pregame"},
		}}})

		Expect(got).To(Equal(map[string]destination{
			"pre":  {text: "3 Hours Pregame", sequence: 0},
			"post": {text: "1 Hour Postgame", sequence: 1},
		}))
	})

	It("keeps the upstream order of other directions", func() {
		got := destinations([]busapi.MapRoute{{DirectionList: []busapi.MapDirectionList{
			{Direction: busapi.MapDirection{Key: "msc"}, Destination: "MSC"},
			{Direction: busapi.MapDirection{Key: "commons"}, Destination: "Commons"},
		}}})

		Expect(got).To(Equal(map[string]destination{
			"msc":     {text: "MSC", sequence: 0},
			"commons": {text: "Commons", sequence: 1},
		}))
	})
})

var _ = Describe("directionRows", Label("unit"), func() {
	It("skips pattern paths for directions missing from base data", func() {
		route := model.Route{ID: uuid.New(), SourceID: "03"}

		paths := []busapi.PatternPathsResponse{
			{RouteKey: "03", PatternPaths: []busapi.MapPatternPath{
				{DirectionKey: "msc"},
				{DirectionKey: "unlisted"},
			}},
		}

		rows := directionRows(paths, map[string]model.Route{"03": route}, map[string]destination{
			"msc": {text: "MSC", sequence: 0},
		})

		Expect(rows).To(HaveLen(1))
		Expect(rows[0].SourceID).To(Equal("msc"))
		Expect(rows[0].Destination).To(Equal("MSC"))
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
