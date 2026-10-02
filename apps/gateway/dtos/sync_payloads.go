package dtos

import (
	"time"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/internal/db/model"
)

// A payload never changes once shipped. A new field means a new version of the entity.
var SyncPayloads = map[SyncEntityType]any{
	SyncEntityTypes.RouteV1:                SyncRouteV1{},
	SyncEntityTypes.RouteDeleteV1:          SyncRouteDeleteV1{},
	SyncEntityTypes.DirectionV1:            SyncDirectionV1{},
	SyncEntityTypes.DirectionDeleteV1:      SyncDirectionDeleteV1{},
	SyncEntityTypes.StopV1:                 SyncStopV1{},
	SyncEntityTypes.StopDeleteV1:           SyncStopDeleteV1{},
	SyncEntityTypes.AlertV1:                SyncAlertV1{},
	SyncEntityTypes.AlertDeleteV1:          SyncAlertDeleteV1{},
	SyncEntityTypes.AlertDirectionV1:       SyncAlertDirectionV1{},
	SyncEntityTypes.AlertDirectionDeleteV1: SyncAlertDirectionDeleteV1{},
	SyncEntityTypes.TimetableV1:            SyncTimetableV1{},
	SyncEntityTypes.TimetableDeleteV1:      SyncTimetableDeleteV1{},
	SyncEntityTypes.SyncResetV1:            SyncResetV1{},
	SyncEntityTypes.SyncCompleteV1:         SyncCompleteV1{},
}

const serviceDateLayout = time.DateOnly

type SyncResetV1 struct{}

type SyncCompleteV1 struct{}

type SyncRouteV1 struct {
	ID         uuid.UUID `json:"id"`
	ShortName  string    `json:"shortName"`
	LongName   string    `json:"longName"`
	LightColor string    `json:"lightColor"`
	DarkColor  string    `json:"darkColor"`
	Active     bool      `json:"active"`
}

func NewSyncRouteV1(r model.Route) SyncRouteV1 {
	return SyncRouteV1{
		ID:         r.ID,
		ShortName:  r.ShortName,
		LongName:   r.LongName,
		LightColor: r.LightColor,
		DarkColor:  r.DarkColor,
		Active:     r.Active,
	}
}

type SyncRouteDeleteV1 struct {
	RouteID uuid.UUID `json:"routeId"`
}

func NewSyncRouteDeleteV1(a model.RouteAudit) SyncRouteDeleteV1 {
	return SyncRouteDeleteV1{RouteID: a.RouteID}
}

type SyncDirectionV1 struct {
	ID          uuid.UUID `json:"id"`
	RouteID     uuid.UUID `json:"routeId"`
	Destination string    `json:"destination"`
	Sequence    int       `json:"sequence"`
	Path        string    `json:"path"`
}

func NewSyncDirectionV1(d model.Direction) SyncDirectionV1 {
	return SyncDirectionV1{
		ID:          d.ID,
		RouteID:     d.RouteID,
		Destination: d.Destination,
		Sequence:    d.Sequence,
		Path:        d.Path,
	}
}

type SyncDirectionDeleteV1 struct {
	DirectionID uuid.UUID `json:"directionId"`
}

func NewSyncDirectionDeleteV1(a model.DirectionAudit) SyncDirectionDeleteV1 {
	return SyncDirectionDeleteV1{DirectionID: a.DirectionID}
}

type SyncStopV1 struct {
	ID          uuid.UUID `json:"id"`
	DirectionID uuid.UUID `json:"directionId"`
	Name        string    `json:"name"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Amenities   []string  `json:"amenities"`
	Sequence    int       `json:"sequence"`
	IsTimepoint bool      `json:"isTimepoint"`
}

func NewSyncStopV1(s model.Stop) SyncStopV1 {
	return SyncStopV1{
		ID:          s.ID,
		DirectionID: s.DirectionID,
		Name:        s.Name,
		Lat:         s.Lat,
		Lon:         s.Lon,
		Amenities:   s.Amenities,
		Sequence:    s.Sequence,
		IsTimepoint: s.IsTimepoint,
	}
}

type SyncStopDeleteV1 struct {
	StopID uuid.UUID `json:"stopId"`
}

func NewSyncStopDeleteV1(a model.StopAudit) SyncStopDeleteV1 {
	return SyncStopDeleteV1{StopID: a.StopID}
}

type SyncAlertV1 struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	TimeRangeText  string     `json:"timeRangeText"`
	DailyStartTime string     `json:"dailyStartTime"`
	DailyEndTime   string     `json:"dailyEndTime"`
	StartsAt       time.Time  `json:"startsAt"`
	EndsAt         *time.Time `json:"endsAt"`
}

func NewSyncAlertV1(a model.Alert) SyncAlertV1 {
	return SyncAlertV1{
		ID:             a.ID,
		Title:          a.Title,
		Description:    a.Description,
		TimeRangeText:  a.TimeRangeText,
		DailyStartTime: a.DailyStartTime,
		DailyEndTime:   a.DailyEndTime,
		StartsAt:       a.StartsAt,
		EndsAt:         a.EndsAt,
	}
}

type SyncAlertDeleteV1 struct {
	AlertID uuid.UUID `json:"alertId"`
}

func NewSyncAlertDeleteV1(a model.AlertAudit) SyncAlertDeleteV1 {
	return SyncAlertDeleteV1{AlertID: a.AlertID}
}

type SyncAlertDirectionV1 struct {
	ID          uuid.UUID `json:"id"`
	AlertID     uuid.UUID `json:"alertId"`
	DirectionID uuid.UUID `json:"directionId"`
}

func NewSyncAlertDirectionV1(ad model.AlertDirection) SyncAlertDirectionV1 {
	return SyncAlertDirectionV1{
		ID:          ad.ID,
		AlertID:     ad.AlertID,
		DirectionID: ad.DirectionID,
	}
}

type SyncAlertDirectionDeleteV1 struct {
	AlertDirectionID uuid.UUID `json:"alertDirectionId"`
}

func NewSyncAlertDirectionDeleteV1(a model.AlertDirectionAudit) SyncAlertDirectionDeleteV1 {
	return SyncAlertDirectionDeleteV1{AlertDirectionID: a.AlertDirectionID}
}

type SyncTimetableV1 struct {
	ID          uuid.UUID   `json:"id"`
	StopID      uuid.UUID   `json:"stopId"`
	ServiceDate string      `json:"serviceDate" format:"date"`
	Departures  []time.Time `json:"departures"`
}

func NewSyncTimetableV1(t model.Timetable) SyncTimetableV1 {
	return SyncTimetableV1{
		ID:          t.ID,
		StopID:      t.StopID,
		ServiceDate: t.ServiceDate.Format(serviceDateLayout),
		Departures:  t.Departures,
	}
}

type SyncTimetableDeleteV1 struct {
	TimetableID uuid.UUID `json:"timetableId"`
}

func NewSyncTimetableDeleteV1(a model.TimetableAudit) SyncTimetableDeleteV1 {
	return SyncTimetableDeleteV1{TimetableID: a.TimetableID}
}
