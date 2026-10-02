package model

import "github.com/MaroonRides/api/internal/db/sync"

var SyncTables = []sync.Table{
	sync.For[Route, RouteAudit]("routeId"),
	sync.For[Direction, DirectionAudit]("directionId"),
	sync.For[Stop, StopAudit]("stopId"),
	sync.For[Alert, AlertAudit]("alertId"),
	sync.For[AlertDirection, AlertDirectionAudit]("alertDirectionId"),
	sync.For[Timetable, TimetableAudit]("timetableId"),
}

var ServerTables = []any{
	(*Vehicle)(nil),
	(*Departure)(nil),
	(*LiveDataSubscription)(nil),
	(*SyncMetadata)(nil),
}
