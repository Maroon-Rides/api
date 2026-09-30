package model

import "github.com/MaroonRides/api/internal/db/sync"

// RouteScope splits a stream by route, so a client can keep only the routes it wants offline.
const RouteScope = "routeId"

var SyncTables = []sync.Table{
	sync.For[Route, RouteAudit]("routeId"),
	sync.For[Direction, DirectionAudit]("directionId"),
	sync.For[Stop, StopAudit]("stopId"),
	sync.For[DirectionStop, DirectionStopAudit]("directionStopId"),
	sync.For[Alert, AlertAudit]("alertId"),
	sync.For[AlertDirection, AlertDirectionAudit]("alertDirectionId"),
	sync.For[Timetable, TimetableAudit]("timetableId").ScopedBy(RouteScope),
}

var ServerTables = []any{
	(*Vehicle)(nil),
	(*Departure)(nil),
	(*LiveDataSubscription)(nil),
	(*SyncMetadata)(nil),
}
