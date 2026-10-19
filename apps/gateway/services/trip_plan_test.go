package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

const (
	fixtureDirect   = "testdata/trip-plan-direct.json"
	fixtureTransfer = "testdata/trip-plan-transfer.json"
)

type fakeTripPlanSource struct {
	plan  *busapi.TripPlan
	err   error
	stops []model.Stop
	query busapi.TripPlanQuery
}

func (f *fakeTripPlanSource) GetTripPlan(_ context.Context, query busapi.TripPlanQuery) (*busapi.TripPlan, error) {
	f.query = query
	return f.plan, f.err
}

func (f *fakeTripPlanSource) GetRouteStops(context.Context, []string, []string) ([]model.Stop, error) {
	return f.stops, nil
}

func loadTripPlan(path string) *busapi.TripPlan {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	var plan busapi.TripPlan
	Expect(json.Unmarshal(raw, &plan)).To(Succeed())
	return &plan
}

func routeStops(route *model.Route, codes ...string) []model.Stop {
	direction := &model.Direction{ID: newV7(), RouteID: route.ID, Route: route}
	stops := make([]model.Stop, len(codes))
	for i, code := range codes {
		stops[i] = model.Stop{ID: newV7(), DirectionID: direction.ID, SourceID: code, Name: "stop " + code, Sequence: i, Direction: direction}
	}
	return stops
}

var tripRequest = dtos.TripPlanRequest{
	Origin:      dtos.TripEndpoint{Name: "My Location", Lat: 30.6123, Lon: -96.3415},
	Destination: dtos.TripEndpoint{Name: "Zachry", Lat: 30.6214, Lon: -96.3402},
}

var _ = Describe("TripPlanService", Label("unit"), func() {
	ctx := context.Background()

	plan := func(source *fakeTripPlanSource, req dtos.TripPlanRequest) dtos.TripPlanResponse {
		res, err := services.NewTripPlanService(source).Plan(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		return res
	}

	It("turns a direct option into walk, ride and walk legs", func() {
		res := plan(&fakeTripPlanSource{plan: loadTripPlan(fixtureDirect)}, tripRequest)

		itinerary := res.Itineraries[0]
		Expect(itinerary.DepartAt).To(Equal(time.Unix(1792422099, 0)))
		Expect(itinerary.ArriveAt).To(Equal(time.Unix(1792422597, 0)))
		Expect(itinerary.WalkingSeconds).To(Equal(300))
		Expect(itinerary.WalkingDistanceMeters).To(BeNumerically("~", 482.8, 0.1))

		legs := itinerary.Legs
		Expect(legs).To(HaveLen(3))
		Expect([]dtos.TripLegType{legs[0].Type, legs[1].Type, legs[2].Type}).To(Equal(
			[]dtos.TripLegType{dtos.TripLegTypes.Walk, dtos.TripLegTypes.Ride, dtos.TripLegTypes.Walk}))

		Expect(legs[0].From.Name).To(Equal("My Location"))
		Expect(legs[0].To.Name).To(Equal("MSC"))
		Expect(legs[0].To.StopCode).To(HaveValue(Equal("2700")))
		Expect(legs[0].Walk.DistanceMeters).To(BeNumerically("~", 102.1, 0.1))
		Expect(legs[0].Walk.Steps[0].Instruction).To(Equal("Head northeast"))

		Expect(legs[1].From.StopCode).To(HaveValue(Equal("2700")))
		Expect(legs[1].Ride.RouteShortName).To(Equal("27"))
		Expect(legs[1].Ride.Headsign).To(Equal("Wolf Pen Creek"))
		Expect(legs[1].Ride.StopCount).To(Equal(3))
		Expect(legs[1].To.Name).To(Equal("Ross and Bizzell - South"))
		Expect(legs[1].To.StopCode).To(HaveValue(Equal("0014")))

		Expect(legs[2].To.Name).To(Equal("Destination"))
		Expect(legs[2].To.StopCode).To(BeNil())
	})

	It("strips every HTML tag", func() {
		for _, fixture := range []string{fixtureDirect, fixtureTransfer} {
			raw, err := json.Marshal(plan(&fakeTripPlanSource{plan: loadTripPlan(fixture)}, tripRequest))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(raw)).NotTo(ContainSubstring(`<`), fixture)
		}
	})

	It("chains leg times from departure to arrival", func() {
		for _, fixture := range []string{fixtureDirect, fixtureTransfer} {
			for _, itinerary := range plan(&fakeTripPlanSource{plan: loadTripPlan(fixture)}, tripRequest).Itineraries {
				legs := itinerary.Legs
				Expect(legs[0].StartAt).To(Equal(itinerary.DepartAt))
				Expect(legs[len(legs)-1].EndAt).To(Equal(itinerary.ArriveAt))
				for i := range legs {
					Expect(legs[i].EndAt).NotTo(BeTemporally("<", legs[i].StartAt))
					if i > 0 {
						Expect(legs[i].StartAt).To(Equal(legs[i-1].EndAt))
					}
				}
			}
		}
	})

	It("transfers from the alight stop to the next board stop while waiting", func() {
		legs := plan(&fakeTripPlanSource{plan: loadTripPlan(fixtureTransfer)}, tripRequest).Itineraries[0].Legs

		Expect(legs[2].Type).To(Equal(dtos.TripLegTypes.Wait))
		Expect(legs[2].From).To(Equal(legs[1].To))
		Expect(legs[2].From.StopCode).To(HaveValue(Equal("3100")))
		Expect(legs[2].To.StopCode).To(HaveValue(Equal("2700")))
		Expect(legs[2].To).To(Equal(legs[3].From))
	})

	It("resolves the direction that reaches the alight stop after the board stop", func() {
		route := &model.Route{ID: newV7(), ShortName: "27"}
		outbound := routeStops(route, "2700", "0001", "0014")
		inbound := routeStops(route, "0014", "0002", "2700")
		source := &fakeTripPlanSource{plan: loadTripPlan(fixtureDirect), stops: append(inbound, outbound...)}

		legs := plan(source, tripRequest).Itineraries[0].Legs

		Expect(legs[1].Ride.RouteID).To(HaveValue(Equal(route.ID)))
		Expect(legs[1].Ride.DirectionID).To(HaveValue(Equal(outbound[0].DirectionID)))
		Expect(legs[1].From.StopID).To(HaveValue(Equal(outbound[0].ID)))
		Expect(legs[1].To.StopID).To(HaveValue(Equal(outbound[2].ID)))
		Expect(legs[0].To).To(Equal(legs[1].From))
		Expect(legs[2].From).To(Equal(legs[1].To))
	})

	It("leaves rides on unknown routes unresolved", func() {
		legs := plan(&fakeTripPlanSource{plan: loadTripPlan(fixtureDirect)}, tripRequest).Itineraries[0].Legs

		Expect(legs[1].Ride.RouteID).To(BeNil())
		Expect(legs[1].To.StopID).To(BeNil())
	})

	It("asks for an arrive by trip with the chosen preference", func() {
		source := &fakeTripPlanSource{plan: &busapi.TripPlan{}}
		at := time.Unix(1792422000, 0)
		req := tripRequest
		req.Time, req.TimeKind, req.Preference = &at, dtos.TripTimeKinds.ArriveBy, dtos.TripPreferences.LessWalking

		Expect(plan(source, req).Itineraries).To(BeEmpty())

		Expect(source.query.ArriveBy).To(HaveValue(Equal(at)))
		Expect(source.query.DepartAt).To(BeNil())
		Expect(source.query.RouteOption).To(Equal(busapi.RouteOptionLessWalking))
		Expect(source.query.Origin.Title).To(Equal("My Location"))
		Expect(source.query.Destination.Latitude).To(HaveValue(Equal(30.6214)))
	})

	It("departs now by default", func() {
		source := &fakeTripPlanSource{plan: &busapi.TripPlan{}}
		plan(source, tripRequest)

		Expect(source.query.DepartAt).To(HaveValue(BeTemporally("~", time.Now(), time.Minute)))
		Expect(source.query.RouteOption).To(Equal(busapi.RouteOptionBest))
	})

	It("reports an upstream failure as the planner being unavailable", func() {
		source := &fakeTripPlanSource{err: errors.New("timeout")}
		_, err := services.NewTripPlanService(source).Plan(ctx, tripRequest)

		Expect(err).To(MatchError(services.ErrTripPlannerUnavailable))
	})
})
