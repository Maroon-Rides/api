package services

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the distroless image ships no zoneinfo

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/internal/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

const (
	tripPlannerTimeZone = "America/Chicago"
	tripPlannerClock    = "3:04 PM"

	metersPerMile = 1609.344
	metersPerFoot = 0.3048
	unitMile      = "mi"

	clockLookback = 12 * time.Hour
)

var tripPlannerLocation = lo.Must(time.LoadLocation(tripPlannerTimeZone))

var (
	blockTagPattern    = regexp.MustCompile(`(?i)</?div[^>]*>`)
	tagPattern         = regexp.MustCompile(`<[^>]*>`)
	stopCodePattern    = regexp.MustCompile(`\s*\(ID: (\w+)\)\s*$`)
	walkToPattern      = regexp.MustCompile(`^Walk to (.+)$`)
	waitAtPattern      = regexp.MustCompile(`^Wait at (.+)$`)
	boardPattern       = regexp.MustCompile(`^Board \S+ (.+) and get off at (.+)$`)
	distancePattern    = regexp.MustCompile(`([\d.]+)\s*(mi|ft)\b`)
	stopCountPattern   = regexp.MustCompile(`(\d+) stops?\b`)
	isoDurationPattern = regexp.MustCompile(`dateTime="PT(?:(\d+)H)?(?:(\d+)M)?"`)
)

// newTripItineraries leaves stop, route and direction ids unset; resolveTripStops fills them.
func newTripItineraries(plan *busapi.TripPlan, origin dtos.TripEndpoint) []dtos.TripItinerary {
	return lo.Map(plan.Options, func(option busapi.TripOption, _ int) dtos.TripItinerary {
		return dtos.TripItinerary{
			DepartAt:              time.Unix(option.StartTime, 0),
			ArriveAt:              time.Unix(option.EndTime, 0),
			WalkingSeconds:        int(isoDuration(option.TotalWalkingTime).Seconds()),
			WalkingDistanceMeters: meters(plainText(option.TotalWalkingDistance)),
			Warnings:              lo.Map(option.Warnings, func(w string, _ int) string { return plainText(w) }),
			Bounds: dtos.TripBounds{
				SwLat: option.MapBounds.SwLatitude,
				SwLon: option.MapBounds.SwLongitude,
				NeLat: option.MapBounds.NeLatitude,
				NeLon: option.MapBounds.NeLongitude,
			},
			Legs: newTripLegs(option, origin),
		}
	})
}

func newTripLegs(option busapi.TripOption, origin dtos.TripEndpoint) []dtos.TripLeg {
	departAt := time.Unix(option.StartTime, 0)
	arriveAt := time.Unix(option.EndTime, 0)
	steps := option.Instructions

	legs := []dtos.TripLeg{}
	at := departAt
	here := dtos.TripPlace{Name: origin.Name, Lat: origin.Lat, Lon: origin.Lon}
	for i, step := range steps {
		if step.StepType == busapi.StepTypeEnd {
			continue
		}

		startAt := at
		endAt := arriveAt
		to := dtos.TripPlace{Lat: step.Latitude, Lon: step.Longitude}
		if i+1 < len(steps) {
			next := steps[i+1]
			if next.StepType != busapi.StepTypeEnd {
				endAt = latest(startAt, clockTime(next.StartTime, startAt))
			}
			to = dtos.TripPlace{Lat: next.Latitude, Lon: next.Longitude}
		}

		from := here
		from.Lat, from.Lon = step.Latitude, step.Longitude

		leg := dtos.TripLeg{StartAt: startAt, EndAt: endAt, From: from, Path: lo.FromPtr(step.Polyline)}
		text, code := instructionTarget(step.Instruction)
		switch step.StepType {
		case busapi.StepTypeWalking:
			leg.Type = dtos.TripLegTypes.Walk
			to.Name, to.StopCode = matchOne(walkToPattern, text), code
			leg.Walk = &dtos.TripWalk{
				DistanceMeters: meters(lo.FromPtr(step.Duration)),
				Steps: lo.Map(step.WalkingInstructions, func(w busapi.WalkingInstruction, _ int) dtos.TripWalkStep {
					return dtos.TripWalkStep{Instruction: plainText(w.Instruction), Path: w.Polyline}
				}),
			}
		case busapi.StepTypeBus:
			leg.Type = dtos.TripLegTypes.Ride
			match := boardPattern.FindStringSubmatch(text)
			to.StopCode = code
			ride := &dtos.TripRide{RouteShortName: lo.FromPtr(step.RouteShortName), StopCount: stopCount(lo.FromPtr(step.Duration))}
			if match != nil {
				ride.Headsign, to.Name = match[1], match[2]
			}
			leg.Ride = ride
		case busapi.StepTypeWaiting:
			leg.Type = dtos.TripLegTypes.Wait
			to.Name, to.StopCode = matchOne(waitAtPattern, text), code
		default:
			continue
		}
		leg.To = to
		legs = append(legs, leg)

		here = to
		at = endAt
	}

	return legs
}

// tripStopLookup is what resolveTripStops needs: every ride's route and every stop code in the plan.
type tripStopLookup struct {
	RouteShortNames []string
	StopCodes       []string
}

func newTripStopLookup(itineraries []dtos.TripItinerary) tripStopLookup {
	var lookup tripStopLookup
	for _, itinerary := range itineraries {
		for _, leg := range itinerary.Legs {
			if leg.Ride != nil {
				lookup.RouteShortNames = append(lookup.RouteShortNames, leg.Ride.RouteShortName)
			}
			for _, place := range []dtos.TripPlace{leg.From, leg.To} {
				if place.StopCode != nil {
					lookup.StopCodes = append(lookup.StopCodes, *place.StopCode)
				}
			}
		}
	}
	lookup.RouteShortNames = lo.Uniq(lookup.RouteShortNames)
	lookup.StopCodes = lo.Uniq(lookup.StopCodes)
	return lookup
}

// resolveTripStops matches each ride to the direction that serves its board stop before its
// alight stop, since a stop code alone can belong to either direction of a route.
// Walks and waits next to a ride share its stops.
func resolveTripStops(itineraries []dtos.TripItinerary, stops []model.Stop) {
	byRoute := lo.GroupBy(stops, func(s model.Stop) string { return s.Direction.Route.ShortName })

	for _, itinerary := range itineraries {
		legs := itinerary.Legs
		for i := range legs {
			if legs[i].Ride != nil {
				resolveRide(&legs[i], byRoute[legs[i].Ride.RouteShortName])
			}
		}
		for i := len(legs) - 2; i >= 0; i-- {
			if resolvesStop(legs[i].To, legs[i+1].From) {
				legs[i].To = legs[i+1].From
			}
		}
		for i := 1; i < len(legs); i++ {
			if resolvesStop(legs[i].From, legs[i-1].To) {
				legs[i].From = legs[i-1].To
			}
		}
	}
}

func resolveRide(leg *dtos.TripLeg, routeStops []model.Stop) {
	byDirection := lo.GroupBy(routeStops, func(s model.Stop) uuid.UUID { return s.DirectionID })

	var board, alight *model.Stop
	for _, directionStops := range byDirection {
		b := findStop(directionStops, leg.From.StopCode)
		a := findStop(directionStops, leg.To.StopCode)
		if a == nil {
			continue
		}
		if b != nil && b.Sequence < a.Sequence {
			board, alight = b, a
			break
		}
		if alight == nil {
			board, alight = b, a
		}
	}
	if alight == nil {
		return
	}

	route := alight.Direction.Route
	leg.Ride.RouteID = &route.ID
	leg.Ride.DirectionID = &alight.DirectionID
	leg.To = stopPlace(*alight, leg.To)
	if board != nil {
		leg.From = stopPlace(*board, leg.From)
	}
}

func findStop(stops []model.Stop, code *string) *model.Stop {
	if code == nil {
		return nil
	}
	stop, ok := lo.Find(stops, func(s model.Stop) bool { return s.SourceID == *code })
	if !ok {
		return nil
	}
	return &stop
}

func stopPlace(stop model.Stop, place dtos.TripPlace) dtos.TripPlace {
	return dtos.TripPlace{Name: stop.Name, Lat: stop.Lat, Lon: stop.Lon, StopCode: place.StopCode, StopID: &stop.ID}
}

// resolvesStop reports whether resolved is the same stop as place and has a known id.
func resolvesStop(place, resolved dtos.TripPlace) bool {
	return resolved.StopID != nil && place.StopCode != nil && resolved.StopCode != nil && *place.StopCode == *resolved.StopCode
}

// instructionTarget returns the last line of an instruction, which names where the step goes,
// without the stop code it may end with.
func instructionTarget(instruction string) (string, *string) {
	lines := plainLines(instruction)
	if len(lines) == 0 {
		return "", nil
	}
	last := lines[len(lines)-1]
	match := stopCodePattern.FindStringSubmatchIndex(last)
	if match == nil {
		return last, nil
	}
	code := last[match[2]:match[3]]
	return strings.TrimSpace(last[:match[0]]), &code
}

func plainLines(s string) []string {
	text := html.UnescapeString(tagPattern.ReplaceAllString(blockTagPattern.ReplaceAllString(s, "\n"), ""))
	return lo.FilterMap(strings.Split(text, "\n"), func(line string, _ int) (string, bool) {
		line = strings.TrimSpace(line)
		return line, line != ""
	})
}

func plainText(s string) string {
	return strings.Join(plainLines(s), ". ")
}

func matchOne(pattern *regexp.Regexp, s string) string {
	if match := pattern.FindStringSubmatch(s); match != nil {
		return match[1]
	}
	return s
}

func meters(s string) float64 {
	match := distancePattern.FindStringSubmatch(s)
	if match == nil {
		return 0
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	if match[2] == unitMile {
		return value * metersPerMile
	}
	return value * metersPerFoot
}

func stopCount(s string) int {
	match := stopCountPattern.FindStringSubmatch(s)
	if match == nil {
		return 0
	}
	return lo.Must(strconv.Atoi(match[1]))
}

func isoDuration(s string) time.Duration {
	match := isoDurationPattern.FindStringSubmatch(s)
	if match == nil {
		return 0
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
}

// clockTime places a planner clock reading like "10:03 AM" on the first day it falls after
// near, which is no more than half a day before it.
func clockTime(clock string, near time.Time) time.Time {
	parsed, err := time.ParseInLocation(tripPlannerClock, clock, tripPlannerLocation)
	if err != nil {
		return near
	}
	local := near.In(tripPlannerLocation)
	t := time.Date(local.Year(), local.Month(), local.Day(), parsed.Hour(), parsed.Minute(), 0, 0, tripPlannerLocation)
	if t.Before(near.Add(-clockLookback)) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
