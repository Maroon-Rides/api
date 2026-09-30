package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// A gateway refreshes its rows on every interval, so a row that misses two refreshes
// belongs to a gateway that is gone.
const (
	LiveDataSubscriptionRefreshInterval = 15 * time.Second
	LiveDataSubscriptionStaleAfter      = 2 * LiveDataSubscriptionRefreshInterval
)

type LiveDataSubscription struct {
	bun.BaseModel `bun:"table:live_data_subscriptions"`

	ID uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`

	RouteID       uuid.UUID `bun:"routeId,type:uuid,notnull,unique:route_client_uq"`
	ClientID      uuid.UUID `bun:"clientId,type:uuid,notnull,unique:route_client_uq"`
	LastUpdatedAt time.Time `bun:"lastUpdatedAt,type:timestamp,notnull,default:current_timestamp"`

	Route *Route `bun:"rel:belongs-to,join:routeId=id,on_delete:CASCADE"`
}
