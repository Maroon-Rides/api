package dtos

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/internal/enum"
)

type TripTimeKind string

var TripTimeKinds = struct {
	DepartAt TripTimeKind
	ArriveBy TripTimeKind
}{
	DepartAt: "departAt",
	ArriveBy: "arriveBy",
}

func (TripTimeKind) EnumValues() []any { return enum.Values(TripTimeKinds) }

type TripPreference string

var TripPreferences = struct {
	Best           TripPreference
	FewerTransfers TripPreference
	LessWalking    TripPreference
}{
	Best:           "best",
	FewerTransfers: "fewerTransfers",
	LessWalking:    "lessWalking",
}

func (TripPreference) EnumValues() []any { return enum.Values(TripPreferences) }

type TripLegType string

var TripLegTypes = struct {
	Walk TripLegType
	Ride TripLegType
	Wait TripLegType
}{
	Walk: "walk",
	Ride: "ride",
	Wait: "wait",
}

func (TripLegType) EnumValues() []any { return enum.Values(TripLegTypes) }

// TripEndpoint's Name is shown back in the plan, so pass what the user picked, like "My Location".
type TripEndpoint struct {
	Name string  `json:"name" validate:"required"`
	Lat  float64 `json:"lat" validate:"latitude"`
	Lon  float64 `json:"lon" validate:"longitude"`
}

// Time defaults to now. TimeKind defaults to departAt and Preference to best.
type TripPlanRequest struct {
	Origin      TripEndpoint   `json:"origin"`
	Destination TripEndpoint   `json:"destination"`
	Time        *time.Time     `json:"time,omitempty"`
	TimeKind    TripTimeKind   `json:"timeKind,omitempty" validate:"omitempty,oneof=departAt arriveBy"`
	Preference  TripPreference `json:"preference,omitempty" validate:"omitempty,oneof=best fewerTransfers lessWalking"`
}

type TripPlanResponse struct {
	Itineraries []TripItinerary `json:"itineraries"`
}

type TripItinerary struct {
	DepartAt              time.Time  `json:"departAt"`
	ArriveAt              time.Time  `json:"arriveAt"`
	WalkingSeconds        int        `json:"walkingSeconds"`
	WalkingDistanceMeters float64    `json:"walkingDistanceMeters"`
	Warnings              []string   `json:"warnings"`
	Bounds                TripBounds `json:"bounds"`
	Legs                  []TripLeg  `json:"legs"`
}

type TripBounds struct {
	SwLat float64 `json:"swLat"`
	SwLon float64 `json:"swLon"`
	NeLat float64 `json:"neLat"`
	NeLon float64 `json:"neLon"`
}

// TripLeg's Path is Google Polyline encoded, like direction paths, and empty for a wait.
// Walk is set only on walk legs and Ride only on ride legs.
type TripLeg struct {
	Type    TripLegType `json:"type"`
	StartAt time.Time   `json:"startAt"`
	EndAt   time.Time   `json:"endAt"`
	From    TripPlace   `json:"from"`
	To      TripPlace   `json:"to"`
	Path    string      `json:"path"`
	Walk    *TripWalk   `json:"walk,omitempty"`
	Ride    *TripRide   `json:"ride,omitempty"`
}

// TripPlace's StopID is set when the place is a stop this server knows.
type TripPlace struct {
	Name     string     `json:"name"`
	Lat      float64    `json:"lat"`
	Lon      float64    `json:"lon"`
	StopCode *string    `json:"stopCode"`
	StopID   *uuid.UUID `json:"stopId"`
}

type TripWalk struct {
	DistanceMeters float64        `json:"distanceMeters"`
	Steps          []TripWalkStep `json:"steps"`
}

type TripWalkStep struct {
	Instruction string `json:"instruction"`
	Path        string `json:"path"`
}

// TripRide's RouteID and DirectionID are set when the route is one this server knows.
type TripRide struct {
	RouteID        *uuid.UUID `json:"routeId"`
	DirectionID    *uuid.UUID `json:"directionId"`
	RouteShortName string     `json:"routeShortName"`
	Headsign       string     `json:"headsign"`
	StopCount      int        `json:"stopCount"`
}
