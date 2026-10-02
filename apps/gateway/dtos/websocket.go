package dtos

import (
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/MaroonRides/api/internal/db/model"
)

type WebsocketMessageType string

var WebsocketMessageTypes = struct {
	Subscribe   WebsocketMessageType
	Unsubscribe WebsocketMessageType
	Ping        WebsocketMessageType
	Pong        WebsocketMessageType
	Error       WebsocketMessageType
	Vehicles    WebsocketMessageType
	Departures  WebsocketMessageType
}{
	Subscribe:   "subscribe",
	Unsubscribe: "unsubscribe",
	Ping:        "ping",
	Pong:        "pong",
	Error:       "error",
	Vehicles:    "vehicles",
	Departures:  "departures",
}

type WebsocketClientMessage struct {
	Type    WebsocketMessageType `json:"type"`
	RouteID uuid.UUID            `json:"routeId,omitzero"`
}

type WebsocketPongMessage struct {
	Type WebsocketMessageType `json:"type"`
}

type WebsocketErrorMessage struct {
	Type    WebsocketMessageType `json:"type"`
	Message string               `json:"message"`
}

type WebsocketVehiclesMessage struct {
	Type     WebsocketMessageType `json:"type"`
	RouteID  uuid.UUID            `json:"routeId"`
	Vehicles []WebsocketVehicle   `json:"vehicles"`
}

type WebsocketVehicle struct {
	ID          uuid.UUID `json:"id"`
	DirectionID uuid.UUID `json:"directionId"`
	Name        string    `json:"name"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Heading     float64   `json:"heading"`
	Speed       float64   `json:"speed"`
	Passengers  int       `json:"passengers"`
	Capacity    int       `json:"capacity"`
	Amenities   []string  `json:"amenities"`
	SeenAt      time.Time `json:"seenAt"`
}

func NewWebsocketVehiclesMessage(routeID uuid.UUID, vehicles []model.Vehicle) WebsocketVehiclesMessage {
	return WebsocketVehiclesMessage{
		Type:    WebsocketMessageTypes.Vehicles,
		RouteID: routeID,
		Vehicles: lo.Map(vehicles, func(v model.Vehicle, _ int) WebsocketVehicle {
			return WebsocketVehicle{
				ID:          v.ID,
				DirectionID: v.DirectionID,
				Name:        v.Name,
				Lat:         v.Lat,
				Lon:         v.Lon,
				Heading:     v.Heading,
				Speed:       v.Speed,
				Passengers:  v.Passengers,
				Capacity:    v.Capacity,
				Amenities:   v.Amenities,
				SeenAt:      v.SeenAt,
			}
		}),
	}
}

type WebsocketDeparturesMessage struct {
	Type       WebsocketMessageType `json:"type"`
	RouteID    uuid.UUID            `json:"routeId"`
	Departures []WebsocketDeparture `json:"departures"`
}

type WebsocketDeparture struct {
	ID          uuid.UUID  `json:"id"`
	StopID      uuid.UUID  `json:"stopId"`
	ScheduledAt time.Time  `json:"scheduledAt"`
	EstimatedAt *time.Time `json:"estimatedAt"`
	IsCancelled bool       `json:"isCancelled"`
}

func NewWebsocketDeparturesMessage(routeID uuid.UUID, departures []model.Departure) WebsocketDeparturesMessage {
	return WebsocketDeparturesMessage{
		Type:    WebsocketMessageTypes.Departures,
		RouteID: routeID,
		Departures: lo.Map(departures, func(d model.Departure, _ int) WebsocketDeparture {
			return WebsocketDeparture{
				ID:          d.ID,
				StopID:      d.StopID,
				ScheduledAt: d.ScheduledAt,
				EstimatedAt: d.EstimatedAt,
				IsCancelled: d.IsCancelled,
			}
		}),
	}
}
